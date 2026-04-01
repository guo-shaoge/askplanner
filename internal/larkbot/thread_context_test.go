package larkbot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
)

func TestPrepareReplyLoadsThreadContextForCodexQuestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages":
			if got := r.URL.Query().Get("container_id_type"); got != "thread" {
				t.Fatalf("container_id_type = %q, want thread", got)
			}
			if got := r.URL.Query().Get("container_id"); got != "omt-thread" {
				t.Fatalf("container_id = %q, want omt-thread", got)
			}
			if got := r.URL.Query().Get("sort_type"); got != "ByCreateTimeDesc" {
				t.Fatalf("sort_type = %q, want ByCreateTimeDesc", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":false,
					"items":[
						{
							"message_id":"om_newer",
							"root_id":"om_root",
							"parent_id":"om_root",
							"thread_id":"omt-thread",
							"msg_type":"text",
							"create_time":"1711188180000",
							"sender":{"id":"ou_newer","id_type":"open_id","sender_type":"user"},
							"body":{"content":"{\"text\":\"更新的回复\"}"}
						},
						{
							"message_id":"om_current",
							"root_id":"om_root",
							"parent_id":"om_root",
							"thread_id":"omt-thread",
							"msg_type":"text",
							"create_time":"1711188120000",
							"sender":{"id":"ou_current","id_type":"open_id","sender_type":"user"},
							"body":{"content":"{\"text\":\"当前问题\"}"}
						},
						{
							"message_id":"om_prev",
							"root_id":"om_root",
							"parent_id":"om_root",
							"thread_id":"omt-thread",
							"msg_type":"post",
							"create_time":"1711188060000",
							"sender":{"id":"ou_bob","id_type":"open_id","sender_type":"user"},
							"body":{"content":"{\"zh_cn\":{\"content\":[[{\"tag\":\"text\",\"text\":\"上一条回复\"}],[{\"tag\":\"text\",\"text\":\"第二行\"}]]}}"}
						}
					]
				}
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages/om_root":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"items":[
						{
							"message_id":"om_root",
							"thread_id":"omt-thread",
							"msg_type":"text",
							"create_time":"1711188000000",
							"sender":{"id":"ou_alice","id_type":"open_id","sender_type":"user"},
							"body":{"content":"{\"text\":\"@_user_1 线程根消息\"}"},
							"mentions":[{"key":"@_user_1","name":"askplanner"}]
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	apiClient := lark.NewClient("cli_test", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	msgType := "text"
	content := `{"text":"@_user_1 当前问题"}`
	threadID := "omt-thread"
	chatID := "oc-chat"
	messageID := "om_current"
	rootID := "om_root"
	parentID := "om_root"
	createTime := "1711188120000"
	key := "@_user_1"
	name := "askplanner"
	openID := "ou_current"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
				ThreadId:    &threadID,
				ChatId:      &chatID,
				MessageId:   &messageID,
				RootId:      &rootID,
				ParentId:    &parentID,
				CreateTime:  &createTime,
				Mentions: []*larkim.MentionEvent{{
					Key:  &key,
					Name: &name,
				}},
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), apiClient, manager, event, botIdentity{key: "bot-a"})
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	if reply.threadCtxLoader == nil {
		t.Fatalf("expected thread context loader to be set")
	}
	threadCtx, err := reply.threadCtxLoader(context.Background())
	if err != nil {
		t.Fatalf("threadCtxLoader returned error: %v", err)
	}
	if threadCtx.ThreadID != "omt-thread" {
		t.Fatalf("thread id = %q", threadCtx.ThreadID)
	}
	if len(threadCtx.Messages) != 2 {
		t.Fatalf("thread messages = %d, want 2", len(threadCtx.Messages))
	}
	if got := threadCtx.Messages[0].Content; got != "@askplanner 线程根消息" {
		t.Fatalf("root content = %q", got)
	}
	if got := threadCtx.Messages[1].Content; got != "上一条回复\n第二行" {
		t.Fatalf("previous content = %q", got)
	}
}

func TestPrepareReplyLoadsThreadContextAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages" && r.URL.Query().Get("page_token") == "":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":true,
					"page_token":"page-2",
					"items":[
						{"message_id":"om_current","thread_id":"omt-thread","msg_type":"text","create_time":"1711189200000","sender":{"id":"ou_current","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"当前问题\"}"}},
						{"message_id":"om_prev_01","thread_id":"omt-thread","msg_type":"text","create_time":"1711189190000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 01\"}"}},
						{"message_id":"om_prev_02","thread_id":"omt-thread","msg_type":"text","create_time":"1711189180000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 02\"}"}},
						{"message_id":"om_prev_03","thread_id":"omt-thread","msg_type":"text","create_time":"1711189170000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 03\"}"}},
						{"message_id":"om_prev_04","thread_id":"omt-thread","msg_type":"text","create_time":"1711189160000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 04\"}"}},
						{"message_id":"om_prev_05","thread_id":"omt-thread","msg_type":"text","create_time":"1711189150000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 05\"}"}},
						{"message_id":"om_prev_06","thread_id":"omt-thread","msg_type":"text","create_time":"1711189140000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 06\"}"}},
						{"message_id":"om_prev_07","thread_id":"omt-thread","msg_type":"text","create_time":"1711189130000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 07\"}"}},
						{"message_id":"om_prev_08","thread_id":"omt-thread","msg_type":"text","create_time":"1711189120000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 08\"}"}},
						{"message_id":"om_prev_09","thread_id":"omt-thread","msg_type":"text","create_time":"1711189110000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 09\"}"}},
						{"message_id":"om_prev_10","thread_id":"omt-thread","msg_type":"text","create_time":"1711189100000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 10\"}"}}
					]
				}
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages" && r.URL.Query().Get("page_token") == "page-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":false,
					"items":[
						{"message_id":"om_prev_11","thread_id":"omt-thread","msg_type":"text","create_time":"1711189090000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 11\"}"}},
						{"message_id":"om_prev_12","thread_id":"omt-thread","msg_type":"text","create_time":"1711189080000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 12\"}"}},
						{"message_id":"om_prev_13","thread_id":"omt-thread","msg_type":"text","create_time":"1711189070000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 13\"}"}},
						{"message_id":"om_prev_14","thread_id":"omt-thread","msg_type":"text","create_time":"1711189060000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 14\"}"}},
						{"message_id":"om_prev_15","thread_id":"omt-thread","msg_type":"text","create_time":"1711189050000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 15\"}"}},
						{"message_id":"om_prev_16","thread_id":"omt-thread","msg_type":"text","create_time":"1711189040000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 16\"}"}},
						{"message_id":"om_prev_17","thread_id":"omt-thread","msg_type":"text","create_time":"1711189030000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 17\"}"}},
						{"message_id":"om_prev_18","thread_id":"omt-thread","msg_type":"text","create_time":"1711189020000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 18\"}"}},
						{"message_id":"om_prev_19","thread_id":"omt-thread","msg_type":"text","create_time":"1711189010000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 19\"}"}},
						{"message_id":"om_prev_20","thread_id":"omt-thread","msg_type":"text","create_time":"1711189000000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"历史 20\"}"}}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	apiClient := lark.NewClient("cli_test", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	msgType := "text"
	content := `{"text":"当前问题"}`
	threadID := "omt-thread"
	chatID := "oc-chat"
	messageID := "om_current"
	createTime := "1711189200000"
	openID := "ou_current"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
				ThreadId:    &threadID,
				ChatId:      &chatID,
				MessageId:   &messageID,
				CreateTime:  &createTime,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), apiClient, manager, event, botIdentity{key: "bot-a"})
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	if reply.threadCtxLoader == nil {
		t.Fatalf("expected thread context loader to be set")
	}
	threadCtx, err := reply.threadCtxLoader(context.Background())
	if err != nil {
		t.Fatalf("threadCtxLoader returned error: %v", err)
	}
	if len(threadCtx.Messages) != promptThreadMessageLimit {
		t.Fatalf("thread messages = %d, want %d", len(threadCtx.Messages), promptThreadMessageLimit)
	}
	if got := threadCtx.Messages[0].Content; got != "历史 20" {
		t.Fatalf("oldest collected content = %q", got)
	}
	if got := threadCtx.Messages[len(threadCtx.Messages)-1].Content; got != "历史 01" {
		t.Fatalf("newest collected content = %q", got)
	}
}

func TestCompactThreadContentPreservesIndentation(t *testing.T) {
	got := compactThreadContent("  line 1  \n\n\n\n    child  ")
	if got != "  line 1\n\n    child" {
		t.Fatalf("compactThreadContent = %q", got)
	}
}

func TestExtractThreadMessageContentUsesPlaceholdersForNonText(t *testing.T) {
	msgType := "interactive"
	raw := `{"foo":"bar"}`
	content := extractThreadMessageContent(&larkim.Message{
		MsgType: &msgType,
		Body:    &larkim.MessageBody{Content: &raw},
	})
	if !strings.Contains(content, "interactive") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestRewriteMentionKeysReplacesLongerKeysFirst(t *testing.T) {
	key1 := "@_user_1"
	name1 := "alice"
	key10 := "@_user_10"
	name10 := "bob"

	got := rewriteMentionKeys("@_user_10 hi @_user_1", []*larkim.Mention{
		{Key: &key1, Name: &name1},
		{Key: &key10, Name: &name10},
	})
	if got != "@bob hi @alice" {
		t.Fatalf("rewriteMentionKeys = %q", got)
	}
}

func TestRewriteMentionKeysHandlesNilMentions(t *testing.T) {
	key := "@_user_2"
	name := "bob"

	got := rewriteMentionKeys("hi @_user_2", []*larkim.Mention{
		nil,
		{Key: &key, Name: &name},
	})
	if got != "hi @bob" {
		t.Fatalf("rewriteMentionKeys = %q", got)
	}
}

func TestRewriteMentionKeysPreservesKeyWhenNameMissing(t *testing.T) {
	key := "@_user_2"

	got := rewriteMentionKeys("hi @_user_2", []*larkim.Mention{
		{Key: &key},
	})
	if got != "hi @_user_2" {
		t.Fatalf("rewriteMentionKeys = %q", got)
	}
}

func TestExtractThreadMessageContentRewritesMentionsInTextWithoutAtBot(t *testing.T) {
	msgType := "text"
	raw := `{"text":"@_user_1 hi @_user_2","text_without_at_bot":"hi @_user_2"}`
	key1 := "@_user_1"
	name1 := "askplanner"
	key2 := "@_user_2"
	name2 := "bob"

	content := extractThreadMessageContent(&larkim.Message{
		MsgType: &msgType,
		Body:    &larkim.MessageBody{Content: &raw},
		Mentions: []*larkim.Mention{
			{Key: &key1, Name: &name1},
			{Key: &key2, Name: &name2},
		},
	})
	if content != "hi @bob" {
		t.Fatalf("content = %q", content)
	}
}

func TestPrepareReplyKeepsThreadHistoryWhenRootFetchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":false,
					"items":[
						{"message_id":"om_current","root_id":"om_root","parent_id":"om_root","thread_id":"omt-thread","msg_type":"text","create_time":"1711188120000","sender":{"id":"ou_current","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"当前问题\"}"}},
						{"message_id":"om_prev","root_id":"om_root","parent_id":"om_root","thread_id":"omt-thread","msg_type":"text","create_time":"1711188060000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"上一条\"}"}}
					]
				}
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages/om_root":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":123,"msg":"boom"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	apiClient := lark.NewClient("cli_test", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	msgType := "text"
	content := `{"text":"当前问题"}`
	threadID := "omt-thread"
	chatID := "oc-chat"
	messageID := "om_current"
	rootID := "om_root"
	parentID := "om_root"
	createTime := "1711188120000"
	openID := "ou_current"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
				ThreadId:    &threadID,
				ChatId:      &chatID,
				MessageId:   &messageID,
				RootId:      &rootID,
				ParentId:    &parentID,
				CreateTime:  &createTime,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), apiClient, manager, event, botIdentity{key: "bot-a"})
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	if reply.threadCtxLoader == nil {
		t.Fatalf("expected thread context loader to be set")
	}
	threadCtx, err := reply.threadCtxLoader(context.Background())
	if err != nil {
		t.Fatalf("threadCtxLoader returned error: %v", err)
	}
	if len(threadCtx.Messages) != 1 {
		t.Fatalf("thread messages = %d, want 1", len(threadCtx.Messages))
	}
	if got := threadCtx.Messages[0].Content; got != "上一条" {
		t.Fatalf("previous content = %q", got)
	}
}

func TestThreadContextFallsBackToLatestHistoryWhenCurrentNotVisibleYet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":false,
					"items":[
						{"message_id":"om_future_reply","thread_id":"omt-thread","msg_type":"text","create_time":"1711189210000","sender":{"id":"ou_future","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"未来回复\"}"}},
						{"message_id":"om_latest_1","thread_id":"omt-thread","msg_type":"text","create_time":"1711189180000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"最近历史 1\"}"}},
						{"message_id":"om_latest_2","thread_id":"omt-thread","msg_type":"text","create_time":"1711189170000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"最近历史 2\"}"}}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	apiClient := lark.NewClient("cli_test", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	msgType := "text"
	content := `{"text":"当前问题"}`
	threadID := "omt-thread"
	chatID := "oc-chat"
	messageID := "om_current_not_visible_yet"
	createTime := "1711189200000"
	openID := "ou_current"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
				ThreadId:    &threadID,
				ChatId:      &chatID,
				MessageId:   &messageID,
				CreateTime:  &createTime,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), apiClient, manager, event, botIdentity{key: "bot-a"})
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	if reply.threadCtxLoader == nil {
		t.Fatalf("expected thread context loader to be set")
	}
	threadCtx, err := reply.threadCtxLoader(context.Background())
	if err != nil {
		t.Fatalf("threadCtxLoader returned error: %v", err)
	}
	if len(threadCtx.Messages) != 2 {
		t.Fatalf("thread messages = %d, want 2", len(threadCtx.Messages))
	}
	if got := threadCtx.Messages[0].Content; got != "最近历史 2" {
		t.Fatalf("oldest fallback content = %q", got)
	}
	if got := threadCtx.Messages[1].Content; got != "最近历史 1" {
		t.Fatalf("newest fallback content = %q", got)
	}
	for _, msg := range threadCtx.Messages {
		if msg.Content == "未来回复" {
			t.Fatalf("future reply leaked into thread context: %+v", threadCtx.Messages)
		}
	}
}

func TestThreadContextPreservesFeishuOrderForSameMillisecondMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","expire":7200,"tenant_access_token":"tenant-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/open-apis/im/v1/messages":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"code":0,
				"msg":"success",
				"data":{
					"has_more":false,
					"items":[
						{"message_id":"om_current","thread_id":"omt-thread","msg_type":"text","create_time":"1711189200000","sender":{"id":"ou_current","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"当前问题\"}"}},
						{"message_id":"om_prev_b","thread_id":"omt-thread","msg_type":"text","create_time":"1711189190000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"同毫秒回复 B\"}"}},
						{"message_id":"om_prev_a","thread_id":"omt-thread","msg_type":"text","create_time":"1711189190000","sender":{"id":"ou_prev","id_type":"open_id","sender_type":"user"},"body":{"content":"{\"text\":\"同毫秒回复 A\"}"}}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	apiClient := lark.NewClient("cli_test", "secret", lark.WithOpenBaseUrl(server.URL), lark.WithEnableTokenCache(false))

	msgType := "text"
	content := `{"text":"当前问题"}`
	threadID := "omt-thread"
	chatID := "oc-chat"
	messageID := "om_current"
	createTime := "1711189200000"
	openID := "ou_current"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
				ThreadId:    &threadID,
				ChatId:      &chatID,
				MessageId:   &messageID,
				CreateTime:  &createTime,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), apiClient, manager, event, botIdentity{key: "bot-a"})
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	if reply.threadCtxLoader == nil {
		t.Fatalf("expected thread context loader to be set")
	}
	threadCtx, err := reply.threadCtxLoader(context.Background())
	if err != nil {
		t.Fatalf("threadCtxLoader returned error: %v", err)
	}
	if len(threadCtx.Messages) != 2 {
		t.Fatalf("thread messages = %d, want 2", len(threadCtx.Messages))
	}
	if got := threadCtx.Messages[0].Content; got != "同毫秒回复 B" {
		t.Fatalf("first same-millisecond content = %q", got)
	}
	if got := threadCtx.Messages[1].Content; got != "同毫秒回复 A" {
		t.Fatalf("second same-millisecond content = %q", got)
	}
}
