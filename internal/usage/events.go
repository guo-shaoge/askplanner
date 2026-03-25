package usage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
)

const (
	sourceCLI   = "cli"
	sourceLark  = "lark"
	sourceOther = "other"

	statusSuccess      = "success"
	statusError        = "error"
	statusShortCircuit = "short_circuit"

	defaultIntroQuestion = "Please introduce your capabilities."
	cliVirtualUserKey    = "cli:default"
)

type QuestionEvent struct {
	EventID             string    `json:"event_id"`
	AskedAt             time.Time `json:"asked_at"`
	Source              string    `json:"source"`
	UserKey             string    `json:"user_key"`
	ConversationKey     string    `json:"conversation_key"`
	Question            string    `json:"question"`
	Status              string    `json:"status"`
	DurationMs          int64     `json:"duration_ms,omitempty"`
	Model               string    `json:"model,omitempty"`
	Error               string    `json:"error,omitempty"`
	WorkspaceEnvHash    string    `json:"workspace_env_hash,omitempty"`
	Backfilled          bool      `json:"backfilled,omitempty"`
	QuestionFingerprint string    `json:"question_fingerprint,omitempty"`
}

type QuestionTracker struct {
	store *QuestionStore
	now   func() time.Time
}

type QuestionSpan struct {
	tracker *QuestionTracker
	event   QuestionEvent
	done    bool
}

type QuestionStore struct {
	path         string
	sessionStore string
}

func NewQuestionTracker(cfg *config.Config) (*QuestionTracker, error) {
	store := &QuestionStore{
		path:         cfg.UsageQuestionsPath,
		sessionStore: cfg.CodexSessionStore,
	}
	if err := store.BackfillFromSessions(); err != nil {
		return nil, err
	}
	return &QuestionTracker{
		store: store,
		now:   time.Now,
	}, nil
}

func NewQuestionStore(cfg *config.Config) (*QuestionStore, error) {
	store := &QuestionStore{
		path:         cfg.UsageQuestionsPath,
		sessionStore: cfg.CodexSessionStore,
	}
	if err := store.BackfillFromSessions(); err != nil {
		return nil, err
	}
	return store, nil
}

func (t *QuestionTracker) Begin(source, userKey, conversationKey, question, model, envHash string) *QuestionSpan {
	question = strings.TrimSpace(question)
	if !shouldTrackQuestion(question) || t == nil || t.store == nil {
		return nil
	}
	now := time.Now().UTC()
	if t.now != nil {
		now = t.now().UTC()
	}
	event := QuestionEvent{
		EventID:             newLiveEventID(source, userKey, conversationKey, question, now),
		AskedAt:             now,
		Source:              normalizeSource(source),
		UserKey:             normalizeUserKey(userKey, source, conversationKey),
		ConversationKey:     strings.TrimSpace(conversationKey),
		Question:            question,
		Model:               strings.TrimSpace(model),
		WorkspaceEnvHash:    strings.TrimSpace(envHash),
		QuestionFingerprint: questionFingerprint(question),
	}
	return &QuestionSpan{tracker: t, event: event}
}

func (s *QuestionSpan) Success() {
	s.finalize(statusSuccess, "")
}

func (s *QuestionSpan) ShortCircuit() {
	s.finalize(statusShortCircuit, "")
}

func (s *QuestionSpan) Error(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	s.finalize(statusError, message)
}

func (s *QuestionSpan) finalize(status, errText string) {
	if s == nil || s.done || s.tracker == nil || s.tracker.store == nil {
		return
	}
	s.done = true
	status = strings.TrimSpace(status)
	if status == "" {
		status = statusSuccess
	}
	s.event.Status = status
	if end := s.tracker.now; end != nil {
		s.event.DurationMs = time.Since(s.event.AskedAt).Milliseconds()
	} else {
		s.event.DurationMs = time.Since(s.event.AskedAt).Milliseconds()
	}
	s.event.Error = compactText(errText, 400)
	if err := s.tracker.store.Append(s.event); err != nil {
		// Usage tracking must not break the user path.
		fmt.Fprintf(os.Stderr, "usage tracker append failed: %v\n", err)
	}
}

func (s *QuestionStore) LoadAll() ([]QuestionEvent, error) {
	if s == nil {
		return nil, nil
	}
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open question store: %w", err)
	}
	defer closeQuietly(file)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var events []QuestionEvent
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event QuestionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse question event: %w", err)
		}
		event.Source = normalizeSource(event.Source)
		event.UserKey = normalizeUserKey(event.UserKey, event.Source, event.ConversationKey)
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan question store: %w", err)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].AskedAt.Equal(events[j].AskedAt) {
			return events[i].EventID < events[j].EventID
		}
		return events[i].AskedAt.Before(events[j].AskedAt)
	})
	return events, nil
}

func (s *QuestionStore) Append(event QuestionEvent) error {
	if s == nil {
		return nil
	}
	event.Source = normalizeSource(event.Source)
	event.UserKey = normalizeUserKey(event.UserKey, event.Source, event.ConversationKey)
	event.Question = strings.TrimSpace(event.Question)
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("question event id is empty")
	}
	if event.AskedAt.IsZero() {
		event.AskedAt = time.Now().UTC()
	}
	if event.Question == "" {
		return nil
	}
	if event.QuestionFingerprint == "" {
		event.QuestionFingerprint = questionFingerprint(event.Question)
	}

	lock, err := acquireUsageLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer closeQuietly(lock)

	existing, err := s.loadAllNoLock()
	if err != nil {
		return err
	}
	for _, item := range existing {
		if item.EventID == event.EventID {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create question store dir: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open question store for append: %w", err)
	}
	defer closeQuietly(f)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal question event: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append question event: %w", err)
	}
	return nil
}

func (s *QuestionStore) BackfillFromSessions() error {
	if s == nil || strings.TrimSpace(s.sessionStore) == "" {
		return nil
	}
	lock, err := acquireUsageLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer closeQuietly(lock)

	sessions, err := loadSessionRecords(s.sessionStore)
	if err != nil {
		return err
	}
	existing, err := s.loadAllNoLock()
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(existing))
	for _, event := range existing {
		seen[event.EventID] = struct{}{}
	}

	toAppend := make([]QuestionEvent, 0, 32)
	for _, session := range sessions {
		source := sourceForConversation(session.ConversationKey)
		userKey := normalizeUserKey(strings.TrimSpace(session.UserKey), source, session.ConversationKey)
		model := strings.TrimSpace(session.ModelOverride)
		if model == "" {
			model = "(default)"
		}
		for _, turn := range session.Turns {
			question := strings.TrimSpace(turn.User)
			if !shouldTrackQuestion(question) {
				continue
			}
			askedAt := turn.At.UTC()
			if askedAt.IsZero() {
				askedAt = session.LastActiveAt.UTC()
			}
			event := QuestionEvent{
				EventID:             newBackfillEventID(source, userKey, session.ConversationKey, question, askedAt),
				AskedAt:             askedAt,
				Source:              source,
				UserKey:             userKey,
				ConversationKey:     strings.TrimSpace(session.ConversationKey),
				Question:            question,
				Status:              statusSuccess,
				Model:               model,
				WorkspaceEnvHash:    strings.TrimSpace(session.EnvironmentHash),
				Backfilled:          true,
				QuestionFingerprint: questionFingerprint(question),
			}
			if _, ok := seen[event.EventID]; ok {
				continue
			}
			seen[event.EventID] = struct{}{}
			toAppend = append(toAppend, event)
		}
	}
	if len(toAppend) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create question store dir: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open question store for backfill: %w", err)
	}
	defer closeQuietly(f)

	for _, event := range toAppend {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal backfill question event: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("append backfill question event: %w", err)
		}
	}
	return nil
}

func (s *QuestionStore) loadAllNoLock() ([]QuestionEvent, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open question store: %w", err)
	}
	defer closeQuietly(file)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var events []QuestionEvent
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event QuestionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse question event: %w", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan question store: %w", err)
	}
	return events, nil
}

type usageLock struct {
	file *os.File
}

func (l *usageLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func acquireUsageLock(path string) (*usageLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create usage lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open usage lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock usage store: %w", err)
	}
	return &usageLock{file: f}, nil
}

func loadSessionRecords(path string) (map[string]codex.SessionRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]codex.SessionRecord{}, nil
		}
		return nil, fmt.Errorf("read session store: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]codex.SessionRecord{}, nil
	}
	var sessions map[string]codex.SessionRecord
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parse session store: %w", err)
	}
	if sessions == nil {
		return map[string]codex.SessionRecord{}, nil
	}
	return sessions, nil
}

func shouldTrackQuestion(question string) bool {
	question = strings.TrimSpace(question)
	return question != "" && question != defaultIntroQuestion
}

func normalizeSource(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	switch source {
	case sourceCLI, sourceLark:
		return source
	default:
		return sourceOther
	}
}

func normalizeUserKey(userKey, source, conversationKey string) string {
	userKey = strings.TrimSpace(userKey)
	if normalizeSource(source) == sourceCLI {
		return cliVirtualUserKey
	}
	if userKey != "" {
		return userKey
	}
	return strings.TrimSpace(conversationKey)
}

func newLiveEventID(source, userKey, conversationKey, question string, askedAt time.Time) string {
	return buildEventID("live", source, userKey, conversationKey, question, askedAt)
}

func newBackfillEventID(source, userKey, conversationKey, question string, askedAt time.Time) string {
	return buildEventID("backfill", source, userKey, conversationKey, question, askedAt)
}

func buildEventID(kind, source, userKey, conversationKey, question string, askedAt time.Time) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(kind),
		normalizeSource(source),
		normalizeUserKey(userKey, source, conversationKey),
		strings.TrimSpace(conversationKey),
		strings.TrimSpace(question),
		askedAt.UTC().Format(time.RFC3339Nano),
	}, "|")))
	return hex.EncodeToString(sum[:])
}

func questionFingerprint(question string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(question)))
	return hex.EncodeToString(sum[:8])
}
