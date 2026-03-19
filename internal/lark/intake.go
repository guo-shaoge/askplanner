package lark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lab/askplanner/internal/codex"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type UserVisibleError struct {
	Message string
}

func (e *UserVisibleError) Error() string {
	return e.Message
}

type Intake struct {
	Store   *BundleStore
	Fetcher *ResourceFetcher
	TTL     time.Duration
}

func NewIntake(client MessageResourceClient, root string, ttl time.Duration, maxBytes int64) *Intake {
	return &Intake{
		Store: NewBundleStore(root),
		Fetcher: &ResourceFetcher{
			Client:   client,
			MaxBytes: maxBytes,
		},
		TTL: ttl,
	}
}

func (i *Intake) BuildRequest(ctx context.Context, conversationKey string, event *larkim.P2MessageReceiveV1) (codex.Request, error) {
	parsed, err := ParseMessageEvent(event)
	if err != nil {
		return codex.Request{}, err
	}
	if strings.TrimSpace(parsed.MessageID) == "" {
		return codex.Request{}, &UserVisibleError{Message: "Unable to process attachments because the Lark message_id is empty."}
	}
	if len(parsed.Attachments) == 0 {
		switch parsed.MessageType {
		case "text", "post":
			if strings.TrimSpace(parsed.UserText) == "" {
				return codex.Request{}, &UserVisibleError{Message: "The message is empty. Please send text, a PLAN REPLAYER .zip file, or an image."}
			}
			return codex.NewTextRequest(parsed.UserText), nil
		default:
			return codex.Request{}, &UserVisibleError{Message: "This Lark message type is not supported yet. Please send text, a PLAN REPLAYER .zip file, or an image."}
		}
	}

	bundle, err := i.Store.Create(conversationKey, parsed.MessageID, parsed.MessageType, i.TTL)
	if err != nil {
		return codex.Request{}, err
	}
	cleanupBundle := true
	defer func() {
		if cleanupBundle {
			_ = os.RemoveAll(bundle.Dir)
		}
	}()

	request := codex.Request{
		UserMessage: defaultUserMessage(parsed.UserText, parsed.Attachments),
		RuntimeNotes: []string{
			fmt.Sprintf("Lark conversation key: %s", strings.TrimSpace(conversationKey)),
			fmt.Sprintf("Lark message ID: %s", strings.TrimSpace(parsed.MessageID)),
			fmt.Sprintf("Attachment bundle directory: %s", bundle.Dir),
			fmt.Sprintf("Attachment bundle expires at: %s", bundle.Meta.ExpiresAt.Format(time.RFC3339)),
		},
	}

	for _, ref := range parsed.Attachments {
		saved, err := i.Fetcher.Fetch(ctx, parsed.MessageID, ref, bundle.RawDir)
		if err != nil {
			return codex.Request{}, err
		}

		attachmentMeta := BundleAttachmentMetadata{
			Kind:         ref.Kind,
			OriginalName: saved.OriginalName,
			SavedPath:    saved.SavedPath,
		}
		attachment := codex.Attachment{
			Kind:         ref.Kind,
			OriginalName: saved.OriginalName,
			SavedPath:    saved.SavedPath,
		}

		switch ref.Kind {
		case AttachmentKindImage:
			attachment.Notes = append(attachment.Notes, "Image downloaded from the Lark message resource API.")
		case AttachmentKindFile:
			if !strings.EqualFold(filepath.Ext(saved.OriginalName), ".zip") {
				return codex.Request{}, &UserVisibleError{Message: "Only PLAN REPLAYER .zip files and images are supported right now."}
			}
			extractedDir := filepath.Join(bundle.ExtractedDir, strings.TrimSuffix(filepath.Base(saved.OriginalName), filepath.Ext(saved.OriginalName)))
			manifest, err := ExtractPlanReplayer(saved.SavedPath, extractedDir)
			if err != nil {
				return codex.Request{}, err
			}
			attachment.Kind = "plan_replayer_zip"
			attachment.ExtractedDir = extractedDir
			attachment.Notes = append(attachment.Notes, "PLAN REPLAYER ZIP extracted for Codex inspection.")
			if len(manifest.DetectedFiles) > 0 {
				// todo delte this, codex should know how to read the extracted dir and find these files by itself, no need to tell it what we found, just give it the extracted dir and let it figure out what to do with it
				// attachment.Notes = append(attachment.Notes, "Detected PLAN REPLAYER files: "+strings.Join(manifest.DetectedFiles, ", "))
			}
			attachmentMeta.Kind = attachment.Kind
			attachmentMeta.ExtractedDir = extractedDir
			attachmentMeta.Notes = append(attachmentMeta.Notes, attachment.Notes...)
		default:
			return codex.Request{}, &UserVisibleError{Message: "Unsupported attachment kind in Lark message."}
		}

		if err := bundle.AddAttachment(attachmentMeta); err != nil {
			return codex.Request{}, err
		}
		request.Attachments = append(request.Attachments, attachment)
	}

	cleanupBundle = false
	return request, nil
}

func defaultUserMessage(userText string, attachments []AttachmentRef) string {
	userText = strings.TrimSpace(userText)
	if userText != "" {
		return userText
	}
	if len(attachments) == 0 {
		return ""
	}
	return "I attached files without a question. Please reply briefly that you can see the attached file or files, and ask me to send a more specific question that uses them. Do not analyze the files yet."
}

func AsUserVisibleError(err error) (*UserVisibleError, bool) {
	var target *UserVisibleError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
