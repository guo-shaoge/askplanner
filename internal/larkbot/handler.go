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
	"lab/askplanner/internal/workspace"
)

type responderClient interface {
	AnswerWithContext(ctx context.Context, conversationKey, question string, runtime codex.RuntimeContext) (string, error)
}

type prefetcherService interface {
	Enrich(ctx context.Context, userKey, question string, runtime codex.RuntimeContext) (clinic.EnrichResult, error)
}

type workspaceService interface {
	Ensure(ctx context.Context, userKey string) (*workspace.Workspace, error)
	Status(ctx context.Context, userKey string) (*workspace.Workspace, error)
	SwitchRepo(ctx context.Context, userKey, repoName, ref string) (*workspace.Workspace, error)
	Sync(ctx context.Context, userKey, repoName string) (*workspace.Workspace, error)
	Reset(ctx context.Context, userKey, repoName string) (*workspace.Workspace, error)
}

func (a *App) answerEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) (string, error) {
	start := time.Now()
	prepared, err := prepareReply(ctx, a.apiClient, a.attachments, event)
	if err != nil {
		return "", err
	}
	answer, err := handlePreparedReply(ctx, a.responder, a.prefetcher, a.workspace, prepared)
	if err != nil {
		return "", err
	}
	log.Printf("[larkbot] handle event done message_id=%s conversation=%s elapsed=%s",
		extractMessageID(event), prepared.conversationKey, time.Since(start))
	return answer, nil
}

// handlePreparedReply is the shared execution path after message parsing.
// Normal questions, /upload_N follow-up questions, and /ws ... -- question all
// flow through here so behavior stays aligned as the bot grows.
func handlePreparedReply(ctx context.Context, responder responderClient, prefetcher prefetcherService, workspaceManager workspaceService, prepared *preparedReply) (string, error) {
	if prepared.skipCodex {
		return prepared.directReply, nil
	}
	if prepared.workspaceCmd != nil {
		return runWorkspaceCommand(ctx, workspaceManager, responder, prefetcher, prepared)
	}

	ws, err := workspaceManager.Ensure(ctx, prepared.userKey)
	if err != nil {
		return "", err
	}
	answer, err := answerPreparedQuestion(ctx, responder, prefetcher, prepared, ws)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(prepared.prefix) != "" {
		return joinReplySections(prepared.prefix, answer), nil
	}
	return answer, nil
}

// runWorkspaceCommand keeps the user-facing status output coupled to the
// underlying workspace mutation, then optionally re-enters the normal answer
// pipeline with the updated workspace bound into runtime context.
func runWorkspaceCommand(ctx context.Context, manager workspaceService, responder responderClient, prefetcher prefetcherService, prepared *preparedReply) (string, error) {
	start := time.Now()
	var (
		ws  *workspace.Workspace
		err error
	)
	switch prepared.workspaceCmd.Action {
	case "status":
		ws, err = manager.Status(ctx, prepared.userKey)
	case "switch":
		ws, err = manager.SwitchRepo(ctx, prepared.userKey, prepared.workspaceCmd.Repo, prepared.workspaceCmd.Ref)
	case "sync":
		ws, err = manager.Sync(ctx, prepared.userKey, prepared.workspaceCmd.Repo)
	case "reset":
		ws, err = manager.Reset(ctx, prepared.userKey, prepared.workspaceCmd.Repo)
	default:
		return "", fmt.Errorf("unsupported workspace command: %s", prepared.workspaceCmd.Action)
	}
	if err != nil {
		return "", err
	}

	status := workspace.FormatStatus(ws)
	if strings.TrimSpace(prepared.question) == "" {
		return status, nil
	}

	answer, err := answerPreparedQuestion(ctx, responder, prefetcher, prepared, ws)
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
func answerPreparedQuestion(ctx context.Context, responder responderClient, prefetcher prefetcherService, prepared *preparedReply, ws *workspace.Workspace) (string, error) {
	question := strings.TrimSpace(prepared.question)
	if question == "" {
		question = "Please introduce your capabilities."
	}
	log.Printf("[larkbot] answering question: %q (conversation=%s)", question, prepared.conversationKey)

	baseRuntime := workspace.BindRuntimeContext(codex.RuntimeContext{
		Attachment:   prepared.attachmentCtx,
		Thread:       prepared.threadCtx,
		ThreadLoader: prepared.threadCtxLoader,
	}, ws)
	enriched, err := prefetcher.Enrich(ctx, prepared.userKey, question, baseRuntime)
	if err != nil {
		if msg := clinic.UserFacingMessage(err); msg != "" {
			log.Printf("[larkbot] clinic prefetch user-visible error: %v (conversation=%s)",
				err, prepared.conversationKey)
			return msg, nil
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
	if strings.TrimSpace(enriched.IntroReply) != "" {
		return joinReplySections(enriched.IntroReply, formatWarning(enriched.Warning)), nil
	}
	answer, err := responder.AnswerWithContext(ctx, prepared.conversationKey, question, enriched.RuntimeContext)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(enriched.Warning) != "" {
		answer = joinReplySections(formatWarning(enriched.Warning), answer)
	}
	return answer, nil
}
