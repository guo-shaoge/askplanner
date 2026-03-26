package larkbot

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type fakeConversationRouteResolver struct {
	selected     string
	usedFallback bool
}

func (f fakeConversationRouteResolver) ResolveExistingConversationKey(preferred string, candidates ...string) (string, bool) {
	if f.selected != "" {
		return f.selected, f.usedFallback
	}
	return preferred, false
}

func TestResolveConversationRouteUsesLegacyFallbackForExistingGroupSession(t *testing.T) {
	chatType := "group"
	chatID := "oc_chat"
	openID := "ou_user"
	messageID := "om_message"

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				ChatType:  &chatType,
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

	route := resolveConversationRoute(event, "lark:root:om_message:user:ou_user", fakeConversationRouteResolver{
		selected:     "lark:chat:oc_chat:user:ou_user",
		usedFallback: true,
	})
	if route.key != "lark:chat:oc_chat:user:ou_user" {
		t.Fatalf("conversation key = %q", route.key)
	}
	if route.preferThread {
		t.Fatalf("expected legacy route to keep non-thread reply")
	}
	if !route.legacy {
		t.Fatalf("expected legacy route flag")
	}
}

func TestResolveConversationRouteKeepsThreadReplyForNewGroupSession(t *testing.T) {
	chatType := "group"
	chatID := "oc_chat"
	openID := "ou_user"
	messageID := "om_message"

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				ChatType:  &chatType,
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

	route := resolveConversationRoute(event, "lark:root:om_message:user:ou_user", fakeConversationRouteResolver{})
	if route.key != "lark:root:om_message:user:ou_user" {
		t.Fatalf("conversation key = %q", route.key)
	}
	if !route.preferThread {
		t.Fatalf("expected new group session to prefer thread reply")
	}
	if route.legacy {
		t.Fatalf("did not expect legacy route flag")
	}
}
