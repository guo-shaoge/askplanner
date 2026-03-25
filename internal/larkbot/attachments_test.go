package larkbot

import (
	"context"
	"strings"
	"testing"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/usererr"
)

func TestParseUploadCommand(t *testing.T) {
	cmd := parseUploadCommand("/upload_3 analyze these files")
	if !cmd.ok {
		t.Fatalf("expected command to parse")
	}
	if cmd.count != 3 {
		t.Fatalf("count = %d, want 3", cmd.count)
	}
	if cmd.remainder != "analyze these files" {
		t.Fatalf("remainder = %q", cmd.remainder)
	}

	if bad := parseUploadCommand("/upload_x test"); bad.ok {
		t.Fatalf("expected invalid command to be rejected")
	}
	if bad := parseUploadCommand("/upload_x test"); !bad.matched {
		t.Fatalf("expected invalid command to keep matched=true")
	}
}

func TestPrepareReplyRejectsInvalidUploadCommand(t *testing.T) {
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
	if got := usererr.Message(err); !strings.Contains(got, "Invalid upload command") {
		t.Fatalf("user-facing message = %q", got)
	}
}

func TestBuildSaveSummaryDoesNotExposeLocalPath(t *testing.T) {
	summary := buildSaveSummary("Downloaded", []attachments.SaveResult{{
		UserDir: "/home/gjt/work/askplanner/.askplanner/lark-files/ou_xxx",
		Item: attachments.Item{
			Name:      "image_20260320_091914_om_x.png",
			Type:      attachments.ItemTypeImage,
			CreatedAt: time.Now(),
		},
	}})

	if strings.Contains(summary, "/home/gjt/work/askplanner/.askplanner/lark-files/ou_xxx") {
		t.Fatalf("summary leaked local path: %s", summary)
	}
	if !strings.Contains(summary, "image_20260320_091914_om_x.png [image]") {
		t.Fatalf("summary missing file entry: %s", summary)
	}
}

func TestMatchAttachmentMessage(t *testing.T) {
	threadID := "thread-1"
	currentMessageID := "om-current"
	currentCreateAt := time.UnixMilli(2_000)
	senderID := "ou_user"

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				ThreadId: &threadID,
			},
		},
	}
	senderIDs := map[string]struct{}{senderID: {}}

	tests := []struct {
		name string
		item *larkim.Message
		ok   bool
	}{
		{
			name: "match file attachment",
			item: newAttachmentMessage("om-prev", "file", `{"file_key":"file-key"}`, threadID, senderID, "open_id", "1000"),
			ok:   true,
		},
		{
			name: "skip thread mismatch",
			item: newAttachmentMessage("om-prev", "file", `{"file_key":"file-key"}`, "other-thread", senderID, "open_id", "1000"),
			ok:   false,
		},
		{
			name: "skip sender mismatch",
			item: newAttachmentMessage("om-prev", "file", `{"file_key":"file-key"}`, threadID, "ou_other", "open_id", "1000"),
			ok:   false,
		},
		{
			name: "skip current message",
			item: newAttachmentMessage(currentMessageID, "file", `{"file_key":"file-key"}`, threadID, senderID, "open_id", "1000"),
			ok:   false,
		},
		{
			name: "skip future message",
			item: newAttachmentMessage("om-prev", "file", `{"file_key":"file-key"}`, threadID, senderID, "open_id", "3000"),
			ok:   false,
		},
		{
			name: "skip missing file key",
			item: newAttachmentMessage("om-prev", "image", `{"image_key":""}`, threadID, senderID, "open_id", "1000"),
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, ok := matchAttachmentMessage(tt.item, event, senderIDs, currentMessageID, currentCreateAt)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if tt.ok {
				if ref == nil {
					t.Fatalf("ref is nil")
				}
				if ref.fileKey != "file-key" {
					t.Fatalf("fileKey = %q, want file-key", ref.fileKey)
				}
			}
		})
	}
}

func newAttachmentMessage(messageID, msgType, body, threadID, senderID, senderType, createTime string) *larkim.Message {
	return &larkim.Message{
		MessageId:  stringPtr(messageID),
		MsgType:    stringPtr(msgType),
		ThreadId:   stringPtr(threadID),
		CreateTime: stringPtr(createTime),
		Body: &larkim.MessageBody{
			Content: stringPtr(body),
		},
		Sender: &larkim.Sender{
			Id:     stringPtr(senderID),
			IdType: stringPtr(senderType),
		},
	}
}

func stringPtr(v string) *string {
	return &v
}
