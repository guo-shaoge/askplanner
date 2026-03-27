package config

import (
	"encoding/json"
	"fmt"
	_ "io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	ProjectRoot string

	// Prompt
	PromptFile string // absolute path, default: "<ProjectRoot>/prompt"

	// Codex CLI
	CodexBin                          string
	CodexModel                        string
	CodexReasoningEffort              string
	CodexSandbox                      string
	CodexSessionStore                 string // absolute path
	CodexMaxTurns                     int
	CodexSessionTTLHours              int
	CodexTimeoutSec                   int
	WorkspaceRoot                     string
	WorkspaceIdleTTLHours             int
	WorkspaceGCIntervalMin            int
	AgentRulesSyncIntervalMin         int
	WorkspaceRepoTidbURL              string
	WorkspaceRepoTidbDefaultRef       string
	WorkspaceRepoAgentRulesURL        string
	WorkspaceRepoAgentRulesDefaultRef string
	WorkspaceRepoTidbDocsURL          string
	WorkspaceRepoTidbDocsDefaultRef   string

	// Clinic
	ClinicEnableAutoSlowQuery bool
	ClinicAPIKey              string
	ClinicHTTPTimeoutSec      int
	ClinicStoreDir            string
	ClinicStoreMaxItems       int

	// Logging
	LogFile string // absolute path

	// Usage dashboard
	UsageHTTPAddr      string
	UsageLogTailBytes  int
	UsageQuestionsPath string

	// Lark (larkbot only)
	LarkBots                []LarkBotConfig
	FeishuAppID             string
	FeishuAppSecret         string
	FeishuBotName           string
	FeishuDedupTimeoutInMin int
	FeishuFileDir           string // absolute path
	FeishuUserFileMaxItems  int
}

type LarkBotConfig struct {
	Key       string `json:"key"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	BotName   string `json:"bot_name,omitempty"`
}

func Load() (*Config, error) {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("detect project root: %w", err)
	}

	workspaceRoot := resolvePath(projectRoot, envOrDefault("WORKSPACE_ROOT", ".askplanner/workspaces"))
	clinicStoreDir := envOrDefault("CLINIC_STORE_DIR", filepath.Join(workspaceRoot, "clinic"))
	feishuFileDir := envOrDefault("FEISHU_FILE_DIR", filepath.Join(workspaceRoot, "uploads"))

	cfg := &Config{
		ProjectRoot:                       projectRoot,
		PromptFile:                        resolvePath(projectRoot, envOrDefault("PROMPT_FILE", "prompt")),
		CodexBin:                          envOrDefault("CODEX_BIN", "codex"),
		CodexModel:                        envOrDefault("CODEX_MODEL", "gpt-5.3-codex"),
		CodexReasoningEffort:              envOrDefault("CODEX_REASONING_EFFORT", "medium"),
		CodexSandbox:                      envOrDefault("CODEX_SANDBOX", "read-only"),
		CodexSessionStore:                 resolvePath(projectRoot, envOrDefault("CODEX_SESSION_STORE", ".askplanner/sessions.json")),
		CodexMaxTurns:                     envAsInt("CODEX_MAX_TURNS", 30),
		CodexSessionTTLHours:              envAsInt("CODEX_SESSION_TTL_HOURS", 24),
		CodexTimeoutSec:                   envAsInt("CODEX_TIMEOUT_SEC", 120),
		WorkspaceRoot:                     workspaceRoot,
		WorkspaceIdleTTLHours:             envAsInt("WORKSPACE_IDLE_TTL_HOURS", 72),
		WorkspaceGCIntervalMin:            envAsInt("WORKSPACE_GC_INTERVAL_MIN", 15),
		AgentRulesSyncIntervalMin:         envAsInt("AGENT_RULES_SYNC_INTERVAL_MIN", 10),
		WorkspaceRepoTidbURL:              envOrDefault("WORKSPACE_REPO_TIDB_URL", "https://gh-proxy.org/https://github.com/pingcap/tidb.git"),
		WorkspaceRepoTidbDefaultRef:       envOrDefault("WORKSPACE_REPO_TIDB_DEFAULT_REF", "master"),
		WorkspaceRepoAgentRulesURL:        envOrDefault("WORKSPACE_REPO_AGENT_RULES_URL", "https://gh-proxy.org/https://github.com/pingcap/agent-rules.git"),
		WorkspaceRepoAgentRulesDefaultRef: envOrDefault("WORKSPACE_REPO_AGENT_RULES_DEFAULT_REF", "main"),
		WorkspaceRepoTidbDocsURL:          envOrDefault("WORKSPACE_REPO_TIDB_DOCS_URL", "https://github.com/pingcap/docs.git"),
		WorkspaceRepoTidbDocsDefaultRef:   envOrDefault("WORKSPACE_REPO_TIDB_DOCS_DEFAULT_REF", "master"),
		ClinicEnableAutoSlowQuery:         envAsBool("CLINIC_ENABLE_AUTO_SLOWQUERY", false),
		ClinicAPIKey:                      strings.TrimSpace(os.Getenv("CLINIC_API_KEY")),
		ClinicHTTPTimeoutSec:              envAsInt("CLINIC_HTTP_TIMEOUT_SEC", 15),
		ClinicStoreDir:                    resolvePath(projectRoot, clinicStoreDir),
		ClinicStoreMaxItems:               envAsInt("CLINIC_STORE_MAX_ITEMS", 50),
		LogFile:                           resolvePath(projectRoot, envOrDefault("LOG_FILE", ".askplanner/askplanner.log")),
		UsageHTTPAddr:                     strings.TrimSpace(envOrDefault("USAGE_HTTP_ADDR", "127.0.0.1:18080")),
		UsageLogTailBytes:                 envAsInt("USAGE_LOG_TAIL_BYTES", 4*1024*1024),
		UsageQuestionsPath:                resolvePath(projectRoot, envOrDefault("USAGE_QUESTIONS_PATH", ".askplanner/usage_questions.jsonl")),
		FeishuAppID:                       os.Getenv("FEISHU_APP_ID"),
		FeishuAppSecret:                   os.Getenv("FEISHU_APP_SECRET"),
		FeishuBotName:                     strings.ToLower(strings.TrimSpace(envOrDefault("FEISHU_BOT_NAME", "askplanner"))),
		FeishuDedupTimeoutInMin:           envAsInt("FEISHU_DEDUP_MESSAGE_TIMEOUT_IN_MIN", 3600),
		FeishuFileDir:                     resolvePath(projectRoot, feishuFileDir),
		FeishuUserFileMaxItems:            envAsInt("FEISHU_USER_FILE_MAX_ITEMS", 100),
	}

	larkBots, err := loadLarkBots(cfg)
	if err != nil {
		return nil, err
	}
	cfg.LarkBots = larkBots
	return cfg, nil
}

func detectProjectRoot() (string, error) {
	if v := os.Getenv("PROJECT_ROOT"); v != "" {
		return filepath.Abs(v)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "prompt")); err == nil {
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

func SetupLogging(logFile string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	// log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return f, nil
}

func loadLarkBots(cfg *Config) ([]LarkBotConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	raw := strings.TrimSpace(os.Getenv("FEISHU_BOTS_JSON"))
	if raw == "" {
		bot := LarkBotConfig{
			Key:       sanitizeConfigKey(cfg.FeishuAppID),
			AppID:     strings.TrimSpace(cfg.FeishuAppID),
			AppSecret: strings.TrimSpace(cfg.FeishuAppSecret),
			BotName:   strings.TrimSpace(cfg.FeishuBotName),
		}
		if bot.AppID == "" || bot.AppSecret == "" {
			return nil, nil
		}
		return []LarkBotConfig{bot}, nil
	}

	var bots []LarkBotConfig
	if err := json.Unmarshal([]byte(raw), &bots); err != nil {
		return nil, fmt.Errorf("parse FEISHU_BOTS_JSON: %w", err)
	}
	if len(bots) == 0 {
		return nil, fmt.Errorf("FEISHU_BOTS_JSON is empty")
	}

	seen := make(map[string]struct{}, len(bots))
	for i := range bots {
		bot := &bots[i]
		bot.AppID = strings.TrimSpace(bot.AppID)
		bot.AppSecret = strings.TrimSpace(bot.AppSecret)
		bot.BotName = strings.TrimSpace(bot.BotName)
		if bot.BotName == "" {
			bot.BotName = strings.TrimSpace(cfg.FeishuBotName)
		}
		bot.Key = sanitizeConfigKey(bot.Key)
		if bot.Key == "" {
			bot.Key = sanitizeConfigKey(bot.AppID)
		}
		if bot.Key == "" {
			return nil, fmt.Errorf("FEISHU_BOTS_JSON[%d] key is empty", i)
		}
		if bot.AppID == "" || bot.AppSecret == "" {
			return nil, fmt.Errorf("FEISHU_BOTS_JSON[%d] requires app_id and app_secret", i)
		}
		if _, ok := seen[bot.Key]; ok {
			return nil, fmt.Errorf("FEISHU_BOTS_JSON contains duplicate key: %s", bot.Key)
		}
		seen[bot.Key] = struct{}{}
	}
	return bots, nil
}

func sanitizeConfigKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "._-")
}

func resolvePath(projectRoot, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectRoot, path)
}
