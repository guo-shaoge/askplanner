package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lab/askplanner/internal/usererr"
)

type Runner struct {
	Bin                    string
	DefaultModel           string
	DefaultReasoningEffort string
	Sandbox                string
}

type RunResult struct {
	Answer    string
	SessionID string
	Stdout    string
	Stderr    string
}

func (r *Runner) RunNew(ctx context.Context, workDir, model, reasoningEffort, prompt string) (*RunResult, error) {
	args := []string{
		"exec",
		"--sandbox", r.Sandbox,
		"--json",
	}
	if resolvedModel := r.resolveModel(model); resolvedModel != "" {
		args = append(args, "--model", resolvedModel)
	}
	if resolvedReasoningEffort := r.resolveReasoningEffort(reasoningEffort); resolvedReasoningEffort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", resolvedReasoningEffort))
	}
	return r.run(ctx, workDir, args, nil, prompt, true)
}

func (r *Runner) RunResume(ctx context.Context, workDir, sessionID, model, reasoningEffort, prompt string) (*RunResult, error) {
	args := []string{
		"exec",
		"resume",
		"--json",
	}
	if resolvedModel := r.resolveModel(model); resolvedModel != "" {
		args = append(args, "--model", resolvedModel)
	}
	if resolvedReasoningEffort := r.resolveReasoningEffort(reasoningEffort); resolvedReasoningEffort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", resolvedReasoningEffort))
	}
	return r.run(ctx, workDir, args, []string{sessionID}, prompt, false)
}

func (r *Runner) resolveModel(model string) string {
	model = strings.TrimSpace(model)
	if model != "" {
		return model
	}
	return strings.TrimSpace(r.DefaultModel)
}

func (r *Runner) resolveReasoningEffort(effort string) string {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort != "" {
		return effort
	}
	return strings.TrimSpace(strings.ToLower(r.DefaultReasoningEffort))
}

func (r *Runner) run(ctx context.Context, workDir string, optionArgs, positionalArgs []string, prompt string, skipLogPrompt bool) (*RunResult, error) {
	start := time.Now()
	replyFile, err := os.CreateTemp("", "askplanner-codex-reply-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp reply file: %w", err)
	}
	replyPath := replyFile.Name()
	if err := replyFile.Close(); err != nil {
		_ = os.Remove(replyPath)
		return nil, fmt.Errorf("close temp reply file: %w", err)
	}
	defer func() {
		_ = os.Remove(replyPath)
	}()

	args := append([]string{}, optionArgs...)
	args = append(args, "-o", replyPath)
	args = append(args, positionalArgs...)
	args = append(args, "-")

	if !skipLogPrompt {
		log.Printf("[codex] running: %s %s workdir=%s prompt: %s", r.Bin, strings.Join(args, " "), workDir, compactText(prompt, 1000))
	} else {
		log.Printf("[codex] running: %s %s workdir=%s prompt: (omitted)", r.Bin, strings.Join(args, " "), workDir)
	}

	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(os.Environ(),
		"OTEL_SDK_DISABLED=true",
		"NO_COLOR=1",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	result := &RunResult{
		SessionID: extractThreadID(stdout.String()),
		Answer:    readReplyFile(replyPath),
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
	}
	if result.Answer == "" {
		result.Answer = extractFinalAnswer(stdout.String())
	}

	if runErr != nil {
		if result.Answer != "" {
			log.Printf("[codex] process exited with error but got answer (session=%s, answer_len=%d): %v",
				result.SessionID, len(result.Answer), runErr)
			return result, nil
		}
		log.Printf("[codex] error: %v\nstderr: %s", runErr, strings.TrimSpace(stderr.String()))
		return nil, classifyRunError(ctx, runErr, stderr.String(), stdout.String())
	}

	result.Answer = strings.TrimSpace(result.Answer)
	if result.Answer == "" {
		log.Printf("[codex] error: empty answer\nstderr: %s", strings.TrimSpace(stderr.String()))
		return nil, usererr.New(usererr.KindUnavailable, "Codex did not return a usable answer. Please retry.")
	}

	log.Printf("[codex] success (session=%s, answer_len=%d, elapsed=%s)", result.SessionID, len(result.Answer), time.Since(start))
	return result, nil
}

func readReplyFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func extractThreadID(stdout string) string {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "thread.started" && strings.TrimSpace(event.ThreadID) != "" {
			return strings.TrimSpace(event.ThreadID)
		}
	}
	return ""
}

func extractFinalAnswer(stdout string) string {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var envelope struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue
		}
		if envelope.Type != "event_msg" {
			continue
		}

		var payload struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Phase   string `json:"phase"`
		}
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			continue
		}
		if payload.Type == "agent_message" && payload.Phase == "final_answer" {
			return strings.TrimSpace(payload.Message)
		}
	}
	return ""
}

func classifyRunError(ctx context.Context, runErr error, stderr, stdout string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(runErr, context.DeadlineExceeded) {
		return usererr.Wrap(usererr.KindTimeout, "The request timed out while waiting for Codex. Please retry.", runErr)
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(runErr, context.Canceled) {
		return usererr.Wrap(usererr.KindUnavailable, "The request was canceled before Codex finished.", runErr)
	}

	lower := strings.ToLower(strings.Join([]string{
		runErr.Error(),
		stderr,
		stdout,
	}, "\n"))

	if errors.Is(runErr, exec.ErrNotFound) || strings.Contains(lower, "executable file not found") {
		return usererr.Wrap(usererr.KindConfig, "Codex CLI is not available on this host. Check `CODEX_BIN` and the Codex installation.", runErr)
	}
	switch {
	case containsAny(lower, "rate limit", "too many requests", "429"):
		return usererr.Wrap(usererr.KindRateLimit, "Codex is rate-limited right now. Please retry in a moment.", runErr)
	case containsAny(lower, "deadline exceeded", "timed out", "i/o timeout", "timeout"):
		return usererr.Wrap(usererr.KindTimeout, "The request timed out while waiting for Codex. Please retry.", runErr)
	case containsAny(lower, "connection refused", "connection reset", "network is unreachable", "no such host", "temporary failure in name resolution", "tls handshake timeout", "dial tcp"):
		return usererr.Wrap(usererr.KindNetwork, "Codex could not be reached because of a network problem. Please retry.", runErr)
	default:
		return usererr.Wrap(usererr.KindUnavailable, "Codex execution failed. Please retry. If it keeps failing, check the relay logs.", runErr)
	}
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func DefaultSessionStorePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".askplanner", "sessions.json")
}
