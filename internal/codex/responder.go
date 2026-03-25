package codex

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"lab/askplanner/internal/config"
)

type Responder struct {
	runner         *Runner
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
	if ok && canResume {
		log.Printf("[codex] resume eligible conversation=%s session=%s", conversationKey, record.SessionID)
		result, err := r.runner.RunResume(ctx, workDir, record.SessionID, BuildResumePrompt(question, runtime))
		if err == nil {
			record.LastActiveAt = now
			record.TurnCount++
			record.LastError = ""
			record.appendTurn(question, result.Answer)
			if err := r.store.Put(record); err != nil {
				return "", err
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

	initialPrompt := BuildInitialPrompt(r.prompt, summarizeTurns(record.Turns), question, runtime)
	result, err := r.runner.RunNew(ctx, workDir, initialPrompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.SessionID) == "" {
		return "", fmt.Errorf("codex did not return a session id")
	}

	record = SessionRecord{
		ConversationKey: conversationKey,
		SessionID:       result.SessionID,
		PromptHash:      r.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: envHash,
		CreatedAt:       now,
		LastActiveAt:    now,
		TurnCount:       1,
		Turns: []Turn{{
			User:      strings.TrimSpace(question),
			Assistant: strings.TrimSpace(result.Answer),
			At:        now,
		}},
	}
	if err := r.store.Put(record); err != nil {
		return "", err
	}
	log.Printf("[codex] answer done conversation=%s mode=new session=%s elapsed=%s",
		conversationKey, result.SessionID, time.Since(start))
	return result.Answer, nil
}

func (r *Responder) Reset(conversationKey string) error {
	return r.store.Delete(conversationKey)
}

// ResolveExistingConversationKey returns the preferred key when it already has
// a stored session; otherwise it returns the first fallback candidate that
// already exists. The caller decides whether that fallback means "legacy".
func (r *Responder) ResolveExistingConversationKey(preferred string, candidates ...string) (string, bool) {
	preferred = strings.TrimSpace(preferred)
	if preferred == "" {
		return preferred, false
	}
	if _, ok := r.store.Get(preferred); ok {
		return preferred, false
	}

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == preferred {
			continue
		}
		if _, ok := r.store.Get(candidate); ok {
			return candidate, true
		}
	}
	return preferred, false
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
