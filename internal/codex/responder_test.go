package codex

import (
	"context"
	"errors"
	"os"
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
	lastNewModel     string
	lastNewEffort    string
	lastNewPrompt    string
	lastResumeModel  string
	lastResumeEffort string
	lastResumePrompt string
}

func (f *fakeRunner) RunNew(ctx context.Context, workDir, model, reasoningEffort, prompt string) (*RunResult, error) {
	f.newCalls++
	f.lastNewModel = model
	f.lastNewEffort = reasoningEffort
	f.lastNewPrompt = prompt
	if f.newErr != nil {
		return nil, f.newErr
	}
	if f.newResult != nil {
		return f.newResult, nil
	}
	return &RunResult{SessionID: "session-new", Answer: "new answer"}, nil
}

func (f *fakeRunner) RunResume(ctx context.Context, workDir, sessionID, model, reasoningEffort, prompt string) (*RunResult, error) {
	f.resumeCalls++
	f.lastResumeModel = model
	f.lastResumeEffort = reasoningEffort
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
		runner:                 &fakeRunner{resumeErr: errors.New("session missing")},
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-thread",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: responder.environmentHashForRuntime(RuntimeContext{}, workDir),
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
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-resume",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: responder.environmentHashForRuntime(RuntimeContext{}, workDir),
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

func TestFormatModelStatusIncludesOptionsAndExample(t *testing.T) {
	got := FormatModelStatus(ModelState{
		DefaultModel:           "gpt-5.3-codex",
		EffectiveModel:         "gpt-5.3-codex",
		DefaultReasoningEffort: "medium",
		ModelOptions: []ModelOption{
			{Slug: "gpt-5.3-codex", Description: "Frontier Codex-optimized agentic coding model."},
			{Slug: "gpt-5.4", Description: "Latest frontier agentic coding model."},
			{Slug: "gpt-5.4-mini", Description: "Smaller frontier agentic coding model."},
		},
		ReasoningEffort:  "medium",
		ReasoningOptions: []ReasoningEffortOption{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}},
	}, "", false)
	for _, part := range []string{
		"Options:",
		"1. gpt-5.3-codex (current)  Frontier Codex-optimized agentic coding model.",
		"2. gpt-5.4  Latest frontier agentic coding model.",
		"Model: gpt-5.3-codex",
		"Reasoning: medium",
		"Reasoning Options: low, medium, high",
		"Set: /model set <model>",
		"Set Effort: /model effort <level>",
		"Reset: /model reset",
		"Reset Effort: /model effort reset",
		"Example: /model set gpt-5.4",
		"Effort Example: /model effort low",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("FormatModelStatus() missing %q in:\n%s", part, got)
		}
	}
}

func TestResponderUsesConversationModelOverride(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	runner := &fakeRunner{}
	responder := &Responder{
		runner:                 runner,
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	result, err := responder.SetModel("conv-model", "gpt-5.4")
	if err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected SetModel to report changed")
	}

	answer, err := responder.AnswerWithContext(context.Background(), "conv-model", "latest question", RuntimeContext{})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if answer != "new answer" {
		t.Fatalf("answer = %q, want %q", answer, "new answer")
	}
	if runner.lastNewModel != "gpt-5.4" {
		t.Fatalf("new model = %q, want gpt-5.4", runner.lastNewModel)
	}
	if runner.lastNewEffort != "medium" {
		t.Fatalf("new effort = %q, want medium", runner.lastNewEffort)
	}
}

func TestResponderResetPreservesModelOverride(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	responder := &Responder{
		runner:                 &fakeRunner{},
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-reset",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: responder.environmentHashForRuntime(RuntimeContext{}, workDir),
		ModelOverride:   "gpt-5.4",
		CreatedAt:       now,
		LastActiveAt:    now,
		TurnCount:       1,
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("store.Put returned error: %v", err)
	}

	if err := responder.Reset("conv-reset"); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}

	got, ok := store.Get("conv-reset")
	if !ok {
		t.Fatalf("expected record to remain after reset")
	}
	if got.ModelOverride != "gpt-5.4" {
		t.Fatalf("model override = %q, want gpt-5.4", got.ModelOverride)
	}
	if got.ReasoningEffortOverride != "" {
		t.Fatalf("reasoning effort override = %q, want empty", got.ReasoningEffortOverride)
	}
	if got.SessionID != "" || got.TurnCount != 0 {
		t.Fatalf("expected session state to be cleared, got %+v", got)
	}
}

func TestResponderResumesWhenModelChanges(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	runner := &fakeRunner{}
	responder := &Responder{
		runner:                 runner,
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	now := time.Now().UTC()
	record := SessionRecord{
		ConversationKey: "conv-switch-model",
		SessionID:       "session-old",
		PromptHash:      responder.promptHash,
		WorkDir:         workDir,
		EnvironmentHash: responder.environmentHashForRuntime(RuntimeContext{}, workDir),
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
	if _, err := responder.SetModel("conv-switch-model", "gpt-5.4"); err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}

	answer, err := responder.AnswerWithContext(context.Background(), "conv-switch-model", "latest question", RuntimeContext{})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if answer != "resume answer" {
		t.Fatalf("answer = %q, want %q", answer, "resume answer")
	}
	if runner.resumeCalls != 1 {
		t.Fatalf("resume calls = %d, want 1", runner.resumeCalls)
	}
	if runner.newCalls != 0 {
		t.Fatalf("new calls = %d, want 0", runner.newCalls)
	}
	if runner.lastResumeModel != "gpt-5.4" {
		t.Fatalf("resume model = %q, want gpt-5.4", runner.lastResumeModel)
	}
}

func TestResponderUsesModelSpecificReasoningDefaultsFromCache(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	cachePath := t.TempDir() + "/models_cache.json"
	if err := os.WriteFile(cachePath, []byte(`{
  "models": [
    {
      "slug": "gpt-5.4",
      "visibility": "list",
      "priority": 0,
      "default_reasoning_level": "high",
      "supported_reasoning_levels": [
        {"effort": "medium"},
        {"effort": "high"}
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	runner := &fakeRunner{}
	responder := &Responder{
		runner:                 runner,
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		modelOptions:           &ModelOptionsSource{cachePath: cachePath},
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	if _, err := responder.SetModel("conv-cache-effort", "gpt-5.4"); err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}

	answer, err := responder.AnswerWithContext(context.Background(), "conv-cache-effort", "latest question", RuntimeContext{})
	if err != nil {
		t.Fatalf("AnswerWithContext returned error: %v", err)
	}
	if answer != "new answer" {
		t.Fatalf("answer = %q, want %q", answer, "new answer")
	}
	if runner.lastNewEffort != "medium" {
		t.Fatalf("new effort = %q, want medium", runner.lastNewEffort)
	}

	state := responder.GetModelState("conv-cache-effort")
	if state.DefaultReasoningEffort != "medium" {
		t.Fatalf("default reasoning effort = %q, want medium", state.DefaultReasoningEffort)
	}
	if len(state.ReasoningOptions) != 2 || state.ReasoningOptions[1].Effort != "high" {
		t.Fatalf("unexpected reasoning options: %+v", state.ReasoningOptions)
	}
}

func TestResponderSwitchingModelClearsUnsupportedReasoningOverride(t *testing.T) {
	store, err := NewFileSessionStore(t.TempDir() + "/sessions.json")
	if err != nil {
		t.Fatalf("NewFileSessionStore returned error: %v", err)
	}

	cachePath := t.TempDir() + "/models_cache.json"
	if err := os.WriteFile(cachePath, []byte(`{
  "models": [
    {
      "slug": "gpt-5.4",
      "visibility": "list",
      "priority": 0,
      "default_reasoning_level": "medium",
      "supported_reasoning_levels": [
        {"effort": "medium"},
        {"effort": "high"}
      ]
    },
    {
      "slug": "gpt-5.4-mini",
      "visibility": "list",
      "priority": 1,
      "default_reasoning_level": "low",
      "supported_reasoning_levels": [
        {"effort": "low"},
        {"effort": "medium"}
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	workDir := t.TempDir()
	prompt := "base prompt"
	responder := &Responder{
		runner:                 &fakeRunner{},
		store:                  store,
		prompt:                 prompt,
		promptHash:             PromptHash(prompt),
		defaultWorkDir:         workDir,
		defaultModel:           "gpt-5.3-codex",
		modelOptions:           &ModelOptionsSource{cachePath: cachePath},
		defaultReasoningEffort: "medium",
		maxTurns:               30,
		sessionTTL:             time.Hour,
	}

	if _, err := responder.SetModel("conv-effort-reset", "gpt-5.4"); err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}
	if _, err := responder.SetReasoningEffort("conv-effort-reset", "high"); err != nil {
		t.Fatalf("SetReasoningEffort returned error: %v", err)
	}

	result, err := responder.SetModel("conv-effort-reset", "gpt-5.4-mini")
	if err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected SetModel to report changed")
	}
	if result.State.OverrideReasoningEffort != "" {
		t.Fatalf("reasoning override = %q, want empty", result.State.OverrideReasoningEffort)
	}
	if result.State.ReasoningEffort != "medium" {
		t.Fatalf("reasoning effort = %q, want medium", result.State.ReasoningEffort)
	}
}
