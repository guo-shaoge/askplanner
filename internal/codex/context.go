package codex

import (
	"context"
	"time"
)

type ThreadContextLoader func(context.Context) (*ThreadContext, error)

type RuntimeContext struct {
	Attachment    AttachmentContext
	ClinicLibrary *ClinicLibraryContext
	Clinic        *ClinicContext
	// UserKey is internal bookkeeping metadata for session management and must
	// not be rendered into prompts.
	UserKey string
	// Thread carries thread-local history for the latest inbound message.
	// In this project it is typically populated by the Lark relay when the
	// incoming event has a non-empty Feishu thread_id.
	//
	// ThreadContext is intended for bootstrapping a brand-new bot session with
	// nearby topic history. The prompt builder currently uses it only in
	// BuildInitialPrompt; resume prompts intentionally ignore it to avoid
	// replaying the same thread history on every turn.
	Thread *ThreadContext
	// ThreadLoader lazily resolves Thread for initial-prompt paths only. This
	// lets callers defer network work until the responder has decided it really
	// needs to start a new session, including resume-failure fallback paths.
	ThreadLoader ThreadContextLoader
	Workspace    *WorkspaceContext
}

type WorkspaceContext struct {
	RootDir         string
	UserFilesDir    string
	ClinicFilesDir  string
	EnvironmentHash string
	Repos           []WorkspaceRepoContext
}

type WorkspaceRepoContext struct {
	Name           string
	RelativePath   string
	RequestedRef   string
	ResolvedSHA    string
	TrackingLatest bool
}

type ClinicLibraryContext struct {
	RootDir        string
	ActiveItemName string
	Items          []ClinicLibraryItem
}

type ClinicLibraryItem struct {
	Name        string
	SavedAt     time.Time
	ClusterID   string
	ClusterName string
	Digest      string
	Database    string
	Instance    string
	IsDetail    bool
}

type ClinicContext struct {
	SourceURL   string
	ClusterID   string
	ClusterName string
	OrgName     string
	DeployType  string
	StartTime   time.Time
	EndTime     time.Time
	Digest      string
	Database    string
	Instance    string
	IsDetail    bool
	Summary     ClinicSummary
	TopDigests  []ClinicDigestSummary
	DetailRows  []ClinicDetailRow
	NoRows      bool
}

type ClinicSummary struct {
	TotalQueries  int64
	UniqueDigests int64
	AvgQueryTime  float64
	MaxQueryTime  float64
}

type ClinicDigestSummary struct {
	Digest            string
	PlanDigest        string
	ExecutionCount    int64
	AvgQueryTime      float64
	MaxQueryTime      float64
	MaxTotalKeys      int64
	MaxProcessKeys    int64
	MaxResultRows     int64
	MaxMemBytes       int64
	MaxDiskBytes      int64
	SampleDB          string
	SampleInstance    string
	SampleIndexes     string
	SamplePrevStmt    string
	SamplePlan        string
	SampleDecodedPlan string
	SampleBinaryPlan  string
	SampleSQL         string
}

type ClinicDetailRow struct {
	TimeUnix    float64
	Digest      string
	PlanDigest  string
	QueryTime   float64
	ParseTime   float64
	CompileTime float64
	CopTime     float64
	ProcessTime float64
	WaitTime    float64
	TotalKeys   int64
	ProcessKeys int64
	ResultRows  int64
	MemBytes    int64
	DiskBytes   int64
	Database    string
	Instance    string
	Indexes     string
	PrevStmt    string
	Plan        string
	DecodedPlan string
	BinaryPlan  string
	Query       string
}

// ThreadContext describes earlier messages from the same external message
// thread as the latest user message.
//
// Usage:
//  1. Populate it only when the transport has a stable thread concept
//     (for example Feishu topic messages with a non-empty thread_id).
//  2. Fill Messages with earlier messages only; do not include the current
//     inbound message again because it is already rendered separately as the
//     latest user question.
//  3. Keep Messages ordered oldest-first so prompt rendering can show a natural
//     conversation flow without extra sorting logic at the call site.
//  4. Prefer concise, user-visible Content. Text should already have mention
//     keys normalized, excessive blank lines collapsed, and non-text payloads
//     summarized into short placeholders when needed.
//  5. Treat this as best-effort context. The producer may provide only the root
//     message plus a nearby history window instead of the entire thread.
//
// Semantics:
//   - ThreadID identifies the external thread/topic.
//   - RootMessageID identifies the thread root when known. If the transport
//     does not expose a stable root ID, producers may fall back to the oldest
//     message currently kept in the local history window.
//   - ParentMessageID identifies the direct parent of the current inbound
//     message when the transport exposes that concept.
//   - OmittedCount is optional metadata about older messages not included in
//     Messages. Depending on the producer, it may be exact or a lower bound.
type ThreadContext struct {
	// ThreadID is the external thread/topic identifier, for example a Feishu
	// thread_id. An empty value means the thread context is not usable.
	ThreadID string
	// RootMessageID is the root message of the thread when known. It helps the
	// model understand which earlier message anchors the thread even when only a
	// subset of historical messages is included. When the transport does not
	// expose a stable root ID, this may instead be the oldest message currently
	// present in the local history window.
	RootMessageID string
	// ParentMessageID is the direct parent of the current inbound message when
	// available from the transport. It is metadata about the current message,
	// not necessarily the parent of every entry in Messages.
	ParentMessageID string
	// OmittedCount reports how many older thread messages were left out of
	// Messages. Depending on the producer, it may be exact or a lower bound if
	// scanning stopped early once a nearby history window was filled.
	OmittedCount int
	// Messages contains earlier thread messages only, ordered oldest-first.
	// The current inbound message should not appear here.
	Messages []ThreadMessage
}

// ThreadMessage is one earlier message included in ThreadContext.Messages.
//
// The goal of this shape is prompt rendering rather than lossless transport
// serialization, so fields should capture just enough structure for the model
// to recover the thread flow:
// - identity (MessageID / RootMessageID / ParentMessageID)
// - rough author and type
// - timestamp
// - normalized human-readable content
type ThreadMessage struct {
	// MessageID is the external identifier of this historical message.
	MessageID string
	// RootMessageID repeats the thread root identifier when the transport
	// provides it. This can be useful for messages loaded from heterogeneous
	// sources where not every item is guaranteed to share the same metadata.
	RootMessageID string
	// ParentMessageID is the parent of this historical message when known.
	ParentMessageID string
	// SenderLabel is a compact human-readable sender marker such as
	// "user:ou_xxx" or "app:cli_xxx". It should be short and stable rather than
	// a rich profile payload.
	SenderLabel string
	// MessageType is the transport-native message type, for example "text",
	// "post", or "interactive".
	MessageType string
	// CreatedAt is the original creation time when known.
	CreatedAt time.Time
	// Content is normalized prompt-friendly text. For text messages this should
	// be the visible text after mention rewriting and whitespace cleanup; for
	// non-text messages a short placeholder such as "[image]" is preferred over
	// raw JSON.
	Content string
}
