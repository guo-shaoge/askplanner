package larkbot

import (
	"context"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/usererr"
)

func TestShowcaseLarkbotUserFacingMessages(t *testing.T) {
	msgType := "text"
	text := `{"text":"/upload_x analyze these files"}`
	openID := "ou_user"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				Content:     &text,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	_, err := prepareReply(context.Background(), nil, nil, event)
	if got := usererr.Message(err); got != "Invalid upload command. Use `/upload_<n> your question`, for example `/upload_3 analyze these files`." {
		t.Fatalf("invalid_upload message = %q", got)
	} else {
		t.Logf("invalid_upload => %s", got)
	}

	testCases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "recent_message_rate_limit",
			err:  classifyFeishuResponseError(feishuOpListRecentMessages, "fallback", 429, "too many requests"),
			want: "Feishu is rate-limiting recent-message lookups right now. Please retry in a moment.",
		},
		{
			name: "attachment_permission",
			err:  classifyFeishuResponseError(feishuOpDownloadAttachment, "fallback", 403, "permission denied"),
			want: "The bot isn't allowed to download this attachment from Feishu. Check attachment permissions or resend it.",
		},
		{
			name: "attachment_not_found",
			err:  classifyFeishuResponseError(feishuOpDownloadAttachment, "fallback", 400, "file not found"),
			want: "The selected attachment is no longer available in Feishu. Please resend it.",
		},
		{
			name: "reply_unsupported_format",
			err:  classifyFeishuResponseError(feishuOpReplyMessage, "fallback", 400, "invalid content format"),
			want: "Feishu rejected the rich reply format or content. Falling back to plain text may work.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := usererr.Message(tc.err)
			if got != tc.want {
				t.Fatalf("user-facing message = %q, want %q", got, tc.want)
			}
			t.Logf("%s => %s", tc.name, got)
		})
	}
}
