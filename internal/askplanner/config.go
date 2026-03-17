package askplanner

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
	MaxToolSteps   int
	MaxResultChars int
	StepDelayMS    int

	// Paths (absolute)
	ProjectRoot string
	SkillsDir   string
	TiDBCodeDir string
}

func Load() (*Config, error) {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("detect project root: %w", err)
	}

	apiKey := strings.TrimSpace(os.Getenv("KIMI_API_KEY"))
	if apiKey == "" {
		keyFile := filepath.Join(projectRoot, "keys", "kimi_free")
		data, err := os.ReadFile(keyFile)
		if err == nil {
			apiKey = strings.TrimSpace(string(data))
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("KIMI_API_KEY not set and keys/kimi_free not found")
	}

	return &Config{
		LLMProvider:    envOrDefault("LLM_PROVIDER", "kimi"),
		KimiAPIKey:     apiKey,
		KimiBaseURL:    envOrDefault("KIMI_BASE_URL", "https://api.moonshot.cn"),
		KimiModel:      envOrDefault("KIMI_MODEL", "moonshot-v1-8k"),
		Temperature:    envAsFloat("TEMPERATURE", 0.2),
		MaxToolSteps:   envAsInt("MAX_TOOL_STEPS", 50),
		MaxResultChars: envAsInt("MAX_RESULT_CHARS", 12000),
		StepDelayMS:    envAsInt("STEP_DELAY_MS", 1000),
		ProjectRoot:    projectRoot,
		SkillsDir:      filepath.Join(projectRoot, envOrDefault("SKILLS_DIR", "contrib/agent-rules/skills/tidb-query-tuning/references")),
		TiDBCodeDir:    filepath.Join(projectRoot, envOrDefault("TIDB_CODE_DIR", "contrib/tidb")),
	}, nil
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
