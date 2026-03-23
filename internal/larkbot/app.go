package larkbot

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	"lab/askplanner/internal/workspace"
)

type App struct {
	appID       string
	appSecret   string
	apiClient   *lark.Client
	responder   *codex.Responder
	prefetcher  *clinic.Prefetcher
	attachments *attachments.Manager
	workspace   *workspace.Manager
	dedup       *messageDedup
	dedupMaxAge time.Duration
	bot         botIdentity
}

func New(cfg *config.Config) (*App, error) {
	if strings.TrimSpace(cfg.FeishuAppID) == "" || strings.TrimSpace(cfg.FeishuAppSecret) == "" {
		return nil, fmt.Errorf("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
	}
	if strings.TrimSpace(cfg.FeishuBotName) == "" {
		log.Printf("[larkbot] FEISHU_BOT_NAME is empty; group @ detection will rely on text_without_at_bot only")
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

	apiClient := lark.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret, lark.WithLogLevel(larkcore.LogLevelInfo))
	return &App{
		appID:       cfg.FeishuAppID,
		appSecret:   cfg.FeishuAppSecret,
		apiClient:   apiClient,
		responder:   responder,
		prefetcher:  prefetcher,
		attachments: attachmentManager,
		workspace:   workspaceManager,
		dedup:       &messageDedup{},
		dedupMaxAge: time.Duration(cfg.FeishuDedupTimeoutInMin) * time.Minute,
		bot:         botIdentity{name: cfg.FeishuBotName},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	a.workspace.StartBackgroundJobs(ctx)
	a.startDedupCleanup(ctx)

	cli := larkws.NewClient(a.appID, a.appSecret,
		larkws.WithEventHandler(a.newEventHandler()),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	log.Printf("[larkbot] starting websocket client...")
	return cli.Start(ctx)
}

func (a *App) startDedupCleanup(ctx context.Context) {
	if a.dedupMaxAge <= 0 {
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
				a.dedup.cleanup(a.dedupMaxAge)
			}
		}
	}()
}

func (a *App) newEventHandler() *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			log.Printf("[larkbot] message received: %s", larkcore.Prettify(event))

			messageID := extractMessageID(event)
			if messageID == "" {
				log.Printf("[larkbot] skip: empty message_id")
				return nil
			}
			if a.dedup.isDuplicate(messageID) {
				log.Printf("[larkbot] skip duplicate message_id=%s", messageID)
				return nil
			}
			if ok, reason := shouldHandleEvent(event, a.bot); !ok {
				log.Printf("[larkbot] skip message_id=%s: %s", messageID, reason)
				return nil
			}

			return withTypingReaction(ctx, a.apiClient, messageID, func() error {
				answer, err := a.answerEvent(ctx, event)
				if err != nil {
					log.Printf("[larkbot] handle event error: %v (message_id=%s)", err, messageID)
					answer = "Agent Error: " + err.Error()
				}

				reply, err := buildReplyBody(answer)
				if err != nil {
					return fmt.Errorf("build reply body: %w", err)
				}
				if err := replyMessage(ctx, a.apiClient, messageID, reply); err != nil {
					return fmt.Errorf("reply message: %w", err)
				}
				return nil
			})
		})
}
