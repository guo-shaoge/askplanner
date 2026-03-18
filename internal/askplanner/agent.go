package askplanner

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"lab/askplanner/internal/askplanner/llmprovider"
	"lab/askplanner/internal/askplanner/tools"
)

// Agent orchestrates the LLM + tools loop to answer questions.
type Agent struct {
	provider       llmprovider.Provider
	toolReg        *tools.Registry
	systemPrompt   string
	temperature    float64
	maxSteps       int
	maxResultChars int
	stepDelay      time.Duration
}

// AgentConfig holds the agent configuration.
type AgentConfig struct {
	Provider       llmprovider.Provider
	ToolRegistry   *tools.Registry
	SkillIndex     *tools.Index
	Temperature    float64
	MaxToolSteps   int
	MaxResultChars int
	StepDelay      time.Duration
}

// New creates an agent with the given configuration.
func New(cfg AgentConfig) *Agent {
	return &Agent{
		provider:       cfg.Provider,
		toolReg:        cfg.ToolRegistry,
		systemPrompt:   buildSystemPrompt(cfg.SkillIndex),
		temperature:    cfg.Temperature,
		maxSteps:       cfg.MaxToolSteps,
		maxResultChars: cfg.MaxResultChars,
		stepDelay:      cfg.StepDelay,
	}
}

// Answer processes a user question through the tool loop and returns the final answer.
// onToolCall is invoked for each tool call so the caller can display progress.
func (a *Agent) Answer(ctx context.Context, question string, onToolCall func(toolName, args string)) (string, error) {
	messages := []llmprovider.Message{
		{Role: "system", Content: a.systemPrompt},
		{Role: "user", Content: question},
	}

	toolDefs := a.toolReg.Definitions()

	for step := 0; step < a.maxSteps; step++ {
		resp, err := a.provider.Complete(ctx, llmprovider.CompletionRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: a.temperature,
		})
		if err != nil {
			return "", fmt.Errorf("LLM call failed (step %d): %w", step+1, err)
		}

		log.Printf("[agent] step=%d finish_reason=%s tool_calls=%d tokens=%d",
			step+1, resp.FinishReason, len(resp.Message.ToolCalls), resp.Usage.TotalTokens)

		// No tool calls — we have the final answer
		if len(resp.Message.ToolCalls) == 0 {
			answer := strings.TrimSpace(resp.Message.Content)
			if answer == "" {
				return "", fmt.Errorf("empty answer from LLM")
			}
			return answer, nil
		}

		// Append assistant message with tool calls
		messages = append(messages, resp.Message)

		// Execute each tool call
		for _, tc := range resp.Message.ToolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}

			log.Printf("[agent] tool_call: %s(%s)", tc.Function.Name, truncate(tc.Function.Arguments, 200))

			result, err := a.toolReg.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				log.Printf("[agent] tool_error: %s: %v", tc.Function.Name, err)
				result = "TOOL_ERROR: " + err.Error()
			} else {
				result = truncate(result, a.maxResultChars)
			}

			messages = append(messages, llmprovider.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}

		// Delay between steps to avoid rate limiting
		if a.stepDelay > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(a.stepDelay):
			}
		}
	}

	return "", fmt.Errorf("exceeded max tool steps (%d)", a.maxSteps)
}

func buildSystemPrompt(idx *tools.Index) string {
	var sb strings.Builder

	sb.WriteString(`You are a TiDB Query Tuning Agent. You help engineers diagnose and optimize slow TiDB queries.

## Rules
1. Always check statistics health first — most bad plans come from stale stats.
2. Use EXPLAIN ANALYZE as ground truth, not just EXPLAIN.
3. Do NOT invent TiDB syntax, features, or configuration that do not exist.
4. Use the available tools to read skill references and TiDB source code before answering.
5. Search the oncall experiences and customer issues for matching precedents.
6. Provide actionable recommendations: specific SQL, hints, session variables, or index changes.
7. If information is insufficient, list what's missing and suggest the smallest diagnostic steps.
8. Default language is English.

## TiDB Planner Code Layout
Available at contrib/tidb/pkg/planner/ — use read_file, search_code, and list_dir tools to explore:
- optimize.go: main optimization entry point
- core/: logical/physical plans, optimization rules, join ordering, cost model
- cascades/: cascades optimizer framework
- cardinality/: cardinality estimation
- property/: plan properties
- indexadvisor/: index advisor
- util/fixcontrol/: fix control variables
Related: pkg/statistics/ (stats), pkg/expression/ (expressions), pkg/parser/ (SQL parser)

## Skill Workflow & References
`)

	sb.WriteString(idx.SystemPromptSection())

	return sb.String()
}

func truncate(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "\n...(truncated)"
}
