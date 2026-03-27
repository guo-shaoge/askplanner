package larkbot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type textMessageContent struct {
	Text             *string `json:"text"`
	TextWithoutAtBot *string `json:"text_without_at_bot"`
}

// shouldHandleEvent applies the bot's product policy before we do any heavier
// work: supported message types, direct messages, and whether a group message
// is actually addressed to the bot.
func shouldHandleEvent(event *larkim.P2MessageReceiveV1, bot botIdentity) (bool, string) {
	msgType := extractMessageType(event)
	if msgType == "" {
		return false, "empty message type"
	}
	if !isGroupChat(event) {
		if msgType == "text" || msgType == "post" || msgType == "file" || msgType == "image" {
			return true, ""
		}
		return false, fmt.Sprintf("unsupported p2p message type=%s", msgType)
	}
	if msgType != "text" && msgType != "post" {
		return false, fmt.Sprintf("unsupported group message type=%s", msgType)
	}
	if isMessageDirectedToBot(event, bot) {
		return true, ""
	}
	return false, fmt.Sprintf("group message not addressed to bot mentions=%d bot_name_set=%t", len(extractMentionKeys(event)), bot.name != "")
}

func isGroupChat(event *larkim.P2MessageReceiveV1) bool {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.ChatType == nil {
		return false
	}
	switch strings.TrimSpace(*event.Event.Message.ChatType) {
	case "group", "topic_group":
		return true
	default:
		return false
	}
}

func isTextDirectedToBot(event *larkim.P2MessageReceiveV1, bot botIdentity) bool {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.Content == nil {
		return false
	}
	payload, ok := decodeTextMessageContent(*event.Event.Message.Content)
	if ok && payload.TextWithoutAtBot != nil {
		return true
	}
	return mentionsBot(event, bot)
}

func isMessageDirectedToBot(event *larkim.P2MessageReceiveV1, bot botIdentity) bool {
	switch extractMessageType(event) {
	case "text":
		return isTextDirectedToBot(event, bot)
	case "post":
		return mentionsBot(event, bot) || postMentionsBot(event, bot)
	default:
		return false
	}
}

func extractMessageID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.MessageId == nil {
		return ""
	}
	return *event.Event.Message.MessageId
}

func extractMessageType(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.MessageType == nil {
		return ""
	}
	return strings.TrimSpace(*event.Event.Message.MessageType)
}

func decodeTextMessageContent(raw string) (textMessageContent, bool) {
	var payload textMessageContent
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return textMessageContent{}, false
	}
	return payload, true
}

func decodePostMessageContent(raw string) (incomingPostMessageContent, bool) {
	var payload incomingPostMessageContent
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return incomingPostMessageContent{}, false
	}
	return payload, true
}

// extractQuestionText returns the user-visible question text regardless of
// whether the source message was plain text or rich-text post content.
func extractQuestionText(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.Content == nil {
		return ""
	}
	raw := trimPtr(event.Event.Message.Content)
	switch extractMessageType(event) {
	case "post":
		return extractPostMessage(raw)
	default:
		return extractTextMessage(event, raw)
	}
}

func extractTextMessage(event *larkim.P2MessageReceiveV1, raw string) string {
	payload, ok := decodeTextMessageContent(raw)
	if !ok {
		return raw
	}
	if payload.TextWithoutAtBot != nil {
		return strings.TrimSpace(*payload.TextWithoutAtBot)
	}
	if payload.Text != nil {
		return stripMentionKeys(strings.TrimSpace(*payload.Text), extractMentionKeys(event))
	}
	return raw
}

func extractPostMessage(raw string) string {
	payload, ok := decodePostMessageContent(raw)
	if !ok {
		return raw
	}
	locale := payload.firstLocale()
	if locale == nil {
		return raw
	}

	lines := make([]string, 0, len(locale.Content)+1)
	if title := strings.TrimSpace(locale.Title); title != "" {
		lines = append(lines, title)
	}
	for _, row := range locale.Content {
		var line strings.Builder
		for _, node := range row {
			line.WriteString(extractPostNodeText(node))
		}
		lines = append(lines, strings.TrimSpace(normalizeInlineWhitespace(line.String())))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (c incomingPostMessageContent) firstLocale() *incomingPostLocale {
	locales := []*incomingPostLocale{c.ZhCN, c.EnUS, c.JaJP}
	for _, locale := range locales {
		if locale == nil {
			continue
		}
		if strings.TrimSpace(locale.Title) != "" || len(locale.Content) > 0 {
			return locale
		}
	}
	return nil
}

func extractPostNodeText(node incomingPostNode) string {
	switch node.Tag {
	case "text":
		return node.Text
	case "a":
		text := strings.TrimSpace(node.Text)
		href := strings.TrimSpace(node.Href)
		switch {
		case text != "" && href != "":
			return fmt.Sprintf("[%s](%s)", text, href)
		case text != "":
			return text
		default:
			return href
		}
	case "at":
		name := strings.TrimSpace(node.UserName)
		if name == "" {
			name = strings.TrimSpace(node.Name)
		}
		if name == "" {
			return "@mention"
		}
		return "@" + name
	case "img":
		return "[image]"
	case "media":
		return "[media]"
	default:
		if text := strings.TrimSpace(node.Text); text != "" {
			return node.Text
		}
		if name := strings.TrimSpace(node.UserName); name != "" {
			return "@" + name
		}
		if name := strings.TrimSpace(node.Name); name != "" {
			return "@" + name
		}
		return ""
	}
}

func extractMentionKeys(event *larkim.P2MessageReceiveV1) []string {
	if event == nil || event.Event == nil || event.Event.Message == nil || len(event.Event.Message.Mentions) == 0 {
		return nil
	}

	keys := make([]string, 0, len(event.Event.Message.Mentions))
	for _, mention := range event.Event.Message.Mentions {
		if mention == nil || mention.Key == nil {
			continue
		}
		key := strings.TrimSpace(*mention.Key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func mentionsBot(event *larkim.P2MessageReceiveV1, bot botIdentity) bool {
	if strings.TrimSpace(bot.name) == "" || event == nil || event.Event == nil || event.Event.Message == nil {
		return false
	}
	for _, mention := range event.Event.Message.Mentions {
		if mention == nil || mention.Name == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(*mention.Name), bot.name) {
			return true
		}
	}
	return false
}

func postMentionsBot(event *larkim.P2MessageReceiveV1, bot botIdentity) bool {
	if strings.TrimSpace(bot.name) == "" || event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.Content == nil {
		return false
	}
	if extractMessageType(event) != "post" {
		return false
	}
	payload, ok := decodePostMessageContent(trimPtr(event.Event.Message.Content))
	if !ok {
		return false
	}
	target := strings.TrimSpace(bot.name)
	for _, locale := range []*incomingPostLocale{payload.ZhCN, payload.EnUS, payload.JaJP} {
		if locale == nil {
			continue
		}
		for _, row := range locale.Content {
			for _, node := range row {
				if node.Tag != "at" {
					continue
				}
				name := strings.TrimSpace(node.UserName)
				if name == "" {
					name = strings.TrimSpace(node.Name)
				}
				if name != "" && strings.EqualFold(name, target) {
					return true
				}
			}
		}
	}
	return false
}

func stripMentionKeys(text string, keys []string) string {
	text = strings.TrimSpace(text)
	if text == "" || len(keys) == 0 {
		return text
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		text = strings.ReplaceAll(text, key, "")
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(normalizeInlineWhitespace(line))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeInlineWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\v', '\f', '\u00a0':
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}

func parseMillis(s string) time.Time {
	if strings.TrimSpace(s) == "" {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func trimPtr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func sanitizePathSegment(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}

	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return fallback
	}
	return out
}

func extractSenderIDs(event *larkim.P2MessageReceiveV1) map[string]struct{} {
	out := make(map[string]struct{})
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return out
	}
	if v := trimUserID(event.Event.Sender.SenderId.OpenId); v != "" {
		out[v] = struct{}{}
	}
	if v := trimUserID(event.Event.Sender.SenderId.UserId); v != "" {
		out[v] = struct{}{}
	}
	if v := trimUserID(event.Event.Sender.SenderId.UnionId); v != "" {
		out[v] = struct{}{}
	}
	return out
}

func extractPreferredSenderID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	if v := trimUserID(event.Event.Sender.SenderId.OpenId); v != "" {
		return v
	}
	if v := trimUserID(event.Event.Sender.SenderId.UserId); v != "" {
		return v
	}
	return ""
}

func buildScopedUserKey(event *larkim.P2MessageReceiveV1, bot botIdentity) string {
	raw := sanitizePathSegment(extractPreferredSenderID(event), "")
	botKey := sanitizePathSegment(bot.key, "")
	switch {
	case botKey != "" && raw != "":
		return "larkbot:" + botKey + ":" + raw
	case raw != "":
		return raw
	default:
		return ""
	}
}

func trimUserID(s *string) string {
	return trimPtr(s)
}

func extractChatID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return ""
	}
	return trimPtr(event.Event.Message.ChatId)
}

func extractRootID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return ""
	}
	return trimPtr(event.Event.Message.RootId)
}

func extractThreadID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return ""
	}
	return trimPtr(event.Event.Message.ThreadId)
}

func extractEventCreateTime(event *larkim.P2MessageReceiveV1) time.Time {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return time.Now()
	}
	t := parseMillis(trimPtr(event.Event.Message.CreateTime))
	if t.IsZero() {
		return time.Now()
	}
	return t
}

// buildConversationKey keeps one Codex conversation per user/root-message in
// groups so the initial channel mention and later thread follow-ups share the
// same context, while still isolating different users from each other.
func buildConversationKey(event *larkim.P2MessageReceiveV1, bot botIdentity) string {
	if event == nil || event.Event == nil {
		return "lark:unknown"
	}

	rootID := extractRootID(event)
	threadID := extractThreadID(event)
	chatID := extractChatID(event)
	senderID := sanitizePathSegment(buildScopedUserKey(event, bot), "")
	messageID := extractMessageID(event)
	prefix := "lark"
	if botKey := sanitizePathSegment(bot.key, ""); botKey != "" {
		prefix = "larkbot:" + botKey
	}

	if isGroupChat(event) {
		anchorID := sanitizePathSegment(rootID, "")
		if anchorID == "" && threadID == "" {
			anchorID = sanitizePathSegment(messageID, "")
		}
		switch {
		case anchorID != "" && senderID != "":
			return prefix + ":root:" + anchorID + ":user:" + senderID
		case anchorID != "":
			return prefix + ":root:" + anchorID
		}
	}

	switch {
	case threadID != "" && senderID != "":
		return prefix + ":thread:" + threadID + ":user:" + senderID
	case chatID != "" && senderID != "":
		return prefix + ":chat:" + chatID + ":user:" + senderID
	case threadID != "":
		return prefix + ":thread:" + threadID
	case chatID != "":
		return prefix + ":chat:" + chatID
	case messageID != "":
		return prefix + ":message:" + messageID
	default:
		return prefix + ":unknown"
	}
}
