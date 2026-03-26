package larkbot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"lab/askplanner/internal/usererr"
)

func TestWithTypingReactionAddsAndDeletesReaction(t *testing.T) {
	var (
		createCalls int
		deleteCalls int
		emojiType   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_message/reactions":
			createCalls++

			var payload struct {
				ReactionType struct {
					EmojiType string `json:"emoji_type"`
				} `json:"reaction_type"`
			}
			if err := readJSONBody(r, &payload); err != nil {
				t.Fatalf("read create request body: %v", err)
			}
			emojiType = payload.ReactionType.EmojiType

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"reaction_id":"reaction-1","reaction_type":{"emoji_type":"Typing"}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/open-apis/im/v1/messages/om_message/reactions/reaction-1":
			deleteCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"reaction_id":"reaction-1"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := lark.NewClient("cli_a", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	callbackRun := false
	err := withTypingReaction(context.Background(), apiClient, "om_message", func() error {
		callbackRun = true
		if createCalls != 1 {
			t.Fatalf("expected create before callback, got %d", createCalls)
		}
		if deleteCalls != 0 {
			t.Fatalf("expected delete after callback, got %d", deleteCalls)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withTypingReaction returned error: %v", err)
	}
	if !callbackRun {
		t.Fatalf("expected callback to run")
	}
	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
	if deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", deleteCalls)
	}
	if emojiType != typingReactionType {
		t.Fatalf("emojiType = %q, want %q", emojiType, typingReactionType)
	}
}

func TestWithTypingReactionDeletesOnCallbackError(t *testing.T) {
	var deleteCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_message/reactions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"reaction_id":"reaction-1"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/open-apis/im/v1/messages/om_message/reactions/reaction-1":
			deleteCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"reaction_id":"reaction-1"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := lark.NewClient("cli_b", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	wantErr := errors.New("callback failed")
	err := withTypingReaction(context.Background(), apiClient, "om_message", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", deleteCalls)
	}
}

func TestWithTypingReactionCreateFailureDoesNotBlockRun(t *testing.T) {
	var (
		createCalls int
		deleteCalls int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_message/reactions":
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":231001,"msg":"reaction type is invalid."}`))
		case r.Method == http.MethodDelete:
			deleteCalls++
			t.Fatalf("delete should not be called when create fails")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := lark.NewClient("cli_c", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	callbackRun := false
	err := withTypingReaction(context.Background(), apiClient, "om_message", func() error {
		callbackRun = true
		return nil
	})
	if err != nil {
		t.Fatalf("withTypingReaction returned error: %v", err)
	}
	if !callbackRun {
		t.Fatalf("expected callback to run")
	}
	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
	if deleteCalls != 0 {
		t.Fatalf("deleteCalls = %d, want 0", deleteCalls)
	}
}

func TestBuildReplyBodyUsesPostMarkdown(t *testing.T) {
	answer := strings.TrimSpace("## Result\n\nSee [TiDB Docs](https://docs.pingcap.com/tidb/stable/) for details.\n\n```sql\nselect 1;\n```")

	body, err := buildReplyBody(answer)
	if err != nil {
		t.Fatalf("buildReplyBody error: %v", err)
	}
	if body.msgType != "post" {
		t.Fatalf("msgType = %q, want post", body.msgType)
	}

	var content struct {
		ZhCN struct {
			Content [][]struct {
				Tag  string `json:"tag"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(body.content), &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if len(content.ZhCN.Content) != 1 || len(content.ZhCN.Content[0]) != 1 {
		t.Fatalf("unexpected content shape: %+v", content.ZhCN.Content)
	}
	node := content.ZhCN.Content[0][0]
	if node.Tag != "md" {
		t.Fatalf("tag = %q, want md", node.Tag)
	}
	if node.Text != answer {
		t.Fatalf("markdown text mismatch:\n got: %q\nwant: %q", node.Text, answer)
	}
	if !strings.Contains(node.Text, "[TiDB Docs](https://docs.pingcap.com/tidb/stable/)") {
		t.Fatalf("hyperlink markdown missing: %q", node.Text)
	}
	if !strings.Contains(node.Text, "```sql\nselect 1;\n```") {
		t.Fatalf("code fence markdown missing: %q", node.Text)
	}
}

func TestBuildReplyBodyNormalizesEmptyText(t *testing.T) {
	body, err := buildReplyBody("   \n\t ")
	if err != nil {
		t.Fatalf("buildReplyBody error: %v", err)
	}
	if body.msgType != "post" {
		t.Fatalf("msgType = %q, want post", body.msgType)
	}

	var content struct {
		ZhCN struct {
			Content [][]struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(body.content), &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if got := content.ZhCN.Content[0][0].Text; got != " " {
		t.Fatalf("text = %q, want single space", got)
	}
}

func TestReplyMessageUsesThreadReplyWhenRequested(t *testing.T) {
	var replyInThread *bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_message/reply":
			var payload struct {
				ReplyInThread *bool `json:"reply_in_thread"`
			}
			if err := readJSONBody(r, &payload); err != nil {
				t.Fatalf("read reply request body: %v", err)
			}
			replyInThread = payload.ReplyInThread
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"message_id":"om_reply","thread_id":"omt_thread"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := lark.NewClient("cli_d", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))
	err := replyMessage(context.Background(), apiClient, "om_message", replyBody{
		msgType: "post",
		content: `{"zh_cn":{"content":[[{"tag":"md","text":"hello"}]]}}`,
	}, replyOptions{preferThread: true})
	if err != nil {
		t.Fatalf("replyMessage returned error: %v", err)
	}
	if replyInThread == nil || !*replyInThread {
		t.Fatalf("expected reply_in_thread=true, got %+v", replyInThread)
	}
}

func TestReplyMessageFallsBackWhenThreadReplyUnsupported(t *testing.T) {
	var replyFlags []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_message/reply":
			var payload struct {
				ReplyInThread *bool `json:"reply_in_thread"`
			}
			if err := readJSONBody(r, &payload); err != nil {
				t.Fatalf("read reply request body: %v", err)
			}
			if payload.ReplyInThread == nil {
				replyFlags = append(replyFlags, "unset")
			} else if *payload.ReplyInThread {
				replyFlags = append(replyFlags, "true")
			} else {
				replyFlags = append(replyFlags, "false")
			}

			w.Header().Set("Content-Type", "application/json")
			if len(replyFlags) == 1 {
				_, _ = w.Write([]byte(`{"code":230071,"msg":"thread reply unsupported"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"message_id":"om_reply"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := lark.NewClient("cli_e", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))
	err := replyMessage(context.Background(), apiClient, "om_message", replyBody{
		msgType: "post",
		content: `{"zh_cn":{"content":[[{"tag":"md","text":"hello"}]]}}`,
	}, replyOptions{preferThread: true})
	if err != nil {
		t.Fatalf("replyMessage returned error: %v", err)
	}
	if got := strings.Join(replyFlags, ","); got != "true,unset" {
		t.Fatalf("reply flags = %q, want true,unset", got)
	}
}

func TestBuildTextReplyBodyNormalizesEmptyText(t *testing.T) {
	body, err := buildTextReplyBody("   \n\t ")
	if err != nil {
		t.Fatalf("buildTextReplyBody error: %v", err)
	}
	if body.msgType != "text" {
		t.Fatalf("msgType = %q, want text", body.msgType)
	}

	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(body.content), &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content.Text != " " {
		t.Fatalf("text = %q, want single space", content.Text)
	}
}

func TestShouldFallbackToTextReply(t *testing.T) {
	if !shouldFallbackToTextReply(usererr.New(usererr.KindInvalidInput, "Feishu rejected the rich reply format or content. Falling back to plain text may work.")) {
		t.Fatalf("expected unsupported format error to fall back")
	}
	if shouldFallbackToTextReply(usererr.New(usererr.KindRateLimit, "Feishu is rate-limiting bot replies right now. Please retry in a moment.")) {
		t.Fatalf("expected rate limit error not to fall back")
	}
}

func readJSONBody(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}
