package usage

import (
	"context"
	"testing"
	"time"
)

type fakeContactGetter struct {
	calls []fakeLookupCall
	byKey map[string]string
}

type fakeLookupCall struct {
	userID string
	idType string
}

func (f *fakeContactGetter) LookupUserName(_ context.Context, rawID, idType string) string {
	f.calls = append(f.calls, fakeLookupCall{userID: rawID, idType: idType})
	return f.byKey[rawID+"\x00"+idType]
}

func TestParseUsageUserIdentity(t *testing.T) {
	tests := []struct {
		name            string
		userKey         string
		conversationKey string
		wantBotKey      string
		wantRawID       string
	}{
		{
			name:       "scoped user key",
			userKey:    "larkbot:bot-a:ou_123",
			wantBotKey: "bot-a",
			wantRawID:  "ou_123",
		},
		{
			name:            "raw legacy user key with legacy conversation",
			userKey:         "ou_legacy",
			conversationKey: "lark:chat:oc_1:user:ou_legacy",
			wantRawID:       "ou_legacy",
		},
		{
			name:            "raw key with scoped conversation",
			userKey:         "ou_456",
			conversationKey: "larkbot:bot-b:root:om_1:user:larkbot_bot-b_ou_456",
			wantBotKey:      "bot-b",
			wantRawID:       "ou_456",
		},
		{
			name:            "scoped conversation only",
			conversationKey: "larkbot:bot-c:root:om_1:user:larkbot_bot-c_ou_789",
			wantBotKey:      "bot-c",
			wantRawID:       "ou_789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUsageUserIdentity(tt.userKey, tt.conversationKey)
			if got.botKey != tt.wantBotKey || got.rawID != tt.wantRawID {
				t.Fatalf("identity = %+v, want bot=%q raw=%q", got, tt.wantBotKey, tt.wantRawID)
			}
		})
	}
}

func TestFeishuUserNameResolverResolveAndCache(t *testing.T) {
	fakeA := &fakeContactGetter{
		byKey: map[string]string{
			"ou_123\x00open_id": "Alice",
			"u_123\x00user_id":  "Bob",
		},
	}
	fakeB := &fakeContactGetter{
		byKey: map[string]string{
			"ou_999\x00open_id": "Carol",
		},
	}

	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	resolver := &feishuUserNameResolver{
		bots: []usageResolverBot{
			{key: "bot-a", client: fakeA},
			{key: "bot-b", client: fakeB},
		},
		now:   func() time.Time { return now },
		cache: make(map[string]cachedUsageUserName),
	}

	if got := resolver.Resolve(context.Background(), sourceCLI, cliVirtualUserKey, ""); got != usageCLIUserName {
		t.Fatalf("CLI user name = %q, want %q", got, usageCLIUserName)
	}

	if got := resolver.Resolve(context.Background(), sourceLark, "larkbot:bot-a:ou_123", ""); got != "Alice" {
		t.Fatalf("resolved scoped name = %q, want Alice", got)
	}
	if len(fakeA.calls) != 1 || fakeA.calls[0].idType != "open_id" {
		t.Fatalf("open_id lookup calls = %+v, want one open_id call", fakeA.calls)
	}

	if got := resolver.Resolve(context.Background(), sourceLark, "larkbot:bot-a:u_123", ""); got != "Bob" {
		t.Fatalf("resolved fallback name = %q, want Bob", got)
	}
	if len(fakeA.calls) != 3 {
		t.Fatalf("fallback lookup calls = %+v, want 3 total calls", fakeA.calls)
	}

	if got := resolver.Resolve(context.Background(), sourceLark, "ou_999", "lark:chat:oc_1:user:ou_999"); got != "Carol" {
		t.Fatalf("resolved wildcard name = %q, want Carol", got)
	}
	if len(fakeB.calls) != 1 {
		t.Fatalf("bot-b calls = %+v, want one call", fakeB.calls)
	}

	beforeCacheCalls := len(fakeA.calls)
	if got := resolver.Resolve(context.Background(), sourceLark, "larkbot:bot-a:ou_123", ""); got != "Alice" {
		t.Fatalf("cached name = %q, want Alice", got)
	}
	if len(fakeA.calls) != beforeCacheCalls {
		t.Fatalf("cache miss: calls = %+v", fakeA.calls)
	}
}
