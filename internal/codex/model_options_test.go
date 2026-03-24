package codex

import "testing"

func TestParseModelOptionsFromCache(t *testing.T) {
	data := []byte(`{
  "models": [
    {
      "slug": "gpt-5.4",
      "description": "Latest frontier agentic coding model.",
      "visibility": "list",
      "priority": 0,
      "default_reasoning_level": "high",
      "supported_reasoning_levels": [
        {"effort": "medium", "description": "Balanced."},
        {"effort": "high", "description": "Deeper reasoning."}
      ]
    },
    {
      "slug": "gpt-5.3-codex",
      "description": "Frontier Codex-optimized agentic coding model.",
      "visibility": "list",
      "priority": 2,
      "default_reasoning_level": "medium",
      "supported_reasoning_levels": [
        {"effort": "low"},
        {"effort": "medium"},
        {"effort": "high"}
      ]
    },
    {"slug":"hidden-model","description":"skip me","visibility":"hidden","priority":1}
  ]
}`)

	options, err := parseModelOptionsFromCache(data)
	if err != nil {
		t.Fatalf("parseModelOptionsFromCache returned error: %v", err)
	}
	if len(options) != 2 {
		t.Fatalf("option count = %d, want 2", len(options))
	}
	if options[0].Slug != "gpt-5.4" || options[1].Slug != "gpt-5.3-codex" {
		t.Fatalf("unexpected options: %+v", options)
	}
	if options[0].DefaultReasoningEffort != "high" {
		t.Fatalf("default reasoning effort = %q, want high", options[0].DefaultReasoningEffort)
	}
	if len(options[0].SupportedReasoningEfforts) != 2 {
		t.Fatalf("supported reasoning effort count = %d, want 2", len(options[0].SupportedReasoningEfforts))
	}
	if options[0].SupportedReasoningEfforts[0].Effort != "medium" || options[0].SupportedReasoningEfforts[1].Effort != "high" {
		t.Fatalf("unexpected supported reasoning efforts: %+v", options[0].SupportedReasoningEfforts)
	}
}

func TestModelOptionsSourceFallsBackWithoutCache(t *testing.T) {
	source := &ModelOptionsSource{
		fallback:  newFallbackModelOptions([]string{"gpt-5.3-codex", "gpt-5.4"}),
		cachePath: t.TempDir() + "/missing.json",
	}

	options := source.List()
	if len(options) != 2 {
		t.Fatalf("option count = %d, want 2", len(options))
	}
	if options[0].Slug != "gpt-5.3-codex" || options[1].Slug != "gpt-5.4" {
		t.Fatalf("unexpected options: %+v", options)
	}
}
