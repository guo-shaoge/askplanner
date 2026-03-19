package lark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

var publicIDPattern = regexp.MustCompile(`bundle-[a-f0-9]{8}`)

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
				return codex.Request{}, &UserVisibleError{Message: "The message is empty. Images and PLAN REPLAYER .zip files should be sent through Lark by themselves. After the upload reply returns an ID, send a follow-up text question that includes that ID."}
			}
			return i.buildTextRequestWithReferences(parsed.UserText)
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
			attachment.PublicID = bundle.RawPublicID
			attachment.Notes = append(attachment.Notes, "Image downloaded from the Lark message resource API.")
			attachmentMeta.PublicID = bundle.RawPublicID
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
			attachment.PublicID = bundle.ExtractedPublicID
			attachment.ExtractedDir = extractedDir
			attachment.Notes = append(attachment.Notes, "PLAN REPLAYER ZIP extracted for Codex inspection.")
			attachment.Notes = append(attachment.Notes, "Use the public ID in any user-facing reply. The public ID maps to the extracted bundle root, not the hidden .askplanner path.")
			if len(manifest.DetectedFiles) > 0 {
				attachment.Notes = append(attachment.Notes, "Detected PLAN REPLAYER files: "+strings.Join(manifest.DetectedFiles, ", "))
			}
			attachmentMeta.Kind = attachment.Kind
			attachmentMeta.PublicID = bundle.ExtractedPublicID
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
	request.UserMessage = defaultUploadUserMessage(request.Attachments)

	cleanupBundle = false
	return request, nil
}

func (i *Intake) buildTextRequestWithReferences(userText string) (codex.Request, error) {
	request := codex.NewTextRequest(userText)
	publicIDs := extractReferencedPublicIDs(userText)
	if len(publicIDs) == 0 {
		return request, nil
	}

	sort.Strings(publicIDs)
	for _, publicID := range publicIDs {
		resolved, err := i.Store.Resolve(publicID)
		if err != nil {
			return codex.Request{}, &UserVisibleError{Message: fmt.Sprintf("The attachment ID %q is invalid or expired. Please upload the image or PLAN REPLAYER .zip file again through Lark and use the new returned ID.", publicID)}
		}
		request.RuntimeNotes = append(request.RuntimeNotes,
			fmt.Sprintf("Resolved attachment ID: %s", publicID),
			fmt.Sprintf("Resolved internal attachment path: %s", resolved.Dir),
		)
		for _, meta := range resolved.Meta.Attachments {
			publicIDForAttachment := strings.TrimSpace(meta.PublicID)
			if publicIDForAttachment == "" {
				publicIDForAttachment = publicID
			}
			request.Attachments = append(request.Attachments, codex.Attachment{
				Kind:         strings.TrimSpace(meta.Kind),
				PublicID:     publicIDForAttachment,
				OriginalName: strings.TrimSpace(meta.OriginalName),
				SavedPath:    strings.TrimSpace(meta.SavedPath),
				ExtractedDir: strings.TrimSpace(meta.ExtractedDir),
				Notes: append([]string{
					"Attachment was resolved from the user-provided public ID. Use the public ID in any user-facing reply.",
				}, meta.Notes...),
			})
		}
	}
	return request, nil
}

func defaultUploadUserMessage(attachments []codex.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("The user uploaded image or PLAN REPLAYER ZIP files through Lark without a follow-up question.\n")
	sb.WriteString("Reply briefly that you can see the file or files, return the public ID for each one exactly as shown below, and tell the user that every later message that needs those files must include the ID.\n")
	sb.WriteString("Also tell the user that images and PLAN REPLAYER ZIP files should only be sent through Lark, and later text questions should reference the returned ID. Do not analyze the files yet.\n")
	sb.WriteString("Public IDs:\n")
	for _, attachment := range attachments {
		if publicID := strings.TrimSpace(attachment.PublicID); publicID != "" {
			fmt.Fprintf(&sb, "- %s (%s)\n", publicID, strings.TrimSpace(attachment.Kind))
		}
	}
	return strings.TrimSpace(sb.String())
}

func extractReferencedPublicIDs(userText string) []string {
	matches := publicIDPattern.FindAllString(userText, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, match)
	}
	return out
}

func AsUserVisibleError(err error) (*UserVisibleError, bool) {
	var target *UserVisibleError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
