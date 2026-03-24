package codex

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ModelOptionsSource struct {
	fallback  []ModelOption
	cachePath string
}

func NewModelOptionsSource(fallback []string) *ModelOptionsSource {
	return &ModelOptionsSource{
		fallback:  newFallbackModelOptions(fallback),
		cachePath: defaultModelsCachePath(),
	}
}

func (s *ModelOptionsSource) List() []ModelOption {
	if s == nil {
		return nil
	}
	if options := s.loadFromCache(); len(options) > 0 {
		return options
	}
	return cloneModelOptions(s.fallback)
}

func (s *ModelOptionsSource) loadFromCache() []ModelOption {
	if s == nil || strings.TrimSpace(s.cachePath) == "" {
		return nil
	}
	options, err := loadModelOptionsFromCacheFile(s.cachePath)
	if err != nil {
		log.Printf("[codex] load model options from cache failed path=%s: %v", s.cachePath, err)
		return nil
	}
	return options
}

func loadModelOptionsFromCacheFile(path string) ([]ModelOption, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseModelOptionsFromCache(data)
}

func parseModelOptionsFromCache(data []byte) ([]ModelOption, error) {
	var cache struct {
		Models []struct {
			Slug                     string `json:"slug"`
			Description              string `json:"description"`
			Visibility               string `json:"visibility"`
			Priority                 int    `json:"priority"`
			SupportedReasoningLevels []struct {
				Effort      string `json:"effort"`
				Description string `json:"description"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	type entry struct {
		option   ModelOption
		priority int
		index    int
	}
	entries := make([]entry, 0, len(cache.Models))
	seen := make(map[string]struct{}, len(cache.Models))
	for idx, model := range cache.Models {
		slug := strings.TrimSpace(model.Slug)
		if slug == "" {
			continue
		}
		if visibility := strings.TrimSpace(model.Visibility); visibility != "" && visibility != "list" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		entries = append(entries, entry{
			option: ModelOption{
				Slug:                      slug,
				Description:               strings.TrimSpace(model.Description),
				SupportedReasoningEfforts: newReasoningEffortOptions(model.SupportedReasoningLevels),
			},
			priority: model.Priority,
			index:    idx,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].priority != entries[j].priority {
			return entries[i].priority < entries[j].priority
		}
		return entries[i].index < entries[j].index
	})

	options := make([]ModelOption, 0, len(entries))
	for _, entry := range entries {
		options = append(options, entry.option)
	}
	return options, nil
}

func newReasoningEffortOptions(items []struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}) []ReasoningEffortOption {
	options := make([]ReasoningEffortOption, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		effort := strings.TrimSpace(strings.ToLower(item.Effort))
		if effort == "" {
			continue
		}
		if _, ok := seen[effort]; ok {
			continue
		}
		seen[effort] = struct{}{}
		options = append(options, ReasoningEffortOption{
			Effort:      effort,
			Description: strings.TrimSpace(item.Description),
		})
	}
	return options
}

func newFallbackModelOptions(options []string) []ModelOption {
	out := make([]ModelOption, 0, len(options))
	seen := make(map[string]struct{}, len(options))
	for _, option := range options {
		slug := strings.TrimSpace(option)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, ModelOption{Slug: slug})
	}
	return out
}

func cloneModelOptions(options []ModelOption) []ModelOption {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]ModelOption, len(options))
	for i, option := range options {
		cloned[i] = option
		cloned[i].SupportedReasoningEfforts = cloneReasoningEffortOptions(option.SupportedReasoningEfforts)
	}
	return cloned
}

func defaultModelsCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".codex", "models_cache.json")
}
