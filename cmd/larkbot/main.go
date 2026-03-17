package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"lab/askplanner/internal/askplanner"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	appID := mustGetEnv("FEISHU_APP_ID")
	appSecret := mustGetEnv("FEISHU_APP_SECRET")

	agent, err := buildAgent()
	if err != nil {
		log.Fatalf("build agent: %v", err)
	}

	apiClient := lark.NewClient(appID, appSecret, lark.WithLogLevel(larkcore.LogLevelInfo))

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			log.Printf("[larkbot] message received: %s", larkcore.Prettify(event))

			messageID := extractMessageID(event)
			if messageID == "" {
				log.Printf("[larkbot] skip: empty message_id")
				return nil
			}

			question := extractQuestion(event)
			if question == "" {
				question = "Please introduce your capabilities."
			}

			log.Printf("[larkbot] answering question: %q (message_id=%s)", question, messageID)

			answer, err := agent.Answer(ctx, question, func(toolName, args string) {
				log.Printf("[larkbot]   [tool] %s(%s)", toolName, truncate(args, 100))
			})
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

	cli := larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	log.Printf("[larkbot] starting websocket client...")
	if err := cli.Start(context.Background()); err != nil {
		log.Fatalf("lark websocket start: %v", err)
	}
}

func buildAgent() (*askplanner.Agent, error) {
	cfg, err := askplanner.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var provider askplanner.Provider
	switch cfg.LLMProvider {
	case "kimi":
		provider = askplanner.NewKimiProvider(cfg.KimiAPIKey, cfg.KimiModel, cfg.KimiBaseURL)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLMProvider)
	}

	skillIdx, err := askplanner.BuildIndex(cfg.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("build skill index: %w", err)
	}
	log.Printf("[larkbot] skills loaded: %d core, %d oncall, %d customer issues",
		len(skillIdx.CoreFiles), len(skillIdx.OncallFiles), skillIdx.CustomerIssues)

	sandbox := askplanner.NewSandbox(cfg.ProjectRoot, []string{
		"contrib/agent-rules/skills/tidb-query-tuning/references",
		"contrib/tidb/pkg/planner",
		"contrib/tidb/pkg/statistics",
		"contrib/tidb/pkg/expression",
		"contrib/tidb/pkg/parser",
		"contrib/tidb/.agents/skills",
		"contrib/tidb/AGENTS.md",
	})

	toolReg := askplanner.NewRegistry(
		askplanner.NewReadFileTool(sandbox),
		askplanner.NewSearchCodeTool(sandbox, "contrib/tidb/pkg/planner"),
		askplanner.NewListDirTool(sandbox),
		askplanner.NewListSkillsTool(cfg.SkillsDir),
		askplanner.NewReadSkillTool(cfg.SkillsDir),
	)

	return askplanner.New(askplanner.AgentConfig{
		Provider:       provider,
		ToolRegistry:   toolReg,
		SkillIndex:     skillIdx,
		Temperature:    cfg.Temperature,
		MaxToolSteps:   cfg.MaxToolSteps,
		MaxResultChars: cfg.MaxResultChars,
		StepDelay:      time.Duration(cfg.StepDelayMS) * time.Millisecond,
	}), nil
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func extractMessageID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.MessageId == nil {
		return ""
	}
	return *event.Event.Message.MessageId
}

func extractQuestion(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.Content == nil {
		return ""
	}

	raw := strings.TrimSpace(*event.Event.Message.Content)
	if raw == "" {
		return ""
	}

	if event.Event.Message.MessageType != nil && *event.Event.Message.MessageType == "text" {
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err == nil {
			return strings.TrimSpace(payload.Text)
		}
	}
	return raw
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
