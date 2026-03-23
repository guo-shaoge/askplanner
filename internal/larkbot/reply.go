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

func buildReplyBody(text string) (replyBody, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		text = " "
	}

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
		msgType: "post",
		content: string(b),
	}, nil
}

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

	resp, err := apiClient.Im.V1.MessageReaction.Create(reactionCtx,
		larkim.NewCreateMessageReactionReqBuilder().
			MessageId(messageID).
			Body(larkim.NewCreateMessageReactionReqBodyBuilder().
				ReactionType(larkim.NewEmojiBuilder().
					EmojiType(typingReactionType).
					Build()).
				Build()).
			Build())
	if err != nil {
		return "", fmt.Errorf("call create reaction API: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("create reaction API error: code=%d, msg=%s", resp.Code, resp.Msg)
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
	resp, err := apiClient.Im.V1.MessageReaction.Delete(ctx,
		larkim.NewDeleteMessageReactionReqBuilder().
			MessageId(messageID).
			ReactionId(reactionID).
			Build())
	if err != nil {
		return fmt.Errorf("call delete reaction API: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("delete reaction API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	log.Printf("[larkbot] typing reaction deleted (message_id=%s, reaction_id=%s)", messageID, reactionID)
	return nil
}

func replyMessage(ctx context.Context, apiClient *lark.Client, messageID string, body replyBody) error {
	log.Printf("[larkbot] replying to message_id=%s", messageID)
	resp, err := apiClient.Im.V1.Message.Reply(ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType(body.msgType).
				Content(body.content).
				Uuid("reply-"+messageID).
				Build()).
			Build())
	if err != nil {
		return fmt.Errorf("call reply API: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("reply API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	log.Printf("[larkbot] reply sent (message_id=%s)", messageID)
	return nil
}
