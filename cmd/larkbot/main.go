package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	larkrelay "lab/askplanner/internal/lark"
)

type messageDedup struct {
	seen sync.Map // messageId -> time.Time
}

func (d *messageDedup) isDuplicate(messageID string) bool {
	_, loaded := d.seen.LoadOrStore(messageID, time.Now())
	return loaded
}

func (d *messageDedup) cleanup(maxAge time.Duration) {
	now := time.Now()
	d.seen.Range(func(key, value any) bool {
		if now.Sub(value.(time.Time)) > maxAge {
			d.seen.Delete(key)
		}
		return true
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logFile, err := config.SetupLogging(cfg.LogFile)
	if err != nil {
		log.Fatalf("setup logging: %v", err)
	}
	defer logFile.Close()

	if cfg.FeishuAppID == "" || cfg.FeishuAppSecret == "" {
		log.Fatalf("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
	}

	responder, err := codex.NewResponder(cfg)
	if err != nil {
		log.Fatalf("build codex responder: %v", err)
	}

	apiClient := lark.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret, lark.WithLogLevel(larkcore.LogLevelInfo))
	intake := larkrelay.NewIntake(
		apiClient.Im.V1.MessageResource,
		cfg.FeishuAttachmentRoot,
		time.Duration(cfg.FeishuAttachmentTTLMin)*time.Minute,
		int64(cfg.FeishuAttachmentMaxBytes),
	)
	cleaner := &larkrelay.Cleaner{Root: cfg.FeishuAttachmentRoot}

	dedup := &messageDedup{}
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			// six hours timeout
			dedup.cleanup(time.Duration(cfg.FeishuDedupTimeoutInMin) * time.Minute)
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.FeishuAttachmentCleanup) * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := cleaner.CleanupExpired(time.Now().UTC()); err != nil {
				log.Printf("[larkbot] cleanup attachments: %v", err)
			}
		}
	}()

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			log.Printf("[larkbot] message received: %s", larkcore.Prettify(event))

			messageID := extractMessageID(event)
			if messageID == "" {
				log.Printf("[larkbot] skip: empty message_id")
				return nil
			}

			if dedup.isDuplicate(messageID) {
				log.Printf("[larkbot] skip duplicate message_id=%s", messageID)
				return nil
			}

			conversationKey := buildConversationKey(event)
			request, err := intake.BuildRequest(ctx, conversationKey, event)
			if err != nil {
				if userErr, ok := larkrelay.AsUserVisibleError(err); ok {
					content, buildErr := buildTextContent(userErr.Message)
					if buildErr != nil {
						return fmt.Errorf("build reply content: %w", buildErr)
					}
					return replyMessage(ctx, apiClient, messageID, content)
				}
				errMsg := fmt.Sprintf("[larkbot] intake error: %v (message_id=%s, conversation=%s)",
					err, messageID, conversationKey)
				log.Print(errMsg)
				// todo return error by user text
				return fmt.Errorf("[larkbot] intake error: %v", err)
			}

			log.Printf("[larkbot] answering message_id=%s conversation=%s user_message=%q bundle_root=%s",
				messageID, conversationKey, request.UserMessage, filepath.Join(cfg.FeishuAttachmentRoot, conversationKey))

			answer, err := responder.Answer(ctx, conversationKey, request)
			if err != nil {
				log.Printf("[larkbot] agent error: %v (message_id=%s)", err, messageID)
				answer = "Agent Error: " + err.Error()
			}

			content, err := buildTextContent(answer)
			if err != nil {
				return fmt.Errorf("build reply content: %w", err)
			}

			if err := replyMessage(ctx, apiClient, messageID, content); err != nil {
				return fmt.Errorf("reply message: %w", err)
			}

			return nil
		})

	cli := larkws.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	log.Printf("[larkbot] starting websocket client...")
	if err := cli.Start(context.Background()); err != nil {
		log.Fatalf("lark websocket start: %v", err)
	}
}

func extractMessageID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.MessageId == nil {
		return ""
	}
	return *event.Event.Message.MessageId
}

func buildConversationKey(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil {
		return "lark:unknown"
	}

	var threadID string
	var chatID string
	var senderID string
	var messageID string

	if event.Event.Message != nil {
		if event.Event.Message.ThreadId != nil {
			threadID = strings.TrimSpace(*event.Event.Message.ThreadId)
		}
		if event.Event.Message.ChatId != nil {
			chatID = strings.TrimSpace(*event.Event.Message.ChatId)
		}
		if event.Event.Message.MessageId != nil {
			messageID = strings.TrimSpace(*event.Event.Message.MessageId)
		}
	}
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		if event.Event.Sender.SenderId.OpenId != nil {
			senderID = strings.TrimSpace(*event.Event.Sender.SenderId.OpenId)
		} else if event.Event.Sender.SenderId.UserId != nil {
			senderID = strings.TrimSpace(*event.Event.Sender.SenderId.UserId)
		}
	}

	switch {
	case threadID != "":
		return "lark:thread:" + threadID
	case chatID != "" && senderID != "":
		return "lark:chat:" + chatID + ":user:" + senderID
	case chatID != "":
		return "lark:chat:" + chatID
	case messageID != "":
		return "lark:message:" + messageID
	default:
		return "lark:unknown"
	}
}

func buildTextContent(text string) (string, error) {
	payload := map[string]string{
		"text": strings.TrimSpace(text),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func replyMessage(ctx context.Context, apiClient *lark.Client, messageID, content string) error {
	log.Printf("[larkbot] replying to message_id=%s", messageID)
	resp, err := apiClient.Im.V1.Message.Reply(ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("text").
				Content(content).
				Uuid("reply-"+messageID).
				Build()).
			Build())
	if err != nil {
		return fmt.Errorf("call reply API: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("reply API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	log.Printf("[larkbot] reply sent (message_id=%s)", messageID)
	return nil
}
