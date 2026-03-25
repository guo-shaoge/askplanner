package larkbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/usage"
	"lab/askplanner/internal/workspace"
)

type responderClient interface {
	AnswerWithContext(ctx context.Context, conversationKey, question string, runtime codex.RuntimeContext) (string, error)
	GetModelState(conversationKey string) codex.ModelState
	SetModel(conversationKey, model string) (codex.ModelChangeResult, error)
	ResetModel(conversationKey string) (codex.ModelChangeResult, error)
	SetReasoningEffort(conversationKey, effort string) (codex.ModelChangeResult, error)
	ResetReasoningEffort(conversationKey string) (codex.ModelChangeResult, error)
	MarkWorkspaceChanged(userKey, sourceConversationKey string, notice codex.WorkspaceSessionNotice) error
}

type prefetcherService interface {
	Enrich(ctx context.Context, userKey, question string, runtime codex.RuntimeContext) (clinic.EnrichResult, error)
}

type workspaceService interface {
	Ensure(ctx context.Context, userKey string) (*workspace.Workspace, error)
	Status(ctx context.Context, userKey string) (*workspace.Workspace, error)
	SwitchRepo(ctx context.Context, userKey, repoName, ref string) (*workspace.Workspace, bool, error)
	Sync(ctx context.Context, userKey, repoName string) (*workspace.Workspace, bool, error)
	Reset(ctx context.Context, userKey, repoName string) (*workspace.Workspace, bool, error)
}

func (a *App) answerEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) (string, replyOptions, error) {
	start := time.Now()
	opts := replyOptions{preferThread: shouldReplyInThread(event)}
	prepared, err := prepareReply(ctx, a.apiClient, a.attachments, event)
	if err != nil {
		return "", opts, err
	}
	route := resolveConversationRoute(event, prepared.conversationKey, a.responder)
	prepared.conversationKey = route.key
	opts.preferThread = route.preferThread
	answer, err := handlePreparedReply(ctx, a.responder, a.prefetcher, a.workspace, a.tracker, prepared)
	if err != nil {
		return "", opts, err
	}
	log.Printf("[larkbot] handle event done message_id=%s conversation=%s elapsed=%s",
		extractMessageID(event), prepared.conversationKey, time.Since(start))
	return answer, opts, nil
}

// handlePreparedReply is the shared execution path after message parsing.
// Normal questions, /upload_N follow-up questions, and /ws ... -- question all
// flow through here so behavior stays aligned as the bot grows.
func handlePreparedReply(ctx context.Context, responder responderClient, prefetcher prefetcherService, workspaceManager workspaceService, tracker *usage.QuestionTracker, prepared *preparedReply) (string, error) {
	if prepared.skipCodex {
		return prepared.directReply, nil
	}
	if prepared.modelCmd != nil {
		return runModelCommand(ctx, workspaceManager, responder, prefetcher, tracker, prepared)
	}
	if prepared.workspaceCmd != nil {
		return runWorkspaceCommand(ctx, workspaceManager, responder, prefetcher, tracker, prepared)
	}

	ws, err := workspaceManager.Ensure(ctx, prepared.userKey)
	if err != nil {
		return "", err
	}
	answer, err := answerPreparedQuestion(ctx, responder, prefetcher, tracker, prepared, ws)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(prepared.prefix) != "" {
		return joinReplySections(prepared.prefix, answer), nil
	}
	return answer, nil
}

func runModelCommand(ctx context.Context, workspaceManager workspaceService, responder responderClient, prefetcher prefetcherService, tracker *usage.QuestionTracker, prepared *preparedReply) (string, error) {
	status := ""
	switch prepared.modelCmd.Action {
	case "status":
		status = codex.FormatModelStatus(responder.GetModelState(prepared.conversationKey), "", false)
	case "set":
		result, err := responder.SetModel(prepared.conversationKey, prepared.modelCmd.Model)
		if err != nil {
			return "", err
		}
		summary := "Model settings updated for this conversation."
		if !result.Changed {
			summary = "Model settings unchanged for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "effort":
		result, err := responder.SetReasoningEffort(prepared.conversationKey, prepared.modelCmd.Effort)
		if err != nil {
			return "", err
		}
		summary := "Reasoning effort updated for this conversation."
		if !result.Changed {
			summary = "Reasoning effort unchanged for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "reset-effort":
		result, err := responder.ResetReasoningEffort(prepared.conversationKey)
		if err != nil {
			return "", err
		}
		summary := "Reasoning effort override cleared for this conversation."
		if !result.Changed {
			summary = "Reasoning effort already uses the default for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "reset":
		result, err := responder.ResetModel(prepared.conversationKey)
		if err != nil {
			return "", err
		}
		summary := "Model override cleared for this conversation."
		if !result.Changed {
			summary = "Model settings already use the default for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	default:
		return "", fmt.Errorf("unsupported model command: %s", prepared.modelCmd.Action)
	}

	if strings.TrimSpace(prepared.question) == "" {
		return status, nil
	}

	ws, err := workspaceManager.Ensure(ctx, prepared.userKey)
	if err != nil {
		return "", err
	}
	answer, err := answerPreparedQuestion(ctx, responder, prefetcher, tracker, prepared, ws)
	if err != nil {
		return "", err
	}
	return joinReplySections(status, answer), nil
}

// runWorkspaceCommand keeps the user-facing status output coupled to the
// underlying workspace mutation, then optionally re-enters the normal answer
// pipeline with the updated workspace bound into runtime context.
func runWorkspaceCommand(ctx context.Context, manager workspaceService, responder responderClient, prefetcher prefetcherService, tracker *usage.QuestionTracker, prepared *preparedReply) (string, error) {
	start := time.Now()
	var (
		ws                 *workspace.Workspace
		err                error
		environmentChanged bool
	)
	switch prepared.workspaceCmd.Action {
	case "status":
		ws, err = manager.Status(ctx, prepared.userKey)
	case "switch":
		ws, environmentChanged, err = manager.SwitchRepo(ctx, prepared.userKey, prepared.workspaceCmd.Repo, prepared.workspaceCmd.Ref)
	case "sync":
		ws, environmentChanged, err = manager.Sync(ctx, prepared.userKey, prepared.workspaceCmd.Repo)
	case "reset":
		ws, environmentChanged, err = manager.Reset(ctx, prepared.userKey, prepared.workspaceCmd.Repo)
	default:
		return "", fmt.Errorf("unsupported workspace command: %s", prepared.workspaceCmd.Action)
	}
	if err != nil {
		return "", err
	}
	if environmentChanged {
		notice := codex.WorkspaceSessionNotice{
			Message:            buildWorkspaceChangeNotice(prepared.workspaceCmd),
			NewEnvironmentHash: ws.EnvironmentHash,
			ChangedAt:          time.Now().UTC(),
		}
		if err := responder.MarkWorkspaceChanged(prepared.userKey, prepared.conversationKey, notice); err != nil {
			log.Printf("[larkbot] mark workspace changed failed conversation=%s user=%s: %v",
				prepared.conversationKey, prepared.userKey, err)
		}
	}

	status := workspace.FormatStatus(ws)
	if strings.TrimSpace(prepared.question) == "" {
		return status, nil
	}

	answer, err := answerPreparedQuestion(ctx, responder, prefetcher, tracker, prepared, ws)
	if err != nil {
		return "", err
	}
	log.Printf("[larkbot] workspace command answered conversation=%s action=%s elapsed=%s",
		prepared.conversationKey, prepared.workspaceCmd.Action, time.Since(start))
	return joinReplySections(status, answer), nil
}

// answerPreparedQuestion owns the "question -> enrich runtime -> short-circuit
// intro reply -> Codex answer" sequence used by both regular and workspace
// flows. Keeping it centralized avoids subtle behavior drift.
func answerPreparedQuestion(ctx context.Context, responder responderClient, prefetcher prefetcherService, tracker *usage.QuestionTracker, prepared *preparedReply, ws *workspace.Workspace) (string, error) {
	question := strings.TrimSpace(prepared.question)
	if question == "" {
		question = "Please introduce your capabilities."
	}
	log.Printf("[larkbot] answering question: %q (conversation=%s)", question, prepared.conversationKey)
	span := tracker.Begin("lark", prepared.userKey, prepared.conversationKey, question, responder.GetModelState(prepared.conversationKey).EffectiveModel, workspaceEnvHash(ws))

	baseRuntime := workspace.BindRuntimeContext(codex.RuntimeContext{
		Attachment:   prepared.attachmentCtx,
		Thread:       prepared.threadCtx,
		ThreadLoader: prepared.threadCtxLoader,
		UserKey:      prepared.userKey,
	}, ws)
	enriched, err := prefetcher.Enrich(ctx, prepared.userKey, question, baseRuntime)
	if err != nil {
		if msg := clinic.UserFacingMessage(err); msg != "" {
			log.Printf("[larkbot] clinic prefetch user-visible error: %v (conversation=%s)",
				err, prepared.conversationKey)
			if span != nil {
				span.ShortCircuit()
			}
			return msg, nil
		}
		if span != nil {
			span.Error(err)
		}
		return "", err
	}

	enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
	if enriched.RuntimeContext.Thread == nil {
		enriched.RuntimeContext.Thread = prepared.threadCtx
	}
	if enriched.RuntimeContext.ThreadLoader == nil {
		enriched.RuntimeContext.ThreadLoader = prepared.threadCtxLoader
	}
	if strings.TrimSpace(enriched.RuntimeContext.UserKey) == "" {
		enriched.RuntimeContext.UserKey = prepared.userKey
	}
	if strings.TrimSpace(enriched.IntroReply) != "" {
		if span != nil {
			span.ShortCircuit()
		}
		return joinReplySections(enriched.IntroReply, formatWarning(enriched.Warning)), nil
	}
	answer, err := responder.AnswerWithContext(ctx, prepared.conversationKey, question, enriched.RuntimeContext)
	if err != nil {
		if span != nil {
			span.Error(err)
		}
		return "", err
	}
	if span != nil {
		span.Success()
	}
	if strings.TrimSpace(enriched.Warning) != "" {
		answer = joinReplySections(formatWarning(enriched.Warning), answer)
	}
	return answer, nil
}

func workspaceEnvHash(ws *workspace.Workspace) string {
	if ws == nil {
		return ""
	}
	return strings.TrimSpace(ws.EnvironmentHash)
}

func buildWorkspaceChangeNotice(cmd *workspace.Command) string {
	if cmd == nil {
		return "Another chat changed the shared workspace, so this conversation had to start a new Codex session."
	}
	switch cmd.Action {
	case "switch":
		return fmt.Sprintf("Another chat switched the shared workspace to %s@%s, so this conversation had to start a new Codex session.", cmd.Repo, cmd.Ref)
	case "sync":
		return fmt.Sprintf("Another chat synced the shared workspace repo %s, which changed the workspace environment, so this conversation had to start a new Codex session.", cmd.Repo)
	case "reset":
		return fmt.Sprintf("Another chat reset the shared workspace repo %s to its default ref, so this conversation had to start a new Codex session.", cmd.Repo)
	default:
		return "Another chat changed the shared workspace, so this conversation had to start a new Codex session."
	}
}
