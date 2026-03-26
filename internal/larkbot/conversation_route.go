package larkbot

import (
	"log"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// TODO(cleanup): Remove this file after the legacy group-session TTL window has
// elapsed. It exists only to keep pre-thread-reply group sessions on their old
// key/reply behavior during rollout.

type conversationRouteResolver interface {
	ResolveExistingConversationKey(preferred string, candidates ...string) (string, bool)
}

type conversationRoute struct {
	key          string
	preferThread bool
	legacy       bool
}

func resolveConversationRoute(event *larkim.P2MessageReceiveV1, preferred string, resolver conversationRouteResolver) conversationRoute {
	route := conversationRoute{
		key:          preferred,
		preferThread: shouldReplyInThread(event),
	}

	legacy := buildLegacyConversationKey(event)
	if strings.TrimSpace(preferred) == "" || legacy == "" || legacy == preferred || resolver == nil {
		return route
	}

	selected, usedFallback := resolver.ResolveExistingConversationKey(preferred, legacy)
	route.key = selected
	route.legacy = usedFallback
	if usedFallback {
		route.preferThread = false
		log.Printf("[larkbot] using legacy conversation route preferred=%s legacy=%s selected=%s",
			preferred, legacy, selected)
	}
	return route
}

func shouldReplyInThread(event *larkim.P2MessageReceiveV1) bool {
	return isGroupChat(event) && extractThreadID(event) == ""
}

func buildLegacyConversationKey(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil {
		return "lark:unknown"
	}

	threadID := extractThreadID(event)
	chatID := extractChatID(event)
	senderID := sanitizePathSegment(extractPreferredSenderID(event), "")
	messageID := extractMessageID(event)

	switch {
	case threadID != "" && senderID != "":
		return "lark:thread:" + threadID + ":user:" + senderID
	case chatID != "" && senderID != "":
		return "lark:chat:" + chatID + ":user:" + senderID
	case threadID != "":
		return "lark:thread:" + threadID
	case chatID != "":
		return "lark:chat:" + chatID
	case messageID != "":
		return "lark:message:" + messageID
	default:
		return "lark:unknown"
	}
}
