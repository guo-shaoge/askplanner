package larkbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/modelcmd"
	"lab/askplanner/internal/selfcmd"
	"lab/askplanner/internal/usererr"
	"lab/askplanner/internal/workspace"
)

// prepareReply turns a raw Feishu event into a normalized handler input.
// This is where message-type-specific behavior lives: plain questions,
// attachment-only messages, /upload_N, and local/workspace commands.
func prepareReply(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1, bot botIdentity) (*preparedReply, error) {
	userKey := buildScopedUserKey(event, bot)
	if userKey == "" {
		return nil, usererr.New(usererr.KindInvalidInput, "This Feishu message is missing sender information. Please resend it.")
	}
	conversationKey := buildConversationKey(event, bot)

	switch extractMessageType(event) {
	case "text", "post":
		reply, err := prepareTextLikeReply(ctx, apiClient, manager, event, userKey, conversationKey)
		if err != nil || reply == nil || reply.skipCodex {
			return reply, err
		}
		if apiClient != nil && extractThreadID(event) != "" {
			reply.threadCtxLoader = func(loaderCtx context.Context) (*codex.ThreadContext, error) {
				return maybeBuildThreadContext(loaderCtx, apiClient, event)
			}
		}
		return reply, nil
	case "file", "image":
		if isGroupChat(event) {
			return nil, usererr.New(usererr.KindInvalidInput, "Group attachments are not handled directly. Use `@bot /upload_<n> your question`.")
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
		return nil, usererr.New(usererr.KindInvalidInput, "Unsupported message type. Send text, rich text, file, or image.")
	}
}

func prepareTextLikeReply(ctx context.Context, apiClient *lark.Client, manager *attachments.Manager, event *larkim.P2MessageReceiveV1, userKey, conversationKey string) (*preparedReply, error) {
	text := extractQuestionText(event)
	command := parseUploadCommand(text)
	if command.matched && !command.ok {
		return nil, usererr.New(usererr.KindInvalidInput, "Invalid upload command. Use `/upload_<n> your question`, for example `/upload_3 analyze these files`.")
	}
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
	if modelCommand, matched, err := modelcmd.ParseCommand(text); matched {
		if err != nil {
			return nil, err
		}
		return &preparedReply{
			question:        strings.TrimSpace(modelCommand.Question),
			attachmentCtx:   attachmentCtx,
			modelCmd:        modelCommand,
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
	cmd := uploadCommand{matched: true}
	rest := strings.TrimPrefix(text, "/upload_")
	if rest == "" {
		return cmd
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return cmd
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n <= 0 {
		return cmd
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	return uploadCommand{
		count:     n,
		remainder: remainder,
		matched:   true,
		ok:        true,
	}
}
