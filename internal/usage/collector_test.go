package usage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubUsageUserResolver map[string]string

func (s stubUsageUserResolver) Resolve(_ context.Context, source, userKey, conversationKey string) string {
	return s[userLookupKey(source, userKey, conversationKey)]
}

func TestCollectorSnapshotAndPages(t *testing.T) {
	root := t.TempDir()
	sessionStore := filepath.Join(root, "sessions.json")
	logPath := filepath.Join(root, "askplanner.log")
	workspaceRoot := filepath.Join(root, "workspaces")
	questionPath := filepath.Join(root, "usage_questions.jsonl")

	sessionData := `{
  "cli:default": {
    "conversation_key": "cli:default",
    "user_key": "cli_default",
    "session_id": "session-cli",
    "work_dir": "/tmp/cli",
    "created_at": "2026-03-24T09:00:00Z",
    "last_active_at": "2026-03-24T09:55:00Z",
    "turn_count": 3,
    "model_override": "gpt-5.3-codex",
    "turns": [{"user":"show status","assistant":"ok","at":"2026-03-24T09:55:00Z"}]
  },
  "lark:chat:abc:user:u1": {
    "conversation_key": "lark:chat:abc:user:u1",
    "user_key": "u1",
    "session_id": "session-lark",
    "work_dir": "/tmp/lark",
    "created_at": "2026-03-24T08:30:00Z",
    "last_active_at": "2026-03-24T09:58:00Z",
    "turn_count": 7,
    "last_error": "resume failed once",
    "turns": [{"user":"optimize this sql","assistant":"ok","at":"2026-03-24T09:58:00Z"}]
  }
}`
	if err := os.WriteFile(sessionStore, []byte(sessionData), 0o644); err != nil {
		t.Fatalf("write session store: %v", err)
	}

	metaPath := filepath.Join(workspaceRoot, "users", "u1", "data")
	if err := os.MkdirAll(metaPath, 0o755); err != nil {
		t.Fatalf("mkdir workspace metadata: %v", err)
	}
	workspaceData := `{
  "user_key": "u1",
  "last_active_at": "2026-03-24T09:57:00Z",
  "repos": {
    "tidb": {
      "name": "tidb",
      "relative_path": "contrib/tidb",
      "requested_ref": "release-8.5"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(metaPath, "workspace.json"), []byte(workspaceData), 0o644); err != nil {
		t.Fatalf("write workspace metadata: %v", err)
	}

	logData := "" +
		"2026/03/24 17:58:10 main.go:172: [askplanner] request done conversation=cli:default elapsed=850ms\n" +
		"2026/03/24 17:58:20 handler.go:47: [larkbot] handle event done message_id=m1 conversation=lark:chat:abc:user:u1 elapsed=1.7s\n" +
		"2026/03/24 17:58:30 handler.go:133: [larkbot] handle event error: upstream timeout (message_id=m2)\n"
	if err := os.WriteFile(logPath, []byte(logData), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	store := &QuestionStore{path: questionPath}
	if err := store.Append(QuestionEvent{
		EventID:          "live-1",
		AskedAt:          time.Date(2026, 3, 24, 9, 59, 0, 0, time.UTC),
		Source:           sourceLark,
		UserKey:          "u1",
		ConversationKey:  "lark:chat:abc:user:u1",
		Question:         "explain index merge",
		Status:           statusSuccess,
		DurationMs:       1900,
		Model:            "gpt-5.3-codex",
		WorkspaceEnvHash: "env1",
	}); err != nil {
		t.Fatalf("append first question event: %v", err)
	}
	if err := store.Append(QuestionEvent{
		EventID:          "live-2",
		AskedAt:          time.Date(2026, 3, 24, 9, 58, 0, 0, time.UTC),
		Source:           sourceCLI,
		UserKey:          cliVirtualUserKey,
		ConversationKey:  "cli:default",
		Question:         "show status",
		Status:           statusSuccess,
		DurationMs:       850,
		Model:            "gpt-5.3-codex",
		WorkspaceEnvHash: "env-cli",
	}); err != nil {
		t.Fatalf("append second question event: %v", err)
	}

	now := time.Date(2026, 3, 24, 18, 0, 0, 0, time.Local)
	collector := &Collector{
		sessionStorePath: sessionStore,
		logPath:          logPath,
		workspaceRoot:    workspaceRoot,
		sessionTTL:       24 * time.Hour,
		logTailBytes:     1 << 20,
		questionStore:    store,
		userResolver: stubUsageUserResolver{
			userLookupKey(sourceCLI, cliVirtualUserKey, ""):          usageCLIUserName,
			userLookupKey(sourceLark, "u1", "lark:chat:abc:user:u1"): "Alice Zhang",
		},
		now: func() time.Time { return now },
	}

	snapshot, err := collector.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.Summary.TotalConversations != 2 {
		t.Fatalf("total conversations = %d, want 2", snapshot.Summary.TotalConversations)
	}
	if snapshot.Summary.TotalUsers != 2 {
		t.Fatalf("total users = %d, want 2", snapshot.Summary.TotalUsers)
	}
	if snapshot.Summary.TotalQuestions != 2 {
		t.Fatalf("total questions = %d, want 2", snapshot.Summary.TotalQuestions)
	}
	if len(snapshot.TopUsers) == 0 || snapshot.TopUsers[0].UserKey != "u1" {
		t.Fatalf("top users = %+v, want u1 first", snapshot.TopUsers)
	}
	if len(snapshot.TopUsers) == 0 || snapshot.TopUsers[0].UserName != "Alice Zhang" {
		t.Fatalf("top user name = %+v, want Alice Zhang", snapshot.TopUsers)
	}
	if len(snapshot.RecentSessions) == 0 || snapshot.RecentSessions[0].UserName != "Alice Zhang" {
		t.Fatalf("recent sessions = %+v, want enriched lark name", snapshot.RecentSessions)
	}
	if snapshot.RequestStats.Requests1Hour != 2 {
		t.Fatalf("requests 1h = %d, want 2", snapshot.RequestStats.Requests1Hour)
	}
	if snapshot.RequestStats.Errors1Hour != 1 {
		t.Fatalf("errors 1h = %d, want 1", snapshot.RequestStats.Errors1Hour)
	}
	if len(snapshot.QuestionStatusBreakdown) == 0 || snapshot.QuestionStatusBreakdown[0].Name != statusSuccess {
		t.Fatalf("question status breakdown = %+v, want success", snapshot.QuestionStatusBreakdown)
	}

	page, err := collector.QuestionsPage(QuestionQuery{Page: 1, PageSize: 2, UserKey: "u1"})
	if err != nil {
		t.Fatalf("questions page: %v", err)
	}
	if page.TotalItems != 1 {
		t.Fatalf("questions total items = %d, want 1", page.TotalItems)
	}
	if len(page.Items) != 1 {
		t.Fatalf("questions items = %d, want 1", len(page.Items))
	}
	if page.Items[0].Question != "explain index merge" {
		t.Fatalf("first question = %q, want newest", page.Items[0].Question)
	}
	if page.Items[0].UserName != "Alice Zhang" {
		t.Fatalf("first question user name = %q, want Alice Zhang", page.Items[0].UserName)
	}

	pageByName, err := collector.QuestionsPage(QuestionQuery{Page: 1, PageSize: 10, Query: "alice"})
	if err != nil {
		t.Fatalf("questions page by name: %v", err)
	}
	if pageByName.TotalItems != 1 {
		t.Fatalf("questions by name total items = %d, want 1", pageByName.TotalItems)
	}

	users, err := collector.UsersPage(UserQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("users page: %v", err)
	}
	if users.TotalItems != 2 {
		t.Fatalf("users total items = %d, want 2", users.TotalItems)
	}
	if users.Items[0].UserKey != "u1" || users.Items[0].QuestionCount != 1 {
		t.Fatalf("first user = %+v, want u1 count 1", users.Items[0])
	}
	if users.Items[0].UserName != "Alice Zhang" {
		t.Fatalf("first user name = %q, want Alice Zhang", users.Items[0].UserName)
	}
}

func TestSourceForConversationRecognizesLarkbotPrefix(t *testing.T) {
	if got := sourceForConversation("larkbot:bot-a:root:om_1:user:larkbot_bot-a_ou_123"); got != sourceLark {
		t.Fatalf("sourceForConversation returned %q, want %q", got, sourceLark)
	}
}
