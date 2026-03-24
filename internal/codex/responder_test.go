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
<<<<<<< HEAD
=======

func TestAppendAnswerWarning(t *testing.T) {
	got := appendAnswerWarning("final answer", "session history was not saved")
	if !strings.Contains(got, "final answer") || !strings.Contains(got, "Warning: session history was not saved") {
		t.Fatalf("appendAnswerWarning() = %q", got)
	}
}
>>>>>>> 72d3b4494b14c38d3696ac4c22e42ec9b798e508
