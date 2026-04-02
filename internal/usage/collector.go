package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type Collector struct {
	sessionStorePath string
	logPath          string
	workspaceRoot    string
	sessionTTL       time.Duration
	logTailBytes     int64
	questionStore    *QuestionStore
	userResolver     usageUserResolver
	now              func() time.Time
}

func closeQuietly(closer io.Closer) {
	_ = closer.Close()
}

type Snapshot struct {
	GeneratedAt             time.Time          `json:"generated_at"`
	Summary                 Summary            `json:"summary"`
	RequestStats            RequestStats       `json:"request_stats"`
	ModelBreakdown          []NamedValue       `json:"model_breakdown"`
	SourceBreakdown         []NamedValue       `json:"source_breakdown"`
	RepoBreakdown           []RepoBreakdown    `json:"repo_breakdown"`
	QuestionStatusBreakdown []NamedValue       `json:"question_status_breakdown"`
	QuestionsPerDay7D       []DateValue        `json:"questions_per_day_7d"`
	TopUsers                []UserSummary      `json:"top_users"`
	RecentSessions          []SessionView      `json:"recent_sessions"`
	RecentRequests          []RequestEventView `json:"recent_requests"`
	RecentErrors            []LogEventView     `json:"recent_errors"`
}

type Summary struct {
	TotalConversations  int     `json:"total_conversations"`
	Active15Min         int     `json:"active_15_min"`
	Active1Hour         int     `json:"active_1_hour"`
	Active24Hours       int     `json:"active_24_hours"`
	ResumableSessions   int     `json:"resumable_sessions"`
	ErrorSessions       int     `json:"error_sessions"`
	WorkspaceUsers      int     `json:"workspace_users"`
	ActiveUsers24Hours  int     `json:"active_users_24_hours"`
	ActiveUsers7Days    int     `json:"active_users_7_days"`
	TotalUsers          int     `json:"total_users"`
	TotalQuestions      int     `json:"total_questions"`
	AvgQuestionsPerUser float64 `json:"avg_questions_per_user"`
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

type DateValue struct {
	Date  string `json:"date"`
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
	UserName        string    `json:"user_name,omitempty"`
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

type UserSummary struct {
	UserKey          string    `json:"user_key"`
	UserName         string    `json:"user_name,omitempty"`
	Source           string    `json:"source"`
	QuestionCount    int       `json:"question_count"`
	QuestionCount24H int       `json:"question_count_24h"`
	QuestionCount7D  int       `json:"question_count_7d"`
	LastAskedAt      time.Time `json:"last_asked_at"`
	RecentQuestion   string    `json:"recent_question"`
	LastConversation string    `json:"-"`
}

type QuestionView struct {
	EventID         string    `json:"event_id"`
	AskedAt         time.Time `json:"asked_at"`
	Source          string    `json:"source"`
	UserKey         string    `json:"user_key"`
	UserName        string    `json:"user_name,omitempty"`
	ConversationKey string    `json:"conversation_key"`
	Question        string    `json:"question"`
	Status          string    `json:"status"`
	DurationMs      int64     `json:"duration_ms"`
	Model           string    `json:"model"`
	Error           string    `json:"error"`
	Backfilled      bool      `json:"backfilled"`
}

type QuestionsPage struct {
	Items      []QuestionView `json:"items"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalItems int            `json:"total_items"`
	TotalPages int            `json:"total_pages"`
}

type QuestionQuery struct {
	Page     int
	PageSize int
	UserKey  string
	Source   string
	Status   string
	Query    string
	From     time.Time
	To       time.Time
}

type UsersPage struct {
	Items      []UserSummary `json:"items"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalItems int           `json:"total_items"`
	TotalPages int           `json:"total_pages"`
}

type UserQuery struct {
	Page     int
	PageSize int
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

func NewCollector(cfg *config.Config) (*Collector, error) {
	store, err := NewQuestionStore(cfg)
	if err != nil {
		return nil, err
	}
	resolver := newUsageUserResolver(cfg)
	return &Collector{
		sessionStorePath: cfg.CodexSessionStore,
		logPath:          cfg.LogFile,
		workspaceRoot:    cfg.WorkspaceRoot,
		sessionTTL:       time.Duration(cfg.CodexSessionTTLHours) * time.Hour,
		logTailBytes:     int64(cfg.UsageLogTailBytes),
		questionStore:    store,
		userResolver:     resolver,
		now:              time.Now,
	}, nil
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
	questions, err := c.loadQuestions()
	if err != nil {
		return nil, err
	}
	users := aggregateUsers(now, questions)

	return &Snapshot{
		GeneratedAt:             now,
		Summary:                 buildSummary(now, sessions, workspaces, questions, users, c.sessionTTL),
		RequestStats:            buildRequestStats(now, requests, errors),
		ModelBreakdown:          buildModelBreakdown(sessions),
		SourceBreakdown:         buildSourceBreakdown(sessions),
		RepoBreakdown:           buildRepoBreakdown(workspaces),
		QuestionStatusBreakdown: buildQuestionStatusBreakdown(questions),
		QuestionsPerDay7D:       buildQuestionsPerDay(now, questions, 7),
		TopUsers:                c.enrichUsers(context.Background(), topUsers(users, 12)),
		RecentSessions:          c.enrichSessions(context.Background(), buildRecentSessions(sessions)),
		RecentRequests:          buildRecentRequests(requests),
		RecentErrors:            buildRecentErrors(errors),
	}, nil
}

func (c *Collector) QuestionsPage(query QuestionQuery) (*QuestionsPage, error) {
	questions, err := c.loadQuestions()
	if err != nil {
		return nil, err
	}
	names := c.userNamesForQuestions(context.Background(), questions)
	filtered := filterQuestions(questions, query, names)
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].AskedAt.Equal(filtered[j].AskedAt) {
			return filtered[i].EventID > filtered[j].EventID
		}
		return filtered[i].AskedAt.After(filtered[j].AskedAt)
	})
	page, pageSize := normalizePagination(query.Page, query.PageSize)
	start, end, totalPages := paginateBounds(len(filtered), page, pageSize)
	items := make([]QuestionView, 0, end-start)
	for _, event := range filtered[start:end] {
		name := names[userLookupKey(event.Source, event.UserKey, event.ConversationKey)]
		items = append(items, QuestionView{
			EventID:         event.EventID,
			AskedAt:         event.AskedAt,
			Source:          event.Source,
			UserKey:         event.UserKey,
			UserName:        name,
			ConversationKey: event.ConversationKey,
			Question:        event.Question,
			Status:          event.Status,
			DurationMs:      event.DurationMs,
			Model:           fallbackString(event.Model, "(default)"),
			Error:           compactText(event.Error, 240),
			Backfilled:      event.Backfilled,
		})
	}
	return &QuestionsPage{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: len(filtered),
		TotalPages: totalPages,
	}, nil
}

func (c *Collector) UsersPage(query UserQuery) (*UsersPage, error) {
	questions, err := c.loadQuestions()
	if err != nil {
		return nil, err
	}
	users := aggregateUsers(c.now().UTC(), questions)
	sort.Slice(users, func(i, j int) bool {
		if users[i].QuestionCount == users[j].QuestionCount {
			return users[i].LastAskedAt.After(users[j].LastAskedAt)
		}
		return users[i].QuestionCount > users[j].QuestionCount
	})
	page, pageSize := normalizePagination(query.Page, query.PageSize)
	start, end, totalPages := paginateBounds(len(users), page, pageSize)
	return &UsersPage{
		Items:      c.enrichUsers(context.Background(), append([]UserSummary(nil), users[start:end]...)),
		Page:       page,
		PageSize:   pageSize,
		TotalItems: len(users),
		TotalPages: totalPages,
	}, nil
}

func (c *Collector) loadSessions() (map[string]codex.SessionRecord, error) {
	return loadSessionRecords(c.sessionStorePath)
}

func (c *Collector) loadQuestions() ([]QuestionEvent, error) {
	if c.questionStore == nil {
		return nil, nil
	}
	return c.questionStore.LoadAll()
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

func buildSummary(now time.Time, sessions map[string]codex.SessionRecord, workspaces []workspaceMetadata, questions []QuestionEvent, users []UserSummary, ttl time.Duration) Summary {
	var summary Summary
	activeUsers24h := map[string]struct{}{}
	activeUsers7d := map[string]struct{}{}

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
	}
	summary.WorkspaceUsers = len(workspaces)

	for _, event := range questions {
		summary.TotalQuestions++
		if within(now, event.AskedAt, 24*time.Hour) {
			activeUsers24h[event.UserKey] = struct{}{}
		}
		if within(now, event.AskedAt, 7*24*time.Hour) {
			activeUsers7d[event.UserKey] = struct{}{}
		}
	}
	summary.TotalUsers = len(users)
	summary.ActiveUsers24Hours = len(activeUsers24h)
	summary.ActiveUsers7Days = len(activeUsers7d)
	if summary.TotalUsers > 0 {
		summary.AvgQuestionsPerUser = float64(summary.TotalQuestions) / float64(summary.TotalUsers)
	}
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

func buildQuestionStatusBreakdown(questions []QuestionEvent) []NamedValue {
	counts := map[string]int{}
	for _, event := range questions {
		status := strings.TrimSpace(event.Status)
		if status == "" {
			status = statusSuccess
		}
		counts[status]++
	}
	return sortNamedValues(counts)
}

func buildQuestionsPerDay(now time.Time, questions []QuestionEvent, days int) []DateValue {
	if days <= 0 {
		return nil
	}
	start := now.UTC().AddDate(0, 0, -(days - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	counts := map[string]int{}
	for _, event := range questions {
		if event.AskedAt.Before(start) {
			continue
		}
		key := event.AskedAt.UTC().Format("2006-01-02")
		counts[key]++
	}
	result := make([]DateValue, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		result = append(result, DateValue{Date: key, Value: counts[key]})
	}
	return result
}

func aggregateUsers(now time.Time, questions []QuestionEvent) []UserSummary {
	type aggregate struct {
		UserSummary
	}
	users := map[string]*aggregate{}
	for _, event := range questions {
		key := strings.TrimSpace(event.UserKey)
		if key == "" {
			continue
		}
		item, ok := users[key]
		if !ok {
			item = &aggregate{UserSummary: UserSummary{
				UserKey: key,
				Source:  event.Source,
			}}
			users[key] = item
		}
		item.QuestionCount++
		if within(now, event.AskedAt, 24*time.Hour) {
			item.QuestionCount24H++
		}
		if within(now, event.AskedAt, 7*24*time.Hour) {
			item.QuestionCount7D++
		}
		if event.AskedAt.After(item.LastAskedAt) {
			item.LastAskedAt = event.AskedAt
			item.RecentQuestion = compactText(event.Question, 120)
			item.Source = event.Source
			item.LastConversation = event.ConversationKey
		}
	}
	result := make([]UserSummary, 0, len(users))
	for _, item := range users {
		result = append(result, item.UserSummary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].QuestionCount == result[j].QuestionCount {
			return result[i].LastAskedAt.After(result[j].LastAskedAt)
		}
		return result[i].QuestionCount > result[j].QuestionCount
	})
	return result
}

func topUsers(users []UserSummary, limit int) []UserSummary {
	if limit <= 0 || len(users) <= limit {
		return append([]UserSummary(nil), users...)
	}
	return append([]UserSummary(nil), users[:limit]...)
}

func buildRecentSessions(sessions map[string]codex.SessionRecord) []SessionView {
	items := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, SessionView{
			ConversationKey: session.ConversationKey,
			UserKey:         normalizeUserKey(session.UserKey, sourceForConversation(session.ConversationKey), session.ConversationKey),
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

func filterQuestions(questions []QuestionEvent, query QuestionQuery, userNames map[string]string) []QuestionEvent {
	userKey := strings.TrimSpace(query.UserKey)
	source := normalizeSource(query.Source)
	status := strings.TrimSpace(strings.ToLower(query.Status))
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	var result []QuestionEvent
	for _, event := range questions {
		if userKey != "" && event.UserKey != userKey {
			continue
		}
		if strings.TrimSpace(query.Source) != "" && event.Source != source {
			continue
		}
		if status != "" && strings.ToLower(strings.TrimSpace(event.Status)) != status {
			continue
		}
		if !query.From.IsZero() && event.AskedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && event.AskedAt.After(query.To) {
			continue
		}
		if needle != "" {
			haystack := strings.ToLower(event.Question + " " + event.UserKey + " " + event.ConversationKey + " " + userNames[userLookupKey(event.Source, event.UserKey, event.ConversationKey)])
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		result = append(result, event)
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
	defer closeQuietly(file)

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

	var lines []string
	reader := bufio.NewReader(file)
	if start > 0 {
		if _, err := reader.ReadString('\n'); err != nil && err != io.EOF {
			return nil, fmt.Errorf("skip partial log line: %w", err)
		}
	}
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			lines = append(lines, line)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("scan log file: %w", err)
		}
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
	return ts.UTC(), strings.TrimSpace(line[19:]), true
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
	case strings.HasPrefix(key, "larkbot:"):
		return sourceLark
	case strings.HasPrefix(key, "lark:"):
		return sourceLark
	default:
		return sourceOther
	}
}

func (c *Collector) userNamesForQuestions(ctx context.Context, questions []QuestionEvent) map[string]string {
	result := make(map[string]string, len(questions))
	for _, event := range questions {
		key := userLookupKey(event.Source, event.UserKey, event.ConversationKey)
		if _, ok := result[key]; ok {
			continue
		}
		result[key] = c.lookupUserName(ctx, event.Source, event.UserKey, event.ConversationKey)
	}
	return result
}

func (c *Collector) enrichUsers(ctx context.Context, users []UserSummary) []UserSummary {
	for i := range users {
		users[i].UserName = c.lookupUserName(ctx, users[i].Source, users[i].UserKey, users[i].LastConversation)
	}
	return users
}

func (c *Collector) enrichSessions(ctx context.Context, sessions []SessionView) []SessionView {
	for i := range sessions {
		sessions[i].UserName = c.lookupUserName(ctx, sessions[i].Source, sessions[i].UserKey, sessions[i].ConversationKey)
	}
	return sessions
}

func (c *Collector) lookupUserName(ctx context.Context, source, userKey, conversationKey string) string {
	if c == nil || c.userResolver == nil {
		return ""
	}
	return strings.TrimSpace(c.userResolver.Resolve(ctx, source, userKey, conversationKey))
}

func userLookupKey(source, userKey, conversationKey string) string {
	return strings.Join([]string{
		strings.TrimSpace(source),
		strings.TrimSpace(userKey),
		strings.TrimSpace(conversationKey),
	}, "\x00")
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

func normalizePagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	switch {
	case pageSize <= 0:
		pageSize = 50
	case pageSize > 200:
		pageSize = 200
	}
	return page, pageSize
}

func paginateBounds(total, page, pageSize int) (int, int, int) {
	if total == 0 {
		return 0, 0, 0
	}
	totalPages := (total + pageSize - 1) / pageSize
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end, totalPages
}

func parseDateQuery(value string, inclusiveEnd bool) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}
	}
	if inclusiveEnd {
		return t.Add(24*time.Hour - time.Nanosecond).UTC()
	}
	return t.UTC()
}

func parseIntQuery(value string, defaultVal int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return defaultVal
	}
	return n
}
