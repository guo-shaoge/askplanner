package main

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestParseDownloadCommand(t *testing.T) {
	cmd := parseDownloadCommand("/download_3 analyze these files")
	if !cmd.ok {
		t.Fatalf("expected command to parse")
	}
	if cmd.count != 3 {
		t.Fatalf("count = %d, want 3", cmd.count)
	}
	if cmd.remainder != "analyze these files" {
		t.Fatalf("remainder = %q", cmd.remainder)
	}

	if bad := parseDownloadCommand("/download_x test"); bad.ok {
		t.Fatalf("expected invalid command to be rejected")
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
