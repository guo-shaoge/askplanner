package larkbot

import (
	"context"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"

	"lab/askplanner/internal/attachments"
	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/modelcmd"
	"lab/askplanner/internal/usage"
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
	key  string
	name string
}

func (b botIdentity) prefix() string {
	key := sanitizePathSegment(b.key, "default")
	return fmt.Sprintf("[larkbot:%s]", key)
}

type websocketClient interface {
	Start(ctx context.Context) error
}

type websocketClientFactory func(appID, appSecret string, handler *dispatcher.EventDispatcher) websocketClient

type botRuntime struct {
	parent      *App
	cfg         config.LarkBotConfig
	apiClient   *lark.Client
	dedup       *messageDedup
	bot         botIdentity
	client      websocketClient
	dedupMaxAge time.Duration
}

type sharedServices struct {
	responder   *codex.Responder
	prefetcher  *clinic.Prefetcher
	attachments *attachments.Manager
	workspace   *workspace.Manager
	tracker     *usage.QuestionTracker
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
	modelCmd        *modelcmd.Command
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
