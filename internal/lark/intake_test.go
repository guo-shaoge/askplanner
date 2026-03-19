package lark

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lab/askplanner/internal/codex"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestDefaultUploadUserMessageForAttachmentOnlyRequestsAcknowledgement(t *testing.T) {
	got := defaultUploadUserMessage([]codex.Attachment{{
		Kind:     "plan_replayer_zip",
		PublicID: "bundle-abcd1234",
	}})
	for _, want := range []string{
		"can see the file or files",
		"must include the ID",
		"Do not analyze the files yet",
		"bundle-abcd1234",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected synthesized message to contain %q, got %q", want, got)
		}
	}
}

func TestExtractReferencedPublicIDs(t *testing.T) {
	got := extractReferencedPublicIDs("please analyze bundle-abcd1234 and bundle-ef567890")
	if len(got) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(got))
	}
}

func TestBuildTextRequestWithReferences(t *testing.T) {
	root := t.TempDir()
	store := NewBundleStore(root)
	bundle, err := store.Create("lark:thread:oc_123", "om_789", "file", time.Hour)
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if err := bundle.AddAttachment(BundleAttachmentMetadata{
		Kind:         "plan_replayer_zip",
		PublicID:     bundle.ExtractedPublicID,
		OriginalName: "trace.zip",
		SavedPath:    bundle.RawDir + "/trace.zip",
		ExtractedDir: bundle.ExtractedDir + "/trace",
	}); err != nil {
		t.Fatalf("add attachment: %v", err)
	}

	intake := &Intake{Store: store}
	req, err := intake.buildTextRequestWithReferences("please analyze " + bundle.ExtractedPublicID)
	if err != nil {
		t.Fatalf("buildTextRequestWithReferences: %v", err)
	}
	if len(req.Attachments) != 1 {
		t.Fatalf("expected one resolved attachment, got %d", len(req.Attachments))
	}
	if req.Attachments[0].PublicID != bundle.ExtractedPublicID {
		t.Fatalf("unexpected public id: %q", req.Attachments[0].PublicID)
	}
}

func TestBuildRequestReturnsUserVisibleInstructionForEmptyText(t *testing.T) {
	content := `{"text":"   "}`
	messageType := "text"
	messageID := "om_empty"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				MessageType: &messageType,
				Content:     &content,
			},
		},
	}

	intake := &Intake{Store: NewBundleStore(t.TempDir())}
	_, err := intake.BuildRequest(t.Context(), "lark:thread:oc_123", event)
	if err == nil {
		t.Fatalf("expected empty text to return a user-visible error")
	}
	userErr, ok := AsUserVisibleError(err)
	if !ok {
		t.Fatalf("expected user-visible error, got %v", err)
	}
	for _, want := range []string{
		"Images and PLAN REPLAYER .zip files should be sent through Lark by themselves",
		"includes that ID",
	} {
		if !strings.Contains(userErr.Message, want) {
			t.Fatalf("expected error to contain %q, got %q", want, userErr.Message)
		}
	}
}

func TestBuildRequestForAttachmentUploadUsesPublicIDPrompt(t *testing.T) {
	payload := map[string]any{"file_key": "file_123"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	content := string(data)
	messageType := "file"
	messageID := "om_789"
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				MessageType: &messageType,
				Content:     &content,
			},
		},
	}

	root := t.TempDir()
	intake := NewIntake(fakeMessageResourceClient{resp: fakeZipResourceResponse()}, root, time.Hour, 1024)
	req, err := intake.BuildRequest(t.Context(), "lark:thread:oc_123", event)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}
	expectedID := publicReferenceID("lark:thread:oc_123", "om_789", "extracted")
	if !strings.Contains(req.UserMessage, expectedID) {
		t.Fatalf("expected upload prompt to contain public id %q, got %q", expectedID, req.UserMessage)
	}
	if len(req.Attachments) != 1 || req.Attachments[0].PublicID != expectedID {
		t.Fatalf("unexpected attachments: %+v", req.Attachments)
	}
}
