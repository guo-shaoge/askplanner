package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// buildReplyBody always renders a post+markdown payload so the bot can send
// code fences, links, and richer formatting without switching message types.
func buildReplyBody(text string) (replyBody, error) {
	text = normalizeReplyText(text)

	payload := postMessageContent{
		ZhCN: postLocale{
			Content: [][]postMDNode{{
				{
					Tag:  "md",
					Text: text,
				},
			}},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return replyBody{}, err
	}
	return replyBody{
		msgType:      "post",
		content:      string(b),
		fallbackText: text,
	}, nil
}

func buildTextReplyBody(text string) (replyBody, error) {
	text = normalizeReplyText(text)
	payload := map[string]string{
		"text": text,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return replyBody{}, err
	}
	return replyBody{
		msgType:      "text",
		content:      string(b),
		fallbackText: text,
	}, nil
}

func normalizeReplyText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return " "
	}
	return text
}

// withTypingReaction best-effort wraps slow replies with a temporary Typing
// reaction. Reply execution must still proceed even if reaction APIs fail.
func withTypingReaction(ctx context.Context, apiClient *lark.Client, messageID string, run func() error) error {
	reactionID, err := addTypingReaction(ctx, apiClient, messageID)
	if err != nil {
		log.Printf("[larkbot] add typing reaction failed: %v (message_id=%s)", err, messageID)
		return run()
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), feishuReactionTimeout)
		defer cancel()
		if err := deleteMessageReaction(cleanupCtx, apiClient, messageID, reactionID); err != nil {
			log.Printf("[larkbot] delete typing reaction failed: %v (message_id=%s, reaction_id=%s)", err, messageID, reactionID)
		}
	}()

	return run()
}

func addTypingReaction(ctx context.Context, apiClient *lark.Client, messageID string) (string, error) {
	reactionCtx, cancel := context.WithTimeout(ctx, feishuReactionTimeout)
	defer cancel()

	resp, err := apiClient.Im.MessageReaction.Create(reactionCtx,
		larkim.NewCreateMessageReactionReqBuilder().
			MessageId(messageID).
			Body(larkim.NewCreateMessageReactionReqBodyBuilder().
				ReactionType(larkim.NewEmojiBuilder().
					EmojiType(typingReactionType).
					Build()).
				Build()).
			Build())
	if err != nil {
		return "", classifyFeishuOperationError(err, "Agent couldn't add the typing reaction in Feishu. Continuing without it.")
	}
	if !resp.Success() {
		return "", classifyFeishuResponseError(feishuOpAddReaction, "Agent couldn't add the typing reaction in Feishu. Continuing without it.", resp.Code, resp.Msg)
	}

	reactionID := ""
	if resp.Data != nil {
		reactionID = trimPtr(resp.Data.ReactionId)
	}
	if reactionID == "" {
		return "", fmt.Errorf("create reaction API returned empty reaction_id")
	}

	log.Printf("[larkbot] typing reaction added (message_id=%s, reaction_id=%s)", messageID, reactionID)
	return reactionID, nil
}

func deleteMessageReaction(ctx context.Context, apiClient *lark.Client, messageID, reactionID string) error {
	resp, err := apiClient.Im.MessageReaction.Delete(ctx,
		larkim.NewDeleteMessageReactionReqBuilder().
			MessageId(messageID).
			ReactionId(reactionID).
			Build())
	if err != nil {
		return classifyFeishuOperationError(err, "Agent couldn't remove the typing reaction in Feishu.")
	}
	if !resp.Success() {
		return classifyFeishuResponseError(feishuOpDeleteReaction, "Agent couldn't remove the typing reaction in Feishu.", resp.Code, resp.Msg)
	}

	log.Printf("[larkbot] typing reaction deleted (message_id=%s, reaction_id=%s)", messageID, reactionID)
	return nil
}

func replyMessage(ctx context.Context, apiClient *lark.Client, messageID string, body replyBody) error {
	if err := replyMessageOnce(ctx, apiClient, messageID, body, "reply-"+messageID+"-"+body.msgType); err != nil {
		if body.msgType == "post" && strings.TrimSpace(body.fallbackText) != "" && shouldFallbackToTextReply(err) {
			log.Printf("[larkbot] rich post reply failed, falling back to text: %v (message_id=%s)", err, messageID)
			fallback, buildErr := buildTextReplyBody(body.fallbackText)
			if buildErr != nil {
				return fmt.Errorf("build text fallback reply: %w", buildErr)
			}
			if fallbackErr := replyMessageOnce(ctx, apiClient, messageID, fallback, "reply-"+messageID+"-text"); fallbackErr == nil {
				return nil
			} else {
				return fmt.Errorf("reply fallback after rich post failure: %w", fallbackErr)
			}
		}
		return err
	}
	return nil
}

func replyMessageOnce(ctx context.Context, apiClient *lark.Client, messageID string, body replyBody, uuid string) error {
	log.Printf("[larkbot] replying to message_id=%s", messageID)
	resp, err := apiClient.Im.Message.Reply(ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType(body.msgType).
				Content(body.content).
				Uuid(uuid).
				Build()).
			Build())
	if err != nil {
		return classifyFeishuOperationError(err, "Agent couldn't send the reply to Feishu.")
	}
	if !resp.Success() {
		return classifyFeishuResponseError(feishuOpReplyMessage, "Agent couldn't send the reply to Feishu.", resp.Code, resp.Msg)
	}
	log.Printf("[larkbot] reply sent (message_id=%s)", messageID)
	return nil
}

func shouldFallbackToTextReply(err error) bool {
	lower := strings.ToLower(err.Error())
	if containsAny(lower, "rate limit", "too many requests", "429", "forbidden", "unauthorized", "permission", "network", "dial tcp", "timeout", "timed out") {
		return false
	}
	return containsAny(lower, "msg_type", "message type", "invalid param", "invalid parameter", "invalid content", "content invalid", "unsupported", "not support", "format", "rich reply format", "reply content")
}
