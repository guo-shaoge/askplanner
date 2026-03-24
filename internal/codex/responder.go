package codex

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"lab/askplanner/internal/config"
	"lab/askplanner/internal/usererr"
)

type runnerClient interface {
	RunNew(ctx context.Context, workDir, prompt string) (*RunResult, error)
	RunResume(ctx context.Context, workDir, sessionID, prompt string) (*RunResult, error)
}

type Responder struct {
	runner         runnerClient
	store          *FileSessionStore
	prompt         string
	promptHash     string
	defaultWorkDir string
	maxTurns       int
	sessionTTL     time.Duration
	timeout        time.Duration
}

func NewResponder(cfg *config.Config) (*Responder, error) {
	prompt, err := LoadPrompt(cfg.PromptFile)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	store, err := NewFileSessionStore(cfg.CodexSessionStore)
	if err != nil {
		return nil, fmt.Errorf("init session store: %w", err)
	}

	return &Responder{
		runner: &Runner{
			Bin:             cfg.CodexBin,
			Model:           cfg.CodexModel,
			ReasoningEffort: cfg.CodexReasoningEffort,
			Sandbox:         cfg.CodexSandbox,
		},
		store:          store,
		prompt:         prompt,
		promptHash:     PromptHash(prompt),
		defaultWorkDir: cfg.ProjectRoot,
		maxTurns:       cfg.CodexMaxTurns,
		sessionTTL:     time.Duration(cfg.CodexSessionTTLHours) * time.Hour,
		timeout:        time.Duration(cfg.CodexTimeoutSec) * time.Second,
	}, nil
}

func (r *Responder) Answer(ctx context.Context, conversationKey, question string) (string, error) {
	return r.AnswerWithContext(ctx, conversationKey, question, RuntimeContext{})
}

func (r *Responder) AnswerWithContext(ctx context.Context, conversationKey, question string, runtime RuntimeContext) (string, error) {
	start := time.Now()
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	now := time.Now().UTC()
	record, ok := r.store.Get(conversationKey)
	workDir := r.workDirForRuntime(runtime)
	envHash := r.environmentHashForRuntime(runtime, workDir)
	log.Printf("[codex] answer start conversation=%s question_len=%d workdir=%s env=%s session_found=%t",
		conversationKey, len(strings.TrimSpace(question)), workDir, compactText(envHash, 12), ok)

	canResume, resumeReason := r.canResume(record, now, workDir, envHash)
	pendingNotice := r.pendingWorkspaceNotice(record, envHash, resumeReason)
	if ok && canResume {
		log.Printf("[codex] resume eligible conversation=%s session=%s", conversationKey, record.SessionID)
		result, err := r.runner.RunResume(ctx, workDir, record.SessionID, BuildResumePrompt(question, runtime))
		if err == nil {
			record.UserKey = strings.TrimSpace(runtime.UserKey)
			record.PendingNotice = nil
			record.LastActiveAt = now
			record.TurnCount++
			record.LastError = ""
			record.EnvironmentHash = envHash
			record.appendTurn(question, result.Answer)
			if err := r.store.Put(record); err != nil {
				log.Printf("[codex] persist session after resume failed for %s: %v", conversationKey, err)
				return appendAnswerWarning(result.Answer, buildSessionStoreWarning("Agent answered this turn, but couldn't save the local session history. Follow-up turns may start a new session.", err)), nil
			}
			log.Printf("[codex] answer done conversation=%s mode=resume elapsed=%s", conversationKey, time.Since(start))
			return result.Answer, nil
		}
		record.LastError = err.Error()
		if saveErr := r.store.Put(record); saveErr != nil {
			log.Printf("[codex] persist resume failure for %s: %v", conversationKey, saveErr)
		}
		log.Printf("[codex] resume failed for %s, starting a new session: %v", conversationKey, err)
	} else if ok {
		log.Printf("[codex] resume skipped for %s: %s", conversationKey, resumeReason)
	}

	runtime = r.hydrateInitialRuntime(ctx, conversationKey, runtime)
	initialPrompt := BuildInitialPrompt(r.prompt, summarizeTurns(record.Turns), question, runtime)
	result, err := r.runner.RunNew(ctx, workDir, initialPrompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.SessionID) == "" {
		return "", usererr.New(usererr.KindUnavailable, "Codex did not return a session ID. Please retry.")
	}

	record = SessionRecord{
		ConversationKey: conversationKey,
		UserKey:         strings.TrimSpace(runtime.UserKey),
		SessionID:       result.SessionID,
		PromptHash:      r.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: envHash,
		CreatedAt:       now,
		LastActiveAt:    now,
		PendingNotice:   nil,
		TurnCount:       1,
		Turns: []Turn{{
			User:      strings.TrimSpace(question),
			Assistant: strings.TrimSpace(result.Answer),
			At:        now,
		}},
	}
	if err := r.store.Put(record); err != nil {
		log.Printf("[codex] persist new session failed for %s: %v", conversationKey, err)
		return appendAnswerWarning(result.Answer, buildSessionStoreWarning("Agent answered this turn, but couldn't save the local session history. Follow-up turns may start a new session.", err)), nil
	}
	log.Printf("[codex] answer done conversation=%s mode=new session=%s elapsed=%s",
		conversationKey, result.SessionID, time.Since(start))
	return prependAnswerWarning(result.Answer, pendingNotice), nil
}

func (r *Responder) Reset(conversationKey string) error {
	if err := r.store.Delete(conversationKey); err != nil {
		return usererr.WrapLocalStorage("Agent couldn't reset the local session history. Please retry.", err)
	}
	return nil
}

func (r *Responder) ResetByWorkDirPrefix(workDirPrefix string) (int, error) {
	workDirPrefix = strings.TrimSpace(workDirPrefix)
	if workDirPrefix == "" {
		return 0, fmt.Errorf("workdir prefix is empty")
	}
	cleanPrefix := filepath.Clean(workDirPrefix)
	return r.store.DeleteIf(func(record SessionRecord) bool {
		workDir := strings.TrimSpace(record.WorkDir)
		if workDir == "" {
			return false
		}
		workDir = filepath.Clean(workDir)
		if workDir == cleanPrefix {
			return true
		}
		return strings.HasPrefix(workDir, cleanPrefix+string(filepath.Separator))
	})
}

func (r *Responder) MarkWorkspaceChanged(userKey, sourceConversationKey string, notice WorkspaceSessionNotice) error {
	userKey = strings.TrimSpace(userKey)
	sourceConversationKey = strings.TrimSpace(sourceConversationKey)
	notice.Message = strings.TrimSpace(notice.Message)
	notice.NewEnvironmentHash = strings.TrimSpace(notice.NewEnvironmentHash)
	if userKey == "" || sourceConversationKey == "" || notice.Message == "" || notice.NewEnvironmentHash == "" {
		return nil
	}
	if notice.ChangedAt.IsZero() {
		notice.ChangedAt = time.Now().UTC()
	}
	_, err := r.store.UpdateIf(func(record SessionRecord) bool {
		return sessionRecordUserKey(record) == userKey &&
			strings.TrimSpace(record.ConversationKey) != sourceConversationKey &&
			strings.TrimSpace(record.EnvironmentHash) != notice.NewEnvironmentHash
	}, func(record *SessionRecord) bool {
		record.PendingNotice = &WorkspaceSessionNotice{
			Message:            notice.Message,
			NewEnvironmentHash: notice.NewEnvironmentHash,
			ChangedAt:          notice.ChangedAt,
		}
		return true
	})
	return err
}

func (r *Responder) canResume(record SessionRecord, now time.Time, workDir, envHash string) (bool, string) {
	if strings.TrimSpace(record.SessionID) == "" {
		return false, "empty_session_id"
	}
	if record.PromptHash != r.promptHash {
		return false, "prompt_hash_changed"
	}
	if record.WorkDir != workDir {
		return false, "workdir_changed"
	}
	if strings.TrimSpace(record.EnvironmentHash) != strings.TrimSpace(envHash) {
		return false, "environment_changed"
	}
	if r.maxTurns > 0 && record.TurnCount >= r.maxTurns {
		return false, "max_turns_reached"
	}
	if r.sessionTTL > 0 && now.Sub(record.LastActiveAt) > r.sessionTTL {
		return false, "session_ttl_expired"
	}
	return true, "ok"
}

func (r *Responder) workDirForRuntime(runtime RuntimeContext) string {
	if runtime.Workspace != nil && strings.TrimSpace(runtime.Workspace.RootDir) != "" {
		return strings.TrimSpace(runtime.Workspace.RootDir)
	}
	return r.defaultWorkDir
}

func (r *Responder) environmentHashForRuntime(runtime RuntimeContext, workDir string) string {
	if runtime.Workspace != nil && strings.TrimSpace(runtime.Workspace.EnvironmentHash) != "" {
		return strings.TrimSpace(runtime.Workspace.EnvironmentHash)
	}
	return PromptHash(workDir)
}

func (r *Responder) hydrateInitialRuntime(ctx context.Context, conversationKey string, runtime RuntimeContext) RuntimeContext {
	if runtime.Thread != nil || runtime.ThreadLoader == nil {
		return runtime
	}
	threadCtx, err := runtime.ThreadLoader(ctx)
	if err != nil {
		log.Printf("[codex] initial thread context unavailable conversation=%s: %v", conversationKey, err)
		return runtime
	}
	runtime.Thread = threadCtx
	runtime.ThreadLoader = nil
	return runtime
}

func summarizeTurns(turns []Turn) string {
	if len(turns) == 0 {
		return ""
	}
	if len(turns) > 6 {
		turns = turns[len(turns)-6:]
	}

	var sb strings.Builder
	for i, turn := range turns {
		fmt.Fprintf(&sb, "Turn %d user: %s\n", i+1, compactText(turn.User, 300))
		fmt.Fprintf(&sb, "Turn %d assistant: %s\n", i+1, compactText(turn.Assistant, 500))
	}
	return strings.TrimSpace(sb.String())
}

func compactText(s string, max int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func appendAnswerWarning(answer, warning string) string {
	answer = strings.TrimSpace(answer)
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return answer
	}
	if answer == "" {
		return formatAnswerWarning(warning)
	}
	return answer + "\n\n" + formatAnswerWarning(warning)
}

func prependAnswerWarning(answer, warning string) string {
	answer = strings.TrimSpace(answer)
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return answer
	}
	if answer == "" {
		return formatAnswerWarning(warning)
	}
	return formatAnswerWarning(warning) + "\n\n" + answer
}

func buildSessionStoreWarning(message string, err error) string {
	return usererr.OrDefault(usererr.WrapLocalStorage(message, err), message)
}

func (r *Responder) pendingWorkspaceNotice(record SessionRecord, envHash, resumeReason string) string {
	if resumeReason != "environment_changed" || record.PendingNotice == nil {
		return ""
	}
	if strings.TrimSpace(record.PendingNotice.NewEnvironmentHash) != strings.TrimSpace(envHash) {
		return ""
	}
	return strings.TrimSpace(record.PendingNotice.Message)
}

func sessionRecordUserKey(record SessionRecord) string {
	if userKey := strings.TrimSpace(record.UserKey); userKey != "" {
		return userKey
	}
	return parseConversationUserKey(record.ConversationKey)
}

func parseConversationUserKey(conversationKey string) string {
	conversationKey = strings.TrimSpace(conversationKey)
	if conversationKey == "" {
		return ""
	}
	const marker = ":user:"
	idx := strings.LastIndex(conversationKey, marker)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(conversationKey[idx+len(marker):])
}

func formatAnswerWarning(warning string) string {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return ""
	}
	return "**Warning:** " + warning + " The bot has lost your previous conversation content!!!"
}
