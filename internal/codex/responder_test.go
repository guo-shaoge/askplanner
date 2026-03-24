package codex

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	newResult        *RunResult
	newErr           error
	resumeResult     *RunResult
	resumeErr        error
	newCalls         int
	resumeCalls      int
	lastNewPrompt    string
	lastResumePrompt string
}

func (f *fakeRunner) RunNew(ctx context.Context, workDir, prompt string) (*RunResult, error) {
	f.newCalls++
	f.lastNewPrompt = prompt
	if f.newErr != nil {
		return nil, f.newErr
	}
	if f.newResult != nil {
		return f.newResult, nil
	}
	return &RunResult{SessionID: "session-new", Answer: "new answer"}, nil
}

func (f *fakeRunner) RunResume(ctx context.Context, workDir, sessionID, prompt string) (*RunResult, error) {
	f.resumeCalls++
	f.lastResumePrompt = prompt
	if f.resumeErr != nil {
		return nil, f.resumeErr
	}
	if f.resumeResult != nil {
		return f.resumeResult, nil
	}
	return &RunResult{Answer: "resume answer"}, nil
}

func TestResponderLoadsThreadContextWhenResumeFallsBackToNewSession(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	responder := &Responder{
		runner:         &fakeRunner{resumeErr: errors.New("session missing")},
		store:          store,
		prompt:         prompt,
		promptHash:     PromptHash(prompt),
		defaultWorkDir: workDir,
		maxTurns:       30,
		sessionTTL:     time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-thread",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: PromptHash(workDir),
		CreatedAt:       now,
		LastActiveAt:    now,
		TurnCount:       1,
		Turns: []Turn{{
			User:      "older question",
			Assistant: "older answer",
			At:        now,
		}},
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("store.Put returned error: %v", err)
	}

	loads := 0
	answer, err := responder.AnswerWithContext(context.Background(), "conv-thread", "latest question", RuntimeContext{
		ThreadLoader: func(ctx context.Context) (*ThreadContext, error) {
			loads++
			return &ThreadContext{
				ThreadID: "omt-thread",
				Messages: []ThreadMessage{{
					MessageID:   "om_prev",
					SenderLabel: "user:ou_prev",
					MessageType: "text",
					Content:     "earlier thread message",
				}},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if answer != "new answer" {
		t.Fatalf("answer = %q, want %q", answer, "new answer")
	}

	runner := responder.runner.(*fakeRunner)
	if runner.resumeCalls != 1 {
		t.Fatalf("resume calls = %d, want 1", runner.resumeCalls)
	}
	if runner.newCalls != 1 {
		t.Fatalf("new calls = %d, want 1", runner.newCalls)
	}
	if loads != 1 {
		t.Fatalf("thread loader calls = %d, want 1", loads)
	}
	if !strings.Contains(runner.lastNewPrompt, "thread_id=omt-thread") {
		t.Fatalf("new prompt missing thread context:\n%s", runner.lastNewPrompt)
	}
}

func TestResponderSkipsThreadLoaderOnSuccessfulResume(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	responder := &Responder{
		runner: &fakeRunner{
			resumeResult: &RunResult{Answer: "resume answer"},
		},
		store:          store,
		prompt:         prompt,
		promptHash:     PromptHash(prompt),
		defaultWorkDir: workDir,
		maxTurns:       30,
		sessionTTL:     time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-resume",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: PromptHash(workDir),
		CreatedAt:       now,
		LastActiveAt:    now,
		TurnCount:       1,
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("store.Put returned error: %v", err)
	}

	loads := 0
	answer, err := responder.AnswerWithContext(context.Background(), "conv-resume", "latest question", RuntimeContext{
		ThreadLoader: func(ctx context.Context) (*ThreadContext, error) {
			loads++
			return &ThreadContext{ThreadID: "omt-thread"}, nil
		},
	})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if answer != "resume answer" {
		t.Fatalf("answer = %q, want %q", answer, "resume answer")
	}

	runner := responder.runner.(*fakeRunner)
	if runner.resumeCalls != 1 {
		t.Fatalf("resume calls = %d, want 1", runner.resumeCalls)
	}
	if runner.newCalls != 0 {
		t.Fatalf("new calls = %d, want 0", runner.newCalls)
	}
	if loads != 0 {
		t.Fatalf("thread loader calls = %d, want 0", loads)
	}
}

func TestAppendAnswerWarning(t *testing.T) {
	got := appendAnswerWarning("final answer", "session history was not saved")
	if !strings.Contains(got, "final answer") || !strings.Contains(got, "Warning: session history was not saved") {
		t.Fatalf("appendAnswerWarning() = %q", got)
	}
}

func TestResponderMarksWorkspaceChangedForOtherConversations(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	responder := &Responder{store: store}
	now := time.Now().UTC()
	records := []SessionRecord{
		{ConversationKey: "lark:chat:oc_1:user:ou_user", EnvironmentHash: "env-old", LastActiveAt: now},
		{ConversationKey: "lark:chat:oc_2:user:ou_user", UserKey: "ou_user", EnvironmentHash: "env-new", LastActiveAt: now},
		{ConversationKey: "conv-c", UserKey: "other", EnvironmentHash: "env-old", LastActiveAt: now},
	}
	for _, record := range records {
		if err := store.Put(record); err != nil {
			t.Fatalf("store.Put returned error: %v", err)
		}
	}

	err = responder.MarkWorkspaceChanged("ou_user", "lark:chat:oc_2:user:ou_user", WorkspaceSessionNotice{
		Message:            "workspace changed",
		NewEnvironmentHash: "env-new",
		ChangedAt:          now,
	})
	if err != nil {
		t.Fatalf("MarkWorkspaceChanged returned error: %v", err)
	}

	record, _ := store.Get("lark:chat:oc_1:user:ou_user")
	if record.PendingNotice == nil || record.PendingNotice.Message != "workspace changed" {
		t.Fatalf("conv-a pending notice = %+v", record.PendingNotice)
	}
	record, _ = store.Get("lark:chat:oc_2:user:ou_user")
	if record.PendingNotice != nil {
		t.Fatalf("conv-b pending notice = %+v, want nil", record.PendingNotice)
	}
	record, _ = store.Get("conv-c")
	if record.PendingNotice != nil {
		t.Fatalf("conv-c pending notice = %+v, want nil", record.PendingNotice)
	}
}

func TestResponderShowsPendingWorkspaceNoticeOnEnvironmentChange(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	responder := &Responder{
		runner:         &fakeRunner{newResult: &RunResult{SessionID: "session-new", Answer: "new answer"}},
		store:          store,
		prompt:         prompt,
		promptHash:     PromptHash(prompt),
		defaultWorkDir: workDir,
		maxTurns:       30,
		sessionTTL:     time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-env",
		UserKey:         "ou_user",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: "env-old",
		PendingNotice: &WorkspaceSessionNotice{
			Message:            "workspace changed",
			NewEnvironmentHash: "env-new",
			ChangedAt:          now,
		},
		CreatedAt:    now,
		LastActiveAt: now,
		TurnCount:    1,
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("store.Put returned error: %v", err)
	}

	answer, err := responder.AnswerWithContext(context.Background(), "conv-env", "latest question", RuntimeContext{
		UserKey: "ou_user",
		Workspace: &WorkspaceContext{
			RootDir:         workDir,
			EnvironmentHash: "env-new",
		},
	})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if !strings.HasPrefix(answer, "Warning: workspace changed") {
		t.Fatalf("answer = %q, want prefixed warning", answer)
	}
	record, _ = store.Get("conv-env")
	if record.PendingNotice != nil {
		t.Fatalf("pending notice = %+v, want nil", record.PendingNotice)
	}
	if record.EnvironmentHash != "env-new" {
		t.Fatalf("environment hash = %q, want env-new", record.EnvironmentHash)
	}
}
