package larkbot

import (
	"context"
	"sync"
	"time"

	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/workspace"
)

const (
	messagePageSize              = 50
	maxUploadCommandPages        = 20
	maxThreadContextPages        = 20
	promptAttachmentSummaryLimit = 20
	promptThreadMessageLimit     = 20
	promptThreadContentLimit     = 600
	typingReactionType           = "Typing"
	feishuReactionTimeout        = 10 * time.Second
)

type botIdentity struct {
	name string
}

type messageDedup struct {
	seen sync.Map // messageId -> time.Time
}

func (d *messageDedup) isDuplicate(messageID string) bool {
	_, loaded := d.seen.LoadOrStore(messageID, time.Now())
	return loaded
}

func (d *messageDedup) cleanup(maxAge time.Duration) {
	now := time.Now()
	d.seen.Range(func(key, value any) bool {
		if now.Sub(value.(time.Time)) > maxAge {
			d.seen.Delete(key)
		}
		return true
	})
}

type preparedReply struct {
	// The handler first normalizes a raw Feishu event into this shape so the
	// rest of the pipeline does not need to branch on message type again.
	question        string
	prefix          string
	directReply     string
	skipCodex       bool
	attachmentCtx   codex.AttachmentContext
	threadCtx       *codex.ThreadContext
	threadCtxLoader func(context.Context) (*codex.ThreadContext, error)
	workspaceCmd    *workspace.Command
	conversationKey string
	userKey         string
}

type uploadCommand struct {
	// /upload_N optionally carries a trailing question. When the remainder is
	// empty we only save files and reply with the import summary.
	count     int
	remainder string
	matched   bool
	ok        bool
}

type attachmentRef struct {
	messageID    string
	fileKey      string
	resourceType string
	createdAt    time.Time
}

type downloadedResource struct {
	tempPath      string
	originalName  string
	resourceType  string
	messageID     string
	fileKey       string
	messageCreate time.Time
}

type replyBody struct {
	msgType      string
	content      string
	fallbackText string
}

type postMessageContent struct {
	ZhCN postLocale `json:"zh_cn"`
}

type postLocale struct {
	Title   string         `json:"title,omitempty"`
	Content [][]postMDNode `json:"content"`
}

type postMDNode struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
}

type incomingPostMessageContent struct {
	ZhCN *incomingPostLocale `json:"zh_cn,omitempty"`
	EnUS *incomingPostLocale `json:"en_us,omitempty"`
	JaJP *incomingPostLocale `json:"ja_jp,omitempty"`
}

type incomingPostLocale struct {
	Title   string               `json:"title,omitempty"`
	Content [][]incomingPostNode `json:"content"`
}

type incomingPostNode struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	UserName string `json:"user_name,omitempty"`
	Name     string `json:"name,omitempty"`
}
