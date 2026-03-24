package larkbot

import (
	"strings"

	"lab/askplanner/internal/usererr"
)

type feishuAPIOperation string

const (
	feishuOpListRecentMessages feishuAPIOperation = "list_recent_messages"
	feishuOpDownloadAttachment feishuAPIOperation = "download_attachment"
	feishuOpReplyMessage       feishuAPIOperation = "reply_message"
	feishuOpAddReaction        feishuAPIOperation = "add_reaction"
	feishuOpDeleteReaction     feishuAPIOperation = "delete_reaction"
)

func classifyFeishuOperationError(err error, message string) error {
	if msg := usererr.Message(err); msg != "" {
		return err
	}
	lower := strings.ToLower(err.Error())
	switch {
	case containsAny(lower, "rate limit", "too many requests", "429"):
		return usererr.Wrap(usererr.KindRateLimit, "Feishu is rate-limiting requests right now. Please retry in a moment.", err)
	case containsAny(lower, "timeout", "timed out", "deadline exceeded", "i/o timeout"):
		return usererr.Wrap(usererr.KindTimeout, "Feishu did not respond in time. Please retry.", err)
	case containsAny(lower, "dial tcp", "connection refused", "connection reset", "no such host", "network is unreachable", "temporary failure in name resolution"):
		return usererr.Wrap(usererr.KindNetwork, "Feishu could not be reached because of a network problem. Please retry.", err)
	default:
		return usererr.Wrap(usererr.KindUnavailable, message, err)
	}
}

func classifyFeishuResponseError(operation feishuAPIOperation, message string, code int, respMsg string) error {
	text := strings.ToLower(strings.TrimSpace(respMsg))
	switch {
	case code == 429 || containsAny(text, "rate limit", "too many requests"):
		return usererr.New(usererr.KindRateLimit, feishuRateLimitMessage(operation))
	case code == 401 || code == 403 || containsAny(text, "forbidden", "unauthorized", "permission denied", "no permission", "access denied", "tenant_access_token", "tenant access token"):
		return usererr.New(usererr.KindAuth, feishuPermissionMessage(operation))
	case containsAny(text, "message not exist", "message not found", "file not exist", "file not found", "resource not exist", "resource not found", "image not exist", "chat not exist", "has been recalled", "cannot find message"):
		return usererr.New(usererr.KindInvalidInput, feishuNotFoundMessage(operation))
	case operation == feishuOpReplyMessage && containsAny(text, "msg_type", "message type", "invalid content", "content invalid", "format", "content too long", "too long", "too large", "unsupported"):
		return usererr.New(usererr.KindInvalidInput, "Feishu rejected the rich reply format or content. Falling back to plain text may work.")
	case operation == feishuOpAddReaction && containsAny(text, "reaction type is invalid", "unsupported reaction", "not support reaction"):
		return usererr.New(usererr.KindInvalidInput, "Feishu rejected the typing reaction type. Continuing without it.")
	case containsAny(text, "invalid param", "invalid parameter", "bad request", "parameter error"):
		return usererr.New(usererr.KindInvalidInput, feishuInvalidRequestMessage(operation))
	case code >= 500 || containsAny(text, "internal error", "server error", "service unavailable", "gateway timeout", "system busy"):
		return usererr.New(usererr.KindUnavailable, feishuUnavailableMessage(operation, message))
	default:
		return usererr.New(usererr.KindUnavailable, message)
	}
}

func feishuRateLimitMessage(operation feishuAPIOperation) string {
	switch operation {
	case feishuOpReplyMessage:
		return "Feishu is rate-limiting bot replies right now. Please retry in a moment."
	case feishuOpListRecentMessages:
		return "Feishu is rate-limiting recent-message lookups right now. Please retry in a moment."
	case feishuOpDownloadAttachment:
		return "Feishu is rate-limiting attachment downloads right now. Please retry in a moment."
	default:
		return "Feishu is rate-limiting requests right now. Please retry in a moment."
	}
}

func feishuPermissionMessage(operation feishuAPIOperation) string {
	switch operation {
	case feishuOpListRecentMessages:
		return "The bot isn't allowed to read recent messages in this chat. Check Feishu chat history permissions."
	case feishuOpDownloadAttachment:
		return "The bot isn't allowed to download this attachment from Feishu. Check attachment permissions or resend it."
	case feishuOpReplyMessage:
		return "The bot isn't allowed to reply in this chat. Check the bot permissions."
	case feishuOpAddReaction, feishuOpDeleteReaction:
		return "The bot isn't allowed to manage typing reactions in this chat."
	default:
		return "The bot doesn't have permission to complete that Feishu action."
	}
}

func feishuNotFoundMessage(operation feishuAPIOperation) string {
	switch operation {
	case feishuOpListRecentMessages:
		return "The recent Feishu messages for `/upload_n` are no longer available. Please resend the original attachment messages."
	case feishuOpDownloadAttachment:
		return "The selected attachment is no longer available in Feishu. Please resend it."
	case feishuOpReplyMessage:
		return "The original Feishu message is no longer available for reply."
	case feishuOpAddReaction, feishuOpDeleteReaction:
		return "The original Feishu message is no longer available for typing reactions."
	default:
		return "The requested Feishu resource is no longer available."
	}
}

func feishuInvalidRequestMessage(operation feishuAPIOperation) string {
	switch operation {
	case feishuOpListRecentMessages:
		return "Feishu rejected the recent-message lookup request for `/upload_n`."
	case feishuOpDownloadAttachment:
		return "Feishu rejected the attachment download request. Please resend the attachment."
	case feishuOpReplyMessage:
		return "Feishu rejected the reply request because the content or parameters are invalid."
	case feishuOpAddReaction, feishuOpDeleteReaction:
		return "Feishu rejected the typing reaction request. Continuing without it."
	default:
		return "Feishu rejected the request because the parameters are invalid."
	}
}

func feishuUnavailableMessage(operation feishuAPIOperation, fallback string) string {
	switch operation {
	case feishuOpListRecentMessages:
		return "Feishu is temporarily unavailable for recent-message lookups. Please retry."
	case feishuOpDownloadAttachment:
		return "Feishu is temporarily unavailable for attachment downloads. Please retry."
	case feishuOpReplyMessage:
		return "Feishu is temporarily unavailable for sending replies. Please retry."
	case feishuOpAddReaction, feishuOpDeleteReaction:
		return "Feishu is temporarily unavailable for typing reactions. Continuing without them."
	default:
		return strings.TrimSpace(fallback)
	}
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func joinReplySections(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, "\n\n")
}

func formatWarning(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return "**Warning**\n" + message
}
