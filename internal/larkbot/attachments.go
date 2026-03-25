package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/usererr"
)

func saveDirectAttachment(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1, userKey string) (string, error) {
	resource, err := downloadResourceFromEvent(ctx, apiClient, manager.RootDir(), event)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(resource.tempPath)
	}()

	result, err := manager.Import(attachments.ImportRequest{
		UserKey:      userKey,
		OriginalName: resource.originalName,
		MessageID:    resource.messageID,
		FileKey:      resource.fileKey,
		ResourceType: resource.resourceType,
		SourcePath:   resource.tempPath,
		ImportedAt:   resource.messageCreate,
	})
	if err != nil {
		return "", usererr.WrapLocalStorage("Agent couldn't save the attachment to the local library. Please retry.", err)
	}
	return buildSaveSummary("Saved", []attachments.SaveResult{*result}), nil
}

// downloadRecentAttachments powers /upload_N by walking backward from the
// current message and importing the caller's recent file/image messages.
func downloadRecentAttachments(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1, userKey string, count int) (string, codex.AttachmentContext, error) {
	refs, err := findRecentAttachmentMessages(ctx, apiClient, event, count)
	if err != nil {
		return "", codex.AttachmentContext{}, err
	}

	results := make([]attachments.SaveResult, 0, len(refs))
	for _, ref := range refs {
		resource, err := downloadMessageResourceToTemp(ctx, apiClient, manager.RootDir(), ref.messageID, ref.fileKey, ref.resourceType, ref.createdAt)
		if err != nil {
			return "", codex.AttachmentContext{}, err
		}
		result, importErr := manager.Import(attachments.ImportRequest{
			UserKey:      userKey,
			OriginalName: resource.originalName,
			MessageID:    resource.messageID,
			FileKey:      resource.fileKey,
			ResourceType: resource.resourceType,
			SourcePath:   resource.tempPath,
			ImportedAt:   resource.messageCreate,
		})
		_ = os.Remove(resource.tempPath)
		if importErr != nil {
			return "", codex.AttachmentContext{}, usererr.WrapLocalStorage("Agent couldn't save the downloaded attachments to the local library. Please retry.", importErr)
		}
		results = append(results, *result)
	}

	attachmentCtx, err := buildAttachmentContext(manager, userKey)
	if err != nil {
		return "", codex.AttachmentContext{}, err
	}
	return buildSaveSummary("Downloaded", results), attachmentCtx, nil
}

func buildAttachmentContext(manager *attachments.Manager, userKey string) (codex.AttachmentContext, error) {
	library, err := manager.Snapshot(userKey)
	if err != nil {
		return codex.AttachmentContext{}, usererr.WrapLocalStorage("Agent couldn't load your saved attachments from local storage. Please retry.", err)
	}
	items := library.Items
	if len(items) > promptAttachmentSummaryLimit {
		items = items[:promptAttachmentSummaryLimit]
	}
	ctxItems := make([]codex.AttachmentItem, 0, len(items))
	for _, item := range items {
		ctxItems = append(ctxItems, codex.AttachmentItem{
			Name:         item.Name,
			Type:         string(item.Type),
			SavedAt:      item.CreatedAt,
			OriginalName: item.OriginalName,
		})
	}
	return codex.AttachmentContext{
		RootDir: library.RootDir,
		Items:   ctxItems,
	}, nil
}

func buildSaveSummary(verb string, results []attachments.SaveResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %d item(s).", verb, len(results))
	if len(results) == 0 {
		sb.WriteString(" No matching attachments were found.")
		return sb.String()
	}
	sb.WriteString("\n")
	for _, result := range results {
		sb.WriteString("- ")
		sb.WriteString(result.Item.Name)
		if result.Item.Type != "" {
			sb.WriteString(" [")
			sb.WriteString(string(result.Item.Type))
			sb.WriteByte(']')
		}
		if result.Replaced {
			sb.WriteString(" replaced_existing")
		}
		if len(result.Evicted) > 0 {
			sb.WriteString(" evicted=")
			names := make([]string, 0, len(result.Evicted))
			for _, item := range result.Evicted {
				names = append(names, item.Name)
			}
			sb.WriteString(strings.Join(names, ","))
		}
		sb.WriteByte('\n')
	}
	return strings.TrimSpace(sb.String())
}

// findRecentAttachmentMessages pages backward through the same chat and keeps
// only older messages from the same sender and same thread, which matches the
// mental model of "upload the files I just sent before this question".
func findRecentAttachmentMessages(ctx context.Context, apiClient *lark.Client, event *larkim.P2MessageReceiveV1, count int) ([]attachmentRef, error) {
	if count <= 0 {
		return nil, nil
	}
	chatID := extractChatID(event)
	if chatID == "" {
		return nil, nil
	}
	senderIDs := extractSenderIDs(event)
	if len(senderIDs) == 0 {
		return nil, nil
	}
	currentMessageID := extractMessageID(event)
	currentCreateAt := extractEventCreateTime(event)
	pageToken := ""
	refs := make([]attachmentRef, 0, count)

	for page := 0; page < maxUploadCommandPages && len(refs) < count; page++ {
		req := larkim.NewListMessageReqBuilder().
			ContainerIdType("chat").
			ContainerId(chatID).
			EndTime(strconv.FormatInt(currentCreateAt.Unix(), 10)).
			SortType(larkim.SortTypeListMessageByCreateTimeDesc).
			PageSize(messagePageSize)
		if pageToken != "" {
			req.PageToken(pageToken)
		}

		resp, err := apiClient.Im.Message.List(ctx, req.Build())
		if err != nil {
			return nil, classifyFeishuOperationError(err, "Agent couldn't list recent Feishu messages for `/upload_n`. Please retry.")
		}
		if resp == nil {
			return nil, usererr.New(usererr.KindUnavailable, "Feishu returned an empty response while listing recent messages. Please retry.")
		}
		if !resp.Success() {
			return nil, classifyFeishuResponseError(feishuOpListRecentMessages, "Agent couldn't list recent Feishu messages for `/upload_n`. Please retry.", resp.Code, resp.Msg)
		}
		if resp.Data == nil || len(resp.Data.Items) == 0 {
			break
		}

		for _, item := range resp.Data.Items {
			ref, ok := matchAttachmentMessage(item, event, senderIDs, currentMessageID, currentCreateAt)
			if !ok {
				continue
			}
			refs = append(refs, *ref)
			if len(refs) >= count {
				break
			}
		}

		if resp.Data.HasMore == nil || !*resp.Data.HasMore || resp.Data.PageToken == nil || strings.TrimSpace(*resp.Data.PageToken) == "" {
			break
		}
		pageToken = strings.TrimSpace(*resp.Data.PageToken)
	}

	return refs, nil
}

func matchAttachmentMessage(item *larkim.Message, event *larkim.P2MessageReceiveV1, senderIDs map[string]struct{}, currentMessageID string, currentCreateAt time.Time) (*attachmentRef, bool) {
	if item == nil || !sameThread(item, event) || !sameSender(item, senderIDs) {
		return nil, false
	}
	messageID := trimPtr(item.MessageId)
	if messageID == "" || messageID == currentMessageID {
		return nil, false
	}
	msgType := trimPtr(item.MsgType)
	if msgType != "file" && msgType != "image" {
		return nil, false
	}
	itemCreateAt := parseMillis(trimPtr(item.CreateTime))
	if !itemCreateAt.IsZero() && itemCreateAt.After(currentCreateAt) {
		return nil, false
	}

	var fileKey string
	switch msgType {
	case "file":
		fileKey = extractFileKeyFromMessage(item)
	case "image":
		fileKey = extractImageKeyFromMessage(item)
	}
	if fileKey == "" {
		return nil, false
	}

	return &attachmentRef{
		messageID:    messageID,
		fileKey:      fileKey,
		resourceType: msgType,
		createdAt:    itemCreateAt,
	}, true
}

func downloadResourceFromEvent(ctx context.Context, apiClient *lark.Client, tempRoot string, event *larkim.P2MessageReceiveV1) (*downloadedResource, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.Content == nil {
		return nil, usererr.New(usererr.KindInvalidInput, "This message does not contain attachment content.")
	}
	raw := trimPtr(event.Event.Message.Content)
	messageID := extractMessageID(event)
	messageCreate := extractEventCreateTime(event)

	switch extractMessageType(event) {
	case "file":
		var payload larkim.MessageFile
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, usererr.Wrap(usererr.KindInvalidInput, "Agent couldn't parse the Feishu file payload. Please resend the file.", err)
		}
		if strings.TrimSpace(payload.FileKey) == "" {
			return nil, usererr.New(usererr.KindInvalidInput, "This Feishu file message is missing its file key. Please resend the file.")
		}
		return downloadMessageResourceToTemp(ctx, apiClient, tempRoot, messageID, payload.FileKey, "file", messageCreate)
	case "image":
		var payload larkim.MessageImage
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, usererr.Wrap(usererr.KindInvalidInput, "Agent couldn't parse the Feishu image payload. Please resend the image.", err)
		}
		if strings.TrimSpace(payload.ImageKey) == "" {
			return nil, usererr.New(usererr.KindInvalidInput, "This Feishu image message is missing its image key. Please resend the image.")
		}
		return downloadMessageResourceToTemp(ctx, apiClient, tempRoot, messageID, payload.ImageKey, "image", messageCreate)
	default:
		return nil, usererr.New(usererr.KindInvalidInput, "Unsupported direct attachment type. Send a file or an image.")
	}
}

func downloadMessageResourceToTemp(ctx context.Context, apiClient *lark.Client, tempRoot, messageID, fileKey, resourceType string, createdAt time.Time) (*downloadedResource, error) {
	resp, err := apiClient.Im.MessageResource.Get(ctx,
		larkim.NewGetMessageResourceReqBuilder().
			MessageId(messageID).
			FileKey(fileKey).
			Type(resourceType).
			Build())
	if err != nil {
		return nil, classifyFeishuOperationError(err, "Agent couldn't download the attachment from Feishu. Please retry or resend it.")
	}
	if resp != nil && !resp.Success() {
		return nil, classifyFeishuResponseError(feishuOpDownloadAttachment, "Agent couldn't download the attachment from Feishu. Please retry or resend it.", resp.Code, resp.Msg)
	}
	if resp == nil || resp.File == nil {
		return nil, usererr.New(usererr.KindUnavailable, "Feishu returned an empty attachment response. Please retry or resend it.")
	}

	fileName := strings.TrimSpace(resp.FileName)
	if fileName == "" && resourceType == "file" {
		fileName = fileKey + ".bin"
	}
	ext := filepath.Ext(fileName)
	if ext == "" {
		if resourceType == "image" {
			ext = ".png"
		} else {
			ext = ".bin"
		}
	}

	tempFile, err := os.CreateTemp(tempRoot, ".download-*"+ext)
	if err != nil {
		return nil, usererr.WrapLocalStorage("Agent couldn't create local temporary storage for the attachment. Please retry.", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, usererr.WrapLocalStorage("Agent couldn't prepare local temporary storage for the attachment. Please retry.", err)
	}
	if err := resp.WriteFile(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, usererr.WrapLocalStorage("Agent couldn't write the downloaded attachment to local temporary storage. Please retry.", err)
	}

	log.Printf("[larkbot] downloaded %s message_id=%s file_key=%s temp=%s", resourceType, messageID, fileKey, tempPath)
	return &downloadedResource{
		tempPath:      tempPath,
		originalName:  fileName,
		resourceType:  resourceType,
		messageID:     messageID,
		fileKey:       fileKey,
		messageCreate: createdAt,
	}, nil
}

func extractFileKeyFromMessage(item *larkim.Message) string {
	if item == nil || item.Body == nil || item.Body.Content == nil {
		return ""
	}

	var payload larkim.MessageFile
	if err := json.Unmarshal([]byte(strings.TrimSpace(*item.Body.Content)), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.FileKey)
}

func extractImageKeyFromMessage(item *larkim.Message) string {
	if item == nil || item.Body == nil || item.Body.Content == nil {
		return ""
	}

	var payload larkim.MessageImage
	if err := json.Unmarshal([]byte(strings.TrimSpace(*item.Body.Content)), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ImageKey)
}

func sameThread(item *larkim.Message, event *larkim.P2MessageReceiveV1) bool {
	currentThreadID := extractThreadID(event)
	itemThreadID := trimPtr(item.ThreadId)
	if currentThreadID != "" {
		return itemThreadID == currentThreadID
	}
	return itemThreadID == ""
}

func sameSender(item *larkim.Message, senderIDs map[string]struct{}) bool {
	if item == nil || item.Sender == nil || item.Sender.Id == nil {
		return false
	}
	idType := trimPtr(item.Sender.IdType)
	if idType != "" && idType != "open_id" {
		log.Printf("[larkbot] sameSender: unexpected sender id_type=%q message_id=%s, skipping", idType, trimPtr(item.MessageId))
		return false
	}
	_, ok := senderIDs[strings.TrimSpace(*item.Sender.Id)]
	return ok
}
