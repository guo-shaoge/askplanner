package lark

import (
	"encoding/json"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestParseMessageEventText(t *testing.T) {
	content := `{"text":" hello "}`
	messageType := "text"
	messageID := "om_text"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				MessageType: &messageType,
				Content:     &content,
			},
		},
	}

	parsed, err := ParseMessageEvent(event)
	if err != nil {
		t.Fatalf("ParseMessageEvent returned error: %v", err)
	}
	if parsed.UserText != "hello" {
		t.Fatalf("unexpected user text: %q", parsed.UserText)
	}
	if len(parsed.Attachments) != 0 {
		t.Fatalf("expected no attachments, got %d", len(parsed.Attachments))
	}
}

func TestParseMessageEventPostWithAttachments(t *testing.T) {
	payload := map[string]any{
		"post": map[string]any{
			"zh_cn": map[string]any{
				"title": "Update",
				"content": [][]map[string]any{
					{
						{"tag": "text", "text": "See"},
						{"tag": "a", "text": "details", "href": "https://example.com"},
						{"tag": "img", "image_key": "img_123"},
						{"tag": "media", "file_key": "file_456"},
					},
				},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	content := string(data)
	messageType := "post"
	messageID := "om_post"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				MessageType: &messageType,
				Content:     &content,
			},
		},
	}

	parsed, err := ParseMessageEvent(event)
	if err != nil {
		t.Fatalf("ParseMessageEvent returned error: %v", err)
	}
	if parsed.UserText == "" {
		t.Fatalf("expected post text to be extracted")
	}
	if len(parsed.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(parsed.Attachments))
	}
}
