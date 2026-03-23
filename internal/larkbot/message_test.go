package larkbot

import (
	"context"
	"strings"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
)

func TestPrepareReplyExtractsPostMessagePreservingLayout(t *testing.T) {
	msgType := "post"
	content := `{"zh_cn":{"content":[[{"tag":"at","user_name":"OptX"}],[{"tag":"text","text":"这个查询计划还有优化空间吗"}],[{"tag":"text","text":"| id | estRows | task |"}],[{"tag":"text","text":"| Point_Get_1 | 1.00 | root |"}],[{"tag":"text","text":"> ref"}]]}}`
	openID := "ou_user"
	manager, err := attachments.NewManager(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	reply, err := prepareReply(context.Background(), nil, manager, event)
	if err != nil {
		t.Fatalf("prepareReply returned error: %v", err)
	}
	want := strings.Join([]string{
		"@OptX",
		"这个查询计划还有优化空间吗",
		"| id | estRows | task |",
		"| Point_Get_1 | 1.00 | root |",
		"> ref",
	}, "\n")
	if reply.question != want {
		t.Fatalf("question mismatch:\n got: %q\nwant: %q", reply.question, want)
	}
}

func TestExtractTextMessagePreservesNewlinesWhenStrippingMentions(t *testing.T) {
	msgType := "text"
	text := `{"text":"@_user_1\n| id          | estRows |\n| Point_Get_1 | 1.00 |"}`
	key := "@_user_1"
	name := "OptX"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &text,
				Mentions: []*larkim.MentionEvent{{
					Key:  &key,
					Name: &name,
				}},
			},
		},
	}

	got := extractQuestionText(event)
	want := strings.Join([]string{
		"| id | estRows |",
		"| Point_Get_1 | 1.00 |",
	}, "\n")
	if got != want {
		t.Fatalf("question mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestShouldHandleEventAcceptsGroupPostMentioningBot(t *testing.T) {
	msgType := "post"
	chatType := "group"
	content := `{"zh_cn":{"content":[[{"tag":"at","user_name":"OptX"},{"tag":"text","text":" 这个查询计划还有优化空间吗"}]]}}`
	name := "OptX"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				Content:     &content,
				Mentions: []*larkim.MentionEvent{{
					Name: &name,
				}},
			},
		},
	}

	ok, reason := shouldHandleEvent(event, botIdentity{name: "OptX"})
	if !ok {
		t.Fatalf("shouldHandleEvent rejected group post: %s", reason)
	}
}

func TestShouldHandleEventAcceptsGroupPostMentioningBotWithoutMentionsField(t *testing.T) {
	msgType := "post"
	chatType := "group"
	content := `{"zh_cn":{"content":[[{"tag":"at","user_name":"OptX"},{"tag":"text","text":" 这个查询计划还有优化空间吗"}]]}}`
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				Content:     &content,
			},
		},
	}

	ok, reason := shouldHandleEvent(event, botIdentity{name: "OptX"})
	if !ok {
		t.Fatalf("shouldHandleEvent rejected group post without mentions: %s", reason)
	}
}

func TestBuildConversationKeyUsesThreadAndUser(t *testing.T) {
	threadID := "omt-thread"
	chatID := "oc_chat"
	openID := "ou_user"
	messageID := "om_message"

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				ThreadId:  &threadID,
				ChatId:    &chatID,
				MessageId: &messageID,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if got := buildConversationKey(event); got != "lark:thread:omt-thread:user:ou_user" {
		t.Fatalf("conversation key = %q", got)
	}
}
