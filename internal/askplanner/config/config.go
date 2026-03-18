package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	// LLM
	LLMProvider string
	KimiAPIKey  string
	KimiBaseURL string
	KimiModel   string
	Temperature float64

	// Agent
	AgentDebug     bool
	MaxToolSteps   int
	MaxResultChars int
	StepDelayMS    int

	// Paths (absolute)
	ProjectRoot    string
	SkillsDir      string
	TiDBCodeDir    string
	TiDBDocsDir    string
	DocsOverlayDir string
}

func Load() (*Config, error) {
	return load(true)
}

func LoadPromptOnly() (*Config, error) {
	return load(false)
}

func load(requireAPIKey bool) (*Config, error) {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("detect project root: %w", err)
	}

	apiKey := loadAPIKey(projectRoot)
	if requireAPIKey && apiKey == "" {
		return nil, fmt.Errorf("KIMI_API_KEY not set and keys/kimi_free not found")
	}

	return &Config{
		LLMProvider:    envOrDefault("LLM_PROVIDER", "kimi"),
		KimiAPIKey:     apiKey,
		KimiBaseURL:    envOrDefault("KIMI_BASE_URL", "https://api.moonshot.cn"),
		KimiModel:      envOrDefault("KIMI_MODEL", "moonshot-v1-8k"),
		Temperature:    envAsFloat("TEMPERATURE", 0.2),
		AgentDebug:     envAsBool("AGENT_DEBUG", false),
		MaxToolSteps:   envAsInt("MAX_TOOL_STEPS", 50),
		MaxResultChars: envAsInt("MAX_RESULT_CHARS", 12000),
		StepDelayMS:    envAsInt("STEP_DELAY_MS", 1000),
		ProjectRoot:    projectRoot,
		SkillsDir:      filepath.Join(projectRoot, envOrDefault("SKILLS_DIR", "contrib/agent-rules/skills/tidb-query-tuning/references")),
		TiDBCodeDir:    filepath.Join(projectRoot, envOrDefault("TIDB_CODE_DIR", "contrib/tidb")),
		TiDBDocsDir:    filepath.Join(projectRoot, envOrDefault("TIDB_DOCS_DIR", "contrib/tidb-docs")),
		DocsOverlayDir: filepath.Join(projectRoot, envOrDefault("DOCS_OVERLAY_DIR", "prompts/tidb-query-tuning-official-docs")),
	}, nil
}

func loadAPIKey(projectRoot string) string {
	apiKey := strings.TrimSpace(os.Getenv("KIMI_API_KEY"))
	if apiKey != "" {
		return apiKey
	}

	keyFile := filepath.Join(projectRoot, "keys", "kimi_free")
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func detectProjectRoot() (string, error) {
	if v := os.Getenv("PROJECT_ROOT"); v != "" {
		return filepath.Abs(v)
	}
	// Walk up from cwd looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return os.Getwd()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envAsInt(key string, defaultVal int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

func envAsFloat(key string, defaultVal float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func envAsBool(key string, defaultVal bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return defaultVal
	}

	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}
