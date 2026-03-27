package larkbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/usage"
	"lab/askplanner/internal/usererr"
	"lab/askplanner/internal/workspace"
)

// App owns the long-lived shared services and the websocket supervisor for one
// or more Feishu bot runtimes in the same process.
type App struct {
	shared        sharedServices
	bots          []*botRuntime
	clientFactory websocketClientFactory
}

// New constructs a fully wired Lark bot application from process config.
func New(cfg *config.Config) (*App, error) {
	if len(cfg.LarkBots) == 0 {
		return nil, fmt.Errorf("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
	}

	responder, err := codex.NewResponder(cfg)
	if err != nil {
		return nil, fmt.Errorf("build codex responder: %w", err)
	}
	prefetcher, err := clinic.NewPrefetcher(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clinic prefetcher: %w", err)
	}
	attachmentManager, err := attachments.NewManager(cfg.FeishuFileDir, cfg.FeishuUserFileMaxItems)
	if err != nil {
		return nil, fmt.Errorf("build attachment manager: %w", err)
	}
	workspaceManager, err := workspace.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("build workspace manager: %w", err)
	}
	tracker, err := usage.NewQuestionTracker(cfg)
	if err != nil {
		log.Printf("[larkbot] usage tracker disabled: %v", err)
	}

	app := &App{
		shared: sharedServices{
			responder:   responder,
			prefetcher:  prefetcher,
			attachments: attachmentManager,
			workspace:   workspaceManager,
			tracker:     tracker,
		},
		clientFactory: defaultWebsocketClientFactory,
	}

	bots := make([]*botRuntime, 0, len(cfg.LarkBots))
	for _, botCfg := range cfg.LarkBots {
		if strings.TrimSpace(botCfg.BotName) == "" {
			log.Printf("[larkbot:%s] FEISHU_BOT_NAME is empty; group @ detection will rely on text_without_at_bot only", botCfg.Key)
		}
		runtime := &botRuntime{
			parent:      app,
			cfg:         botCfg,
			apiClient:   lark.NewClient(botCfg.AppID, botCfg.AppSecret, lark.WithLogLevel(larkcore.LogLevelInfo)),
			dedup:       &messageDedup{},
			dedupMaxAge: time.Duration(cfg.FeishuDedupTimeoutInMin) * time.Minute,
			bot: botIdentity{
				key:  botCfg.Key,
				name: strings.ToLower(strings.TrimSpace(botCfg.BotName)),
			},
		}
		runtime.client = app.clientFactory(botCfg.AppID, botCfg.AppSecret, runtime.newEventHandler())
		bots = append(bots, runtime)
	}
	app.bots = bots
	return app, nil
}

func defaultWebsocketClientFactory(appID, appSecret string, handler *dispatcher.EventDispatcher) websocketClient {
	return larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(handler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)
}

// Run starts shared background maintenance and blocks until any bot exits with
// an error or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.shared.workspace.StartBackgroundJobs(runCtx)

	errCh := make(chan error, len(a.bots))
	var wg sync.WaitGroup
	for _, bot := range a.bots {
		bot.startDedupCleanup(runCtx)
		wg.Add(1)
		go func(bot *botRuntime) {
			defer wg.Done()
			log.Printf("%s starting websocket client...", bot.bot.prefix())
			if err := bot.client.Start(runCtx); err != nil && runCtx.Err() == nil {
				select {
				case errCh <- fmt.Errorf("%s websocket client failed: %w", bot.bot.prefix(), err):
				default:
				}
				cancel()
			}
		}(bot)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		<-done
		return err
	case <-done:
		return runCtx.Err()
	}
}

func (b *botRuntime) startDedupCleanup(ctx context.Context) {
	if b.dedupMaxAge <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.dedup.cleanup(b.dedupMaxAge)
			}
		}
	}()
}

func (b *botRuntime) newEventHandler() *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			log.Printf("%s message received: %s", b.bot.prefix(), larkcore.Prettify(event))

			messageID := extractMessageID(event)
			if messageID == "" {
				log.Printf("%s skip: empty message_id", b.bot.prefix())
				return nil
			}
			if b.dedup.isDuplicate(messageID) {
				log.Printf("%s skip duplicate message_id=%s", b.bot.prefix(), messageID)
				return nil
			}
			if ok, reason := shouldHandleEvent(event, b.bot); !ok {
				log.Printf("%s skip message_id=%s: %s", b.bot.prefix(), messageID, reason)
				return nil
			}

			return withTypingReaction(ctx, b.apiClient, messageID, func() error {
				answer, replyOpts, err := b.answerEvent(ctx, event)
				if err != nil {
					log.Printf("%s handle event error: %v (message_id=%s)", b.bot.prefix(), err, messageID)
					answer = usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs.")
				}

				reply, err := buildReplyBody(answer)
				if err != nil {
					return fmt.Errorf("build reply body: %w", err)
				}
				if err := replyMessage(ctx, b.apiClient, messageID, reply, replyOpts); err != nil {
					return fmt.Errorf("reply message: %w", err)
				}
				return nil
			})
		})
}
