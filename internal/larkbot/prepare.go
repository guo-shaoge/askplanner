package larkbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/selfcmd"
	"lab/askplanner/internal/workspace"
)

// prepareReply turns a raw Feishu event into a normalized handler input.
// This is where message-type-specific behavior lives: plain questions,
// attachment-only messages, /upload_N, and local/workspace commands.
func prepareReply(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1) (*preparedReply, error) {
	userKey := extractPreferredSenderID(event)
	if userKey == "" {
		return nil, fmt.Errorf("missing sender id")
	}
	conversationKey := buildConversationKey(event)

	switch extractMessageType(event) {
	case "text", "post":
		return prepareTextLikeReply(ctx, apiClient, manager, event, userKey, conversationKey)
	case "file", "image":
		if isGroupChat(event) {
			return nil, fmt.Errorf("group %s messages should not reach prepareReply", extractMessageType(event))
		}
		summary, err := saveDirectAttachment(ctx, apiClient, manager, event, userKey)
		if err != nil {
			return nil, err
		}
		return &preparedReply{
			directReply:     summary,
			skipCodex:       true,
			conversationKey: conversationKey,
			userKey:         userKey,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %s", extractMessageType(event))
	}
}

func prepareTextLikeReply(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1, userKey, conversationKey string) (*preparedReply, error) {
	text := extractQuestionText(event)
	command := parseUploadCommand(text)
	if command.ok {
		if command.count > manager.MaxItems() {
			command.count = manager.MaxItems()
		}
		summary, attachmentCtx, err := downloadRecentAttachments(ctx, apiClient, manager, event, userKey, command.count)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(command.remainder) == "" {
			return &preparedReply{
				directReply:     summary,
				skipCodex:       true,
				conversationKey: conversationKey,
				userKey:         userKey,
			}, nil
		}
		return &preparedReply{
			question:        command.remainder,
			prefix:          summary,
			attachmentCtx:   attachmentCtx,
			conversationKey: conversationKey,
			userKey:         userKey,
		}, nil
	}

	attachmentCtx, err := buildAttachmentContext(manager, userKey)
	if err != nil {
		return nil, err
	}
	if selfcmd.IsWhoAmI(text) {
		return &preparedReply{
			directReply:     buildWhoAmIReply(userKey, conversationKey),
			skipCodex:       true,
			conversationKey: conversationKey,
			userKey:         userKey,
		}, nil
	}
	if wsCmd, matched, err := workspace.ParseCommand(text); matched {
		if err != nil {
			return nil, err
		}
		return &preparedReply{
			question:        strings.TrimSpace(wsCmd.Question),
			attachmentCtx:   attachmentCtx,
			workspaceCmd:    wsCmd,
			conversationKey: conversationKey,
			userKey:         userKey,
		}, nil
	}
	return &preparedReply{
		question:        text,
		attachmentCtx:   attachmentCtx,
		conversationKey: conversationKey,
		userKey:         userKey,
	}, nil
}

func buildWhoAmIReply(userKey, conversationKey string) string {
	return strings.TrimSpace(fmt.Sprintf(
		"User Key: %s\nConversation Key: %s",
		strings.TrimSpace(userKey),
		strings.TrimSpace(conversationKey),
	))
}

// parseUploadCommand recognizes `/upload_N` with an optional trailing question.
func parseUploadCommand(text string) uploadCommand {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/upload_") {
		return uploadCommand{}
	}
	rest := strings.TrimPrefix(text, "/upload_")
	if rest == "" {
		return uploadCommand{}
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return uploadCommand{}
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n <= 0 {
		return uploadCommand{}
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	return uploadCommand{
		count:     n,
		remainder: remainder,
		ok:        true,
	}
}
