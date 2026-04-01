package config

import (
	"os"
	"testing"
)

func TestLoadLarkBotsFallsBackToLegacyEnv(t *testing.T) {
	t.Setenv("FEISHU_BOTS_JSON", "")

	cfg := &Config{
		FeishuAppID:     "cli_test",
		FeishuAppSecret: "secret",
		FeishuBotName:   "askplanner",
	}
	bots, err := loadLarkBots(cfg)
	if err != nil {
		t.Fatalf("loadLarkBots error: %v", err)
	}
	if len(bots) != 1 {
		t.Fatalf("len(bots) = %d, want 1", len(bots))
	}
	if bots[0].Key != "cli_test" || bots[0].BotName != "askplanner" {
		t.Fatalf("unexpected bot config: %+v", bots[0])
	}
}

func TestLoadLarkBotsParsesJSONAndDefaultsBotName(t *testing.T) {
	t.Setenv("FEISHU_BOTS_JSON", `[{"app_id":"cli_a","app_secret":"sa"},{"key":"bot-b","app_id":"cli_b","app_secret":"sb","bot_name":"bot-b"}]`)

	cfg := &Config{FeishuBotName: "default-bot"}
	bots, err := loadLarkBots(cfg)
	if err != nil {
		t.Fatalf("loadLarkBots error: %v", err)
	}
	if len(bots) != 2 {
		t.Fatalf("len(bots) = %d, want 2", len(bots))
	}
	if bots[0].Key != "cli_a" || bots[0].BotName != "default-bot" {
		t.Fatalf("unexpected first bot: %+v", bots[0])
	}
	if bots[1].Key != "bot-b" || bots[1].BotName != "bot-b" {
		t.Fatalf("unexpected second bot: %+v", bots[1])
	}
}

func TestLoadLarkBotsRejectsDuplicateKeys(t *testing.T) {
	t.Setenv("FEISHU_BOTS_JSON", `[{"key":"dup","app_id":"cli_a","app_secret":"sa"},{"key":"dup","app_id":"cli_b","app_secret":"sb"}]`)

	_, err := loadLarkBots(&Config{})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestLoadLarkBotsReturnsNilWhenLegacyCredentialsMissing(t *testing.T) {
	t.Setenv("FEISHU_BOTS_JSON", "")
	cfg := &Config{}
	bots, err := loadLarkBots(cfg)
	if err != nil {
		t.Fatalf("loadLarkBots error: %v", err)
	}
	if bots != nil {
		t.Fatalf("bots = %#v, want nil", bots)
	}
}

func TestSanitizeConfigKey(t *testing.T) {
	got := sanitizeConfigKey(" Bot A / Prod ")
	if got != "bot-a---prod" {
		t.Fatalf("sanitizeConfigKey = %q", got)
	}
}

func TestLoadUsesProjectRootFallbackInTempDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/prompt", []byte("prompt"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Setenv("FEISHU_BOTS_JSON", `[{"app_id":"cli_a","app_secret":"sa"}]`)
	t.Setenv("FEISHU_APP_ID", "")
	t.Setenv("FEISHU_APP_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.LarkBots) != 1 || cfg.LarkBots[0].Key != "cli_a" {
		t.Fatalf("unexpected lark bots: %+v", cfg.LarkBots)
	}
}
