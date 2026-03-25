package larkbot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"lab/askplanner/internal/codex"
)

func maybeBuildThreadContext(ctx context.Context, apiClient *lark.Client, event *larkim.P2MessageReceiveV1) (*codex.ThreadContext, error) {
	threadID := extractThreadID(event)
	if apiClient == nil || threadID == "" || event == nil || event.Event == nil || event.Event.Message == nil {
		return nil, nil
	}

	currentMessageID := extractMessageID(event)
	currentCreateAt := extractEventCreateTime(event)
	pageToken := ""
	collected := make([]*larkim.Message, 0, promptThreadMessageLimit)
	fallbackRecent := make([]*larkim.Message, 0, promptThreadMessageLimit)
	rootMessageID := trimPtr(event.Event.Message.RootId)
	var rootMessage *larkim.Message
	foundCurrent := false
	omittedCount := 0
	fallbackOmittedCount := 0

	for page := 0; page < maxThreadContextPages; page++ {
		// Pull newest-first so long threads usually resolve in one page around the
		// current message instead of walking from the root.
		req := larkim.NewListMessageReqBuilder().
			ContainerIdType("thread").
			ContainerId(threadID).
			SortType(larkim.SortTypeListMessageByCreateTimeDesc).
			PageSize(messagePageSize)
		if pageToken != "" {
			req.PageToken(pageToken)
		}

		resp, err := apiClient.Im.Message.List(ctx, req.Build())
		if err != nil {
			return nil, fmt.Errorf("list thread messages: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("list thread messages: empty response")
		}
		if !resp.Success() {
			return nil, fmt.Errorf("list thread messages failed: code=%d, msg=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil || len(resp.Data.Items) == 0 {
			break
		}

		collectFromHere := foundCurrent
		for _, item := range resp.Data.Items {
			if item == nil {
				continue
			}
			// Feishu thread listing has no "created before current event" bound, so
			// clamp client-side to avoid leaking replies that were posted after this
			// inbound message but reached the bot first due to delivery lag.
			if isThreadMessageAfter(currentCreateAt, item) {
				continue
			}
			messageID := trimPtr(item.MessageId)
			if rootMessage == nil && rootMessageID != "" && messageID == rootMessageID {
				rootMessage = item
			}
			if !collectFromHere {
				if messageID == currentMessageID {
					foundCurrent = true
					collectFromHere = true
					continue
				}
				if len(fallbackRecent) >= promptThreadMessageLimit {
					fallbackOmittedCount++
				} else if !containsThreadMessage(fallbackRecent, messageID) {
					fallbackRecent = append(fallbackRecent, item)
				}
				continue
			}
			if messageID == currentMessageID {
				continue
			}
			if len(collected) >= promptThreadMessageLimit {
				omittedCount++
				continue
			}
			if !containsThreadMessage(collected, messageID) {
				collected = append(collected, item)
			}
		}

		hasMore := resp.Data.HasMore != nil && *resp.Data.HasMore && resp.Data.PageToken != nil && strings.TrimSpace(*resp.Data.PageToken) != ""
		if foundCurrent && len(collected) >= promptThreadMessageLimit {
			if hasMore {
				// There are older pages we intentionally did not walk after the window
				// filled up, so OmittedCount becomes a lower bound.
				omittedCount++
			}
			break
		}
		if !hasMore {
			break
		}
		pageToken = strings.TrimSpace(*resp.Data.PageToken)
	}
	if !foundCurrent && len(fallbackRecent) > 0 {
		log.Printf("[larkbot] current message not yet visible in thread list; falling back to latest history thread=%s current=%s",
			threadID, currentMessageID)
		collected = fallbackRecent
		omittedCount = fallbackOmittedCount
	}

	// Keep the fast path focused on nearby history; fetch the root separately
	// only when the event told us there is one and it was not in the scanned page.
	if rootMessage == nil && rootMessageID != "" && rootMessageID != currentMessageID {
		item, err := getThreadMessage(ctx, apiClient, rootMessageID)
		if err != nil {
			log.Printf("[larkbot] root message fetch failed thread=%s root=%s: %v", threadID, rootMessageID, err)
		}
		rootMessage = item
	}

	if rootMessage != nil && trimPtr(rootMessage.MessageId) != "" && trimPtr(rootMessage.MessageId) != currentMessageID &&
		!containsThreadMessage(collected, trimPtr(rootMessage.MessageId)) {
		collected = append(collected, rootMessage)
	}

	// The prompt renders history oldest-first even though we fetch newest-first.
	// Preserve Feishu's relative order for same-millisecond messages instead of
	// inventing one from message_id, which is only documented as unique.
	sort.SliceStable(collected, func(i, j int) bool {
		leftAt := parseMillis(trimPtr(collected[i].CreateTime))
		rightAt := parseMillis(trimPtr(collected[j].CreateTime))
		if leftAt.IsZero() {
			return !rightAt.IsZero()
		}
		if rightAt.IsZero() {
			return false
		}
		return leftAt.Before(rightAt)
	})

	messages := make([]codex.ThreadMessage, 0, len(collected))
	for _, item := range collected {
		messages = append(messages, codex.ThreadMessage{
			MessageID:       trimPtr(item.MessageId),
			RootMessageID:   trimPtr(item.RootId),
			ParentMessageID: trimPtr(item.ParentId),
			SenderLabel:     formatThreadSenderLabel(item.Sender),
			MessageType:     trimPtr(item.MsgType),
			CreatedAt:       parseMillis(trimPtr(item.CreateTime)),
			Content:         extractThreadMessageContent(item),
		})
	}
	if len(messages) > promptThreadMessageLimit {
		overflow := len(messages) - promptThreadMessageLimit
		omittedCount += overflow
		if rootMessageID != "" && len(messages) > 0 && messages[0].MessageID == rootMessageID && promptThreadMessageLimit > 1 {
			messages = append([]codex.ThreadMessage{messages[0]}, messages[len(messages)-(promptThreadMessageLimit-1):]...)
		} else {
			messages = messages[len(messages)-promptThreadMessageLimit:]
		}
	}

	if rootMessageID == "" {
		if len(messages) > 0 {
			// Some transports do not expose a stable thread root identifier. In that
			// case we fall back to the oldest message in the local prompt window as a
			// relative anchor; it is not guaranteed to be the true thread root.
			rootMessageID = messages[0].MessageID
		} else if trimPtr(event.Event.Message.ParentId) == "" {
			rootMessageID = currentMessageID
		}
	}

	return &codex.ThreadContext{
		ThreadID:        threadID,
		RootMessageID:   rootMessageID,
		ParentMessageID: trimPtr(event.Event.Message.ParentId),
		OmittedCount:    omittedCount,
		Messages:        messages,
	}, nil
}

func getThreadMessage(ctx context.Context, apiClient *lark.Client, messageID string) (*larkim.Message, error) {
	resp, err := apiClient.Im.Message.Get(ctx, larkim.NewGetMessageReqBuilder().
		MessageId(messageID).
		Build())
	if err != nil {
		return nil, fmt.Errorf("get thread message %s: %w", messageID, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("get thread message %s: empty response", messageID)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("get thread message %s failed: code=%d, msg=%s", messageID, resp.Code, resp.Msg)
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 || resp.Data.Items[0] == nil {
		return nil, fmt.Errorf("get thread message %s: empty items", messageID)
	}
	return resp.Data.Items[0], nil
}

func isThreadMessageAfter(currentCreateAt time.Time, item *larkim.Message) bool {
	if currentCreateAt.IsZero() || item == nil {
		return false
	}
	itemCreateAt := parseMillis(trimPtr(item.CreateTime))
	if itemCreateAt.IsZero() {
		return false
	}
	return itemCreateAt.After(currentCreateAt)
}

func containsThreadMessage(items []*larkim.Message, messageID string) bool {
	for _, item := range items {
		if trimPtr(item.MessageId) == messageID {
			return true
		}
	}
	return false
}

func formatThreadSenderLabel(sender *larkim.Sender) string {
	if sender == nil {
		return "unknown"
	}
	senderType := trimPtr(sender.SenderType)
	senderID := trimPtr(sender.Id)
	switch {
	case senderType != "" && senderID != "":
		return senderType + ":" + senderID
	case senderType != "":
		return senderType
	case senderID != "":
		return senderID
	default:
		return "unknown"
	}
}

func extractThreadMessageContent(item *larkim.Message) string {
	if item == nil {
		return ""
	}
	if item.Deleted != nil && *item.Deleted {
		return "[deleted message]"
	}

	msgType := trimPtr(item.MsgType)
	raw := ""
	if item.Body != nil && item.Body.Content != nil {
		raw = strings.TrimSpace(*item.Body.Content)
	}

	switch msgType {
	case "text":
		payload, ok := decodeTextMessageContent(raw)
		switch {
		case ok && payload.TextWithoutAtBot != nil:
			return compactThreadContent(rewriteMentionKeys(*payload.TextWithoutAtBot, item.Mentions))
		case ok && payload.Text != nil:
			return compactThreadContent(rewriteMentionKeys(*payload.Text, item.Mentions))
		default:
			return compactThreadContent(raw)
		}
	case "post":
		return compactThreadContent(extractPostMessage(raw))
	case "file":
		return "[file]"
	case "image":
		return "[image]"
	case "audio":
		return "[audio]"
	case "media":
		return "[media]"
	case "sticker":
		return "[sticker]"
	case "interactive":
		return "[interactive card]"
	case "share_chat":
		return "[shared chat]"
	case "share_user":
		return "[shared user]"
	default:
		if raw == "" {
			if msgType == "" {
				return "[unknown message]"
			}
			return "[" + msgType + "]"
		}
		return compactThreadContent(raw)
	}
}

func rewriteMentionKeys(text string, mentions []*larkim.Mention) string {
	text = strings.TrimSpace(text)
	if text == "" || len(mentions) == 0 {
		return text
	}
	ordered := append([]*larkim.Mention(nil), mentions...)
	// Replace longer keys first so @_user_1 does not partially rewrite @_user_10.
	sort.SliceStable(ordered, func(i, j int) bool {
		return mentionKeyLen(ordered[i]) > mentionKeyLen(ordered[j])
	})
	for _, mention := range ordered {
		if mention == nil {
			continue
		}
		key := trimPtr(mention.Key)
		if key == "" {
			continue
		}
		name := trimPtr(mention.Name)
		if name == "" {
			continue
		}
		text = strings.ReplaceAll(text, key, "@"+name)
	}
	return text
}

func mentionKeyLen(mention *larkim.Mention) int {
	if mention == nil {
		return 0
	}
	return len(trimPtr(mention.Key))
}

func compactThreadContent(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	normalized := make([]string, 0, len(lines))
	prevEmpty := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r\f\v\u00a0")
		// Collapse empty runs to a single blank line so pasted thread text does not
		// waste prompt budget on vertical whitespace.
		if strings.TrimSpace(line) == "" {
			if prevEmpty {
				continue
			}
			prevEmpty = true
			normalized = append(normalized, "")
			continue
		}
		prevEmpty = false
		normalized = append(normalized, line)
	}
	for len(normalized) > 0 && normalized[0] == "" {
		normalized = normalized[1:]
	}
	for len(normalized) > 0 && normalized[len(normalized)-1] == "" {
		normalized = normalized[:len(normalized)-1]
	}
	s = strings.Join(normalized, "\n")
	runes := []rune(s)
	if len(runes) <= promptThreadContentLimit {
		return s
	}
	return string(runes[:promptThreadContentLimit]) + "..."
}
