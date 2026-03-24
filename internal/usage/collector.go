package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/workspace"
)

const (
	sourceCLI   = "cli"
	sourceLark  = "lark"
	sourceOther = "other"
)

type Collector struct {
	sessionStorePath string
	logPath          string
	workspaceRoot    string
	sessionTTL       time.Duration
	logTailBytes     int64
	now              func() time.Time
}

type Snapshot struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	Summary         Summary            `json:"summary"`
	RequestStats    RequestStats       `json:"request_stats"`
	ModelBreakdown  []NamedValue       `json:"model_breakdown"`
	SourceBreakdown []NamedValue       `json:"source_breakdown"`
	RepoBreakdown   []RepoBreakdown    `json:"repo_breakdown"`
	RecentSessions  []SessionView      `json:"recent_sessions"`
	RecentRequests  []RequestEventView `json:"recent_requests"`
	RecentErrors    []LogEventView     `json:"recent_errors"`
}

type Summary struct {
	TotalConversations int `json:"total_conversations"`
	Active15Min        int `json:"active_15_min"`
	Active1Hour        int `json:"active_1_hour"`
	Active24Hours      int `json:"active_24_hours"`
	ResumableSessions  int `json:"resumable_sessions"`
	ErrorSessions      int `json:"error_sessions"`
	WorkspaceUsers     int `json:"workspace_users"`
	ActiveUsers24Hours int `json:"active_users_24_hours"`
}

type RequestStats struct {
	Requests5Min  int     `json:"requests_5_min"`
	Requests1Hour int     `json:"requests_1_hour"`
	Errors1Hour   int     `json:"errors_1_hour"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
}

type NamedValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type RepoBreakdown struct {
	Name  string       `json:"name"`
	Refs  []NamedValue `json:"refs"`
	Users int          `json:"users"`
}

type SessionView struct {
	ConversationKey string    `json:"conversation_key"`
	UserKey         string    `json:"user_key"`
	Source          string    `json:"source"`
	Model           string    `json:"model"`
	TurnCount       int       `json:"turn_count"`
	LastActiveAt    time.Time `json:"last_active_at"`
	LastError       string    `json:"last_error"`
	LastQuestion    string    `json:"last_question"`
}

type RequestEventView struct {
	Time            time.Time `json:"time"`
	Source          string    `json:"source"`
	ConversationKey string    `json:"conversation_key"`
	ElapsedMs       float64   `json:"elapsed_ms"`
}

type LogEventView struct {
	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}

type workspaceMetadata struct {
	UserKey      string                          `json:"user_key"`
	LastActiveAt time.Time                       `json:"last_active_at"`
	Repos        map[string]*workspace.RepoState `json:"repos"`
}

type logRequestEvent struct {
	Time            time.Time
	Source          string
	ConversationKey string
	Elapsed         time.Duration
}

type logErrorEvent struct {
	Time    time.Time
	Source  string
	Message string
}

func NewCollector(cfg *config.Config) *Collector {
	return &Collector{
		sessionStorePath: cfg.CodexSessionStore,
		logPath:          cfg.LogFile,
		workspaceRoot:    cfg.WorkspaceRoot,
		sessionTTL:       time.Duration(cfg.CodexSessionTTLHours) * time.Hour,
		logTailBytes:     int64(cfg.UsageLogTailBytes),
		now:              time.Now,
	}
}

func (c *Collector) Snapshot() (*Snapshot, error) {
	now := c.now().UTC()
	sessions, err := c.loadSessions()
	if err != nil {
		return nil, err
	}
	workspaces, err := c.loadWorkspaces()
	if err != nil {
		return nil, err
	}
	requests, errors, err := c.loadLogEvents()
	if err != nil {
		return nil, err
	}

	summary := buildSummary(now, sessions, workspaces, c.sessionTTL)
	return &Snapshot{
		GeneratedAt:     now,
		Summary:         summary,
		RequestStats:    buildRequestStats(now, requests, errors),
		ModelBreakdown:  buildModelBreakdown(sessions),
		SourceBreakdown: buildSourceBreakdown(sessions),
		RepoBreakdown:   buildRepoBreakdown(workspaces),
		RecentSessions:  buildRecentSessions(sessions),
		RecentRequests:  buildRecentRequests(requests),
		RecentErrors:    buildRecentErrors(errors),
	}, nil
}

func (c *Collector) loadSessions() (map[string]codex.SessionRecord, error) {
	data, err := os.ReadFile(c.sessionStorePath)
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

func (c *Collector) loadWorkspaces() ([]workspaceMetadata, error) {
	usersDir := filepath.Join(c.workspaceRoot, "users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workspace users dir: %w", err)
	}

	workspaces := make([]workspaceMetadata, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(usersDir, entry.Name(), "data", "workspace.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read workspace metadata %s: %w", metaPath, err)
		}
		var meta workspaceMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parse workspace metadata %s: %w", metaPath, err)
		}
		if meta.Repos == nil {
			meta.Repos = map[string]*workspace.RepoState{}
		}
		if strings.TrimSpace(meta.UserKey) == "" {
			meta.UserKey = entry.Name()
		}
		workspaces = append(workspaces, meta)
	}
	return workspaces, nil
}

func (c *Collector) loadLogEvents() ([]logRequestEvent, []logErrorEvent, error) {
	lines, err := readTailLines(c.logPath, c.logTailBytes)
	if err != nil {
		return nil, nil, err
	}

	requests := make([]logRequestEvent, 0, 64)
	errors := make([]logErrorEvent, 0, 64)
	for _, line := range lines {
		ts, rest, ok := splitLogLine(line)
		if !ok {
			continue
		}
		if event, ok := parseRequestEvent(ts, rest); ok {
			requests = append(requests, event)
		}
		if event, ok := parseErrorEvent(ts, rest); ok {
			errors = append(errors, event)
		}
	}
	return requests, errors, nil
}

func buildSummary(now time.Time, sessions map[string]codex.SessionRecord, workspaces []workspaceMetadata, ttl time.Duration) Summary {
	var summary Summary
	activeUsers := map[string]struct{}{}
	for _, session := range sessions {
		summary.TotalConversations++
		if within(now, session.LastActiveAt, 15*time.Minute) {
			summary.Active15Min++
		}
		if within(now, session.LastActiveAt, time.Hour) {
			summary.Active1Hour++
		}
		if within(now, session.LastActiveAt, 24*time.Hour) {
			summary.Active24Hours++
		}
		if strings.TrimSpace(session.LastError) != "" {
			summary.ErrorSessions++
		}
		if sessionResumable(now, session, ttl) {
			summary.ResumableSessions++
		}
		userKey := strings.TrimSpace(session.UserKey)
		if userKey != "" && within(now, session.LastActiveAt, 24*time.Hour) {
			activeUsers[userKey] = struct{}{}
		}
	}
	summary.WorkspaceUsers = len(workspaces)
	for _, meta := range workspaces {
		if within(now, meta.LastActiveAt, 24*time.Hour) {
			activeUsers[strings.TrimSpace(meta.UserKey)] = struct{}{}
		}
	}
	summary.ActiveUsers24Hours = len(activeUsers)
	return summary
}

func buildRequestStats(now time.Time, requests []logRequestEvent, errors []logErrorEvent) RequestStats {
	var stats RequestStats
	var latencies []float64
	for _, request := range requests {
		if within(now, request.Time, 5*time.Minute) {
			stats.Requests5Min++
		}
		if within(now, request.Time, time.Hour) {
			stats.Requests1Hour++
			if request.Elapsed > 0 {
				latencies = append(latencies, float64(request.Elapsed.Milliseconds()))
			}
		}
	}
	for _, event := range errors {
		if within(now, event.Time, time.Hour) {
			stats.Errors1Hour++
		}
	}
	stats.P50LatencyMs = percentile(latencies, 0.50)
	stats.P95LatencyMs = percentile(latencies, 0.95)
	return stats
}

func buildModelBreakdown(sessions map[string]codex.SessionRecord) []NamedValue {
	counts := map[string]int{}
	for _, session := range sessions {
		model := strings.TrimSpace(session.ModelOverride)
		if model == "" {
			model = "(default)"
		}
		counts[model]++
	}
	return sortNamedValues(counts)
}

func buildSourceBreakdown(sessions map[string]codex.SessionRecord) []NamedValue {
	counts := map[string]int{}
	for _, session := range sessions {
		counts[sourceForConversation(session.ConversationKey)]++
	}
	return sortNamedValues(counts)
}

func buildRepoBreakdown(workspaces []workspaceMetadata) []RepoBreakdown {
	counts := map[string]map[string]int{}
	users := map[string]int{}
	for _, meta := range workspaces {
		for name, state := range meta.Repos {
			if state == nil {
				continue
			}
			if _, ok := counts[name]; !ok {
				counts[name] = map[string]int{}
			}
			ref := strings.TrimSpace(state.RequestedRef)
			if ref == "" {
				ref = "(unknown)"
			}
			counts[name][ref]++
			users[name]++
		}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]RepoBreakdown, 0, len(names))
	for _, name := range names {
		result = append(result, RepoBreakdown{
			Name:  name,
			Refs:  sortNamedValues(counts[name]),
			Users: users[name],
		})
	}
	return result
}

func buildRecentSessions(sessions map[string]codex.SessionRecord) []SessionView {
	items := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, SessionView{
			ConversationKey: session.ConversationKey,
			UserKey:         session.UserKey,
			Source:          sourceForConversation(session.ConversationKey),
			Model:           fallbackString(strings.TrimSpace(session.ModelOverride), "(default)"),
			TurnCount:       session.TurnCount,
			LastActiveAt:    session.LastActiveAt,
			LastError:       compactText(session.LastError, 160),
			LastQuestion:    compactText(lastQuestion(session.Turns), 120),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastActiveAt.After(items[j].LastActiveAt)
	})
	if len(items) > 12 {
		items = items[:12]
	}
	return items
}

func buildRecentRequests(requests []logRequestEvent) []RequestEventView {
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].Time.After(requests[j].Time)
	})
	if len(requests) > 20 {
		requests = requests[:20]
	}
	result := make([]RequestEventView, 0, len(requests))
	for _, event := range requests {
		result = append(result, RequestEventView{
			Time:            event.Time,
			Source:          event.Source,
			ConversationKey: event.ConversationKey,
			ElapsedMs:       float64(event.Elapsed.Milliseconds()),
		})
	}
	return result
}

func buildRecentErrors(errors []logErrorEvent) []LogEventView {
	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Time.After(errors[j].Time)
	})
	if len(errors) > 20 {
		errors = errors[:20]
	}
	result := make([]LogEventView, 0, len(errors))
	for _, event := range errors {
		result = append(result, LogEventView{
			Time:    event.Time,
			Source:  event.Source,
			Message: compactText(event.Message, 220),
		})
	}
	return result
}

func readTailLines(path string, tailBytes int64) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log file: %w", err)
	}
	size := info.Size()
	start := int64(0)
	if tailBytes > 0 && size > tailBytes {
		start = size - tailBytes
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, fmt.Errorf("seek log file: %w", err)
	}

	scanner := bufio.NewScanner(file)
	if start > 0 && scanner.Scan() {
		// Drop the partial first line from the seek offset.
	}

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file: %w", err)
	}
	return lines, nil
}

func splitLogLine(line string) (time.Time, string, bool) {
	if len(line) < len("2006/01/02 15:04:05") {
		return time.Time{}, "", false
	}
	ts, err := time.ParseInLocation("2006/01/02 15:04:05", line[:19], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}
	rest := strings.TrimSpace(line[19:])
	return ts.UTC(), rest, true
}

func parseRequestEvent(ts time.Time, line string) (logRequestEvent, bool) {
	switch {
	case strings.Contains(line, "[askplanner] request done"):
		return logRequestEvent{
			Time:            ts,
			Source:          sourceCLI,
			ConversationKey: extractField(line, "conversation"),
			Elapsed:         parseElapsedField(line),
		}, true
	case strings.Contains(line, "[larkbot] handle event done"):
		return logRequestEvent{
			Time:            ts,
			Source:          sourceLark,
			ConversationKey: extractField(line, "conversation"),
			Elapsed:         parseElapsedField(line),
		}, true
	default:
		return logRequestEvent{}, false
	}
}

func parseErrorEvent(ts time.Time, line string) (logErrorEvent, bool) {
	switch {
	case strings.Contains(line, "[larkbot] handle event error:"):
		return logErrorEvent{Time: ts, Source: sourceLark, Message: afterMarker(line, "[larkbot] handle event error:")}, true
	case strings.Contains(line, "[codex] resume failed"):
		return logErrorEvent{Time: ts, Source: sourceOther, Message: afterMarker(line, "[codex] resume failed")}, true
	case strings.Contains(line, "startup error:"):
		return logErrorEvent{Time: ts, Source: sourceOther, Message: afterMarker(line, "startup error:")}, true
	default:
		return logErrorEvent{}, false
	}
}

func parseElapsedField(line string) time.Duration {
	value := extractField(line, "elapsed")
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return d
}

func extractField(line, key string) string {
	marker := key + "="
	idx := strings.Index(line, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.IndexByte(line[start:], ' ')
	if end < 0 {
		return strings.TrimSpace(line[start:])
	}
	return strings.TrimSpace(line[start : start+end])
}

func afterMarker(line, marker string) string {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(line[idx+len(marker):])
}

func sessionResumable(now time.Time, session codex.SessionRecord, ttl time.Duration) bool {
	if strings.TrimSpace(session.SessionID) == "" {
		return false
	}
	if ttl > 0 && !session.LastActiveAt.IsZero() && now.Sub(session.LastActiveAt) > ttl {
		return false
	}
	return true
}

func sourceForConversation(key string) string {
	switch {
	case strings.HasPrefix(key, "cli:"):
		return sourceCLI
	case strings.HasPrefix(key, "lark:"):
		return sourceLark
	default:
		return sourceOther
	}
}

func sortNamedValues(counts map[string]int) []NamedValue {
	items := make([]NamedValue, 0, len(counts))
	for name, value := range counts {
		items = append(items, NamedValue{Name: name, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Value == items[j].Value {
			return items[i].Name < items[j].Name
		}
		return items[i].Value > items[j].Value
	})
	return items
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	index := int(float64(len(values)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func lastQuestion(turns []codex.Turn) string {
	if len(turns) == 0 {
		return ""
	}
	return turns[len(turns)-1].User
}

func compactText(s string, limit int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func within(now, then time.Time, d time.Duration) bool {
	if then.IsZero() || d <= 0 {
		return false
	}
	return now.Sub(then) <= d
}

func formatMillis(ms float64) string {
	if ms <= 0 {
		return "0"
	}
	return strconv.FormatFloat(ms, 'f', 0, 64)
}
