package tools

import (
	"context"
	"fmt"

	"lab/askplanner/internal/askplanner/llmprovider"
)

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Registry holds registered tools and dispatches calls.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry creates a registry with the given tools.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		r.tools[t.Name()] = t
		r.order = append(r.order, t.Name())
	}
	return r
}

// Definitions returns tool definitions for the LLM API.
func (r *Registry) Definitions() []llmprovider.ToolDefinition {
	defs := make([]llmprovider.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, llmprovider.ToolDefinition{
			Type: "function",
			Function: llmprovider.ToolDefFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// Execute runs a tool by name with the given JSON arguments.
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}
