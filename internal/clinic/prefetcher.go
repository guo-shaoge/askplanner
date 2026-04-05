package clinic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"lab/askplanner/internal/clinicstore"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/usererr"
)

type Prefetcher struct {
	enabled bool
	client  *Client
	store   *clinicstore.Manager
}

const promptClinicLibraryLimit = 10

var genericClinicLinkPhrases = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bcan you see (this )?slow query link\b`),
	regexp.MustCompile(`(?i)\bplease inspect\b`),
	regexp.MustCompile(`(?i)\bcheck (this )?(clinic|slow query) link\b`),
	regexp.MustCompile(`(?i)\bsee (this )?(clinic|slow query) link\b`),
	regexp.MustCompile(`请帮我?(看|瞅)一下`),
	regexp.MustCompile(`帮我?(看|瞅)下`),
	regexp.MustCompile(`看下这个`),
	regexp.MustCompile(`看看这个`),
	regexp.MustCompile(`这个(慢查询|链接)`),
}

var clinicAnalysisIntentKeywords = []string{
	"root cause",
	"analy",
	"bottleneck",
	"optimiz",
	"tune",
	"why",
	"how",
	"what happened",
	"explain",
	"decoded_plan",
	"decoded plan",
	"binary_plan",
	"binary plan",
	"plan",
	"sql",
	"summary",
	"suggestion",
	"recommendation",
	"原因",
	"根因",
	"分析",
	"瓶颈",
	"优化",
	"调优",
	"建议",
	"执行计划",
	"慢在哪里",
	"为什么",
	"怎么看",
	"解释",
	"摘要",
}

type EnrichResult struct {
	RuntimeContext codex.RuntimeContext
	IntroReply     string
	Warning        string
}

func NewPrefetcher(cfg *config.Config) (*Prefetcher, error) {
	timeout := time.Duration(cfg.ClinicHTTPTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	var store *clinicstore.Manager
	var err error
	if cfg.ClinicEnableAutoSlowQuery {
		store, err = clinicstore.NewManager(cfg.ClinicStoreDir, cfg.ClinicStoreMaxItems)
		if err != nil {
			return nil, err
		}
	}
	return &Prefetcher{
		enabled: cfg.ClinicEnableAutoSlowQuery,
		client:  NewClient(cfg.ClinicAPIKey, timeout),
		store:   store,
	}, nil
}

func (p *Prefetcher) Enrich(ctx context.Context, userKey, question string, runtime codex.RuntimeContext) (EnrichResult, error) {
	if !p.enabled {
		return EnrichResult{RuntimeContext: runtime}, nil
	}

	previousURL := ""
	runtime, loadErr := p.attachLatestStored(userKey, runtime)
	if loadErr != nil {
		return EnrichResult{RuntimeContext: runtime}, usererr.WrapLocalStorage("Agent couldn't load the saved Clinic snapshots from local storage. Please retry.", loadErr)
	}
	if runtime.Clinic != nil {
		previousURL = strings.TrimSpace(runtime.Clinic.SourceURL)
	}

	spec, matched, err := ParseSlowQueryLink(question)
	if err != nil {
		log.Printf("[clinic] parse failed for potential slow query link: %v", err)
		return EnrichResult{RuntimeContext: runtime}, usererr.Wrap(usererr.KindInvalidInput, "Agent detected a Clinic slow query link but could not parse its cluster ID and time range. Please send the full share link from Clinic Slow Query.", err)
	}
	if !matched {
		return EnrichResult{RuntimeContext: runtime}, nil
	}
	log.Printf("[clinic] parsed slow query link: cluster_id=%s start=%s end=%s digest=%s db=%s instance=%s url=%s",
		spec.ClusterID,
		spec.StartTime.UTC().Format(time.RFC3339),
		spec.EndTime.UTC().Format(time.RFC3339),
		spec.Digest,
		spec.Database,
		spec.Instance,
		spec.RawURL,
	)
	if strings.TrimSpace(p.client.APIKey) == "" {
		log.Printf("[clinic] prefetch skipped: CLINIC_API_KEY is empty for cluster_id=%s", spec.ClusterID)
		return EnrichResult{RuntimeContext: runtime}, usererr.New(usererr.KindConfig, "Clinic slow query auto-analysis is enabled, but `CLINIC_API_KEY` is not configured.")
	}

	analysis, err := p.client.FetchSlowQueryContext(ctx, *spec)
	if err != nil {
		log.Printf("[clinic] prefetch failed for cluster_id=%s url=%s: %v", spec.ClusterID, spec.RawURL, err)
		return EnrichResult{RuntimeContext: runtime}, classifyClinicFetchError(err)
	}
	if spec.IsStatementDetail && analysis.NoRows {
		log.Printf("[clinic] statement detail prefetch returned no slow-query rows: cluster_id=%s digest=%s db=%s start=%s end=%s",
			spec.ClusterID,
			spec.Digest,
			spec.Database,
			spec.StartTime.UTC().Format(time.RFC3339),
			spec.EndTime.UTC().Format(time.RFC3339),
		)
		return EnrichResult{RuntimeContext: runtime}, usererr.New(
			usererr.KindUnavailable,
			"Agent recognized this Clinic Statement Detail link, but the relay could not map it to any slow-query samples. This page likely uses statement-summary data that the current relay cannot prefetch yet. Please send the SQL text, `EXPLAIN ANALYZE`, or a Clinic slow-query link instead.",
		)
	}
	log.Printf("[clinic] prefetch succeeded: cluster_id=%s total_queries=%d unique_digests=%d top_digests=%d",
		analysis.ClusterID,
		analysis.Summary.TotalQueries,
		analysis.Summary.UniqueDigests,
		len(analysis.TopDigests),
	)
	runtime.Clinic = toClinicRuntimeContext(analysis)

	if storeErr := p.saveAnalysis(userKey, analysis); storeErr != nil {
		log.Printf("[clinic] saved analysis fetch succeeded but local persistence failed for cluster_id=%s: %v", analysis.ClusterID, storeErr)
		if runtime.ClinicLibrary != nil {
			runtime.ClinicLibrary.ActiveItemName = ""
		}
		result := EnrichResult{
			RuntimeContext: runtime,
			Warning:        buildClinicPersistenceWarning("Clinic data was fetched, but Agent couldn't save this snapshot locally. Follow-up turns may not be able to reuse it.", storeErr),
		}
		if shouldReturnIntroReply(question, previousURL, spec) {
			result.IntroReply = buildIntroReply(runtime, false)
		}
		return result, nil
	}
	runtime, err = p.attachLatestStored(userKey, runtime)
	if err != nil {
		result := EnrichResult{
			RuntimeContext: runtime,
			Warning:        buildClinicPersistenceWarning("Clinic data was fetched, but Agent couldn't refresh the saved snapshot library locally. Follow-up turns may not be able to reuse it.", err),
		}
		if shouldReturnIntroReply(question, previousURL, spec) {
			result.IntroReply = buildIntroReply(runtime, false)
		}
		return result, nil
	}
	result := EnrichResult{RuntimeContext: runtime}
	if shouldReturnIntroReply(question, previousURL, spec) {
		result.IntroReply = buildIntroReply(runtime, true)
	}
	return result, nil
}

func shouldReturnIntroReply(question, previousURL string, spec *LinkSpec) bool {
	if spec == nil || strings.TrimSpace(spec.RawURL) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(previousURL), strings.TrimSpace(spec.RawURL)) {
		return false
	}
	return !containsClinicAnalysisIntent(question)
}

func containsClinicAnalysisIntent(question string) bool {
	withoutURLs := strings.TrimSpace(urlPattern.ReplaceAllString(question, " "))
	normalized := strings.ToLower(strings.TrimSpace(strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		"“", " ",
		"”", " ",
		"'", " ",
		"\"", " ",
		"`", " ",
		",", " ",
		".", " ",
		"!", " ",
		"?", " ",
		"？", " ",
		"，", " ",
		"。", " ",
		"；", " ",
		";", " ",
		":", " ",
		"：", " ",
		"（", " ",
		"）", " ",
		"(", " ",
		")", " ",
	).Replace(withoutURLs)))
	if normalized == "" {
		return false
	}
	for _, pattern := range genericClinicLinkPhrases {
		normalized = strings.TrimSpace(pattern.ReplaceAllString(normalized, " "))
	}
	if normalized == "" {
		return false
	}
	if strings.Contains(question, "?") || strings.Contains(question, "？") {
		return true
	}
	for _, keyword := range clinicAnalysisIntentKeywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func (p *Prefetcher) saveAnalysis(userKey string, analysis *AnalysisContext) error {
	if strings.TrimSpace(userKey) == "" || analysis == nil {
		return nil
	}
	payload, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal Clinic analysis: %w", err)
	}
	_, err = p.store.Save(clinicstore.SaveRequest{
		UserKey:         userKey,
		AnalysisJSON:    payload,
		SummaryMarkdown: BuildStoredSummary(analysis),
		Metadata: clinicstore.Metadata{
			SourceURL:   analysis.SourceURL,
			ClusterID:   analysis.ClusterID,
			ClusterName: analysis.ClusterName,
			OrgName:     analysis.OrgName,
			DeployType:  analysis.DeployType,
			StartTime:   analysis.StartTime.UTC(),
			EndTime:     analysis.EndTime.UTC(),
			Digest:      analysis.Digest,
			Database:    analysis.Database,
			Instance:    analysis.Instance,
			IsDetail:    analysis.IsDetail,
			SavedAt:     time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("save Clinic analysis: %w", err)
	}
	return nil
}

func (p *Prefetcher) attachLatestStored(userKey string, runtime codex.RuntimeContext) (codex.RuntimeContext, error) {
	if strings.TrimSpace(userKey) == "" {
		return runtime, nil
	}

	library, err := p.store.Snapshot(userKey)
	if err != nil {
		return runtime, fmt.Errorf("load Clinic library snapshot: %w", err)
	}
	runtime.ClinicLibrary = toClinicLibraryContext(library)

	entry, ok, err := p.store.Latest(userKey)
	if err != nil {
		return runtime, fmt.Errorf("load latest Clinic entry: %w", err)
	}
	if !ok {
		runtime.Clinic = nil
		return runtime, nil
	}
	runtime.ClinicLibrary.ActiveItemName = entry.Item.Name

	var analysis AnalysisContext
	if err := json.Unmarshal(entry.AnalysisJSON, &analysis); err != nil {
		return runtime, fmt.Errorf("decode latest Clinic entry %s: %w", entry.Item.Name, err)
	}
	runtime.Clinic = toClinicRuntimeContext(&analysis)
	return runtime, nil
}

func toClinicLibraryContext(library clinicstore.Library) *codex.ClinicLibraryContext {
	if strings.TrimSpace(library.RootDir) == "" {
		return nil
	}
	items := library.Items
	if len(items) > promptClinicLibraryLimit {
		items = items[:promptClinicLibraryLimit]
	}
	ctxItems := make([]codex.ClinicLibraryItem, 0, len(items))
	for _, item := range items {
		ctxItems = append(ctxItems, codex.ClinicLibraryItem{
			Name:        item.Name,
			SavedAt:     item.SavedAt,
			ClusterID:   item.ClusterID,
			ClusterName: item.ClusterName,
			Digest:      item.Digest,
			Database:    item.Database,
			Instance:    item.Instance,
			IsDetail:    item.IsDetail,
		})
	}
	return &codex.ClinicLibraryContext{
		RootDir: library.RootDir,
		Items:   ctxItems,
	}
}

func toClinicRuntimeContext(analysis *AnalysisContext) *codex.ClinicContext {
	if analysis == nil {
		return nil
	}
	ctx := &codex.ClinicContext{
		SourceURL:   analysis.SourceURL,
		ClusterID:   analysis.ClusterID,
		ClusterName: analysis.ClusterName,
		OrgName:     analysis.OrgName,
		DeployType:  analysis.DeployType,
		StartTime:   analysis.StartTime,
		EndTime:     analysis.EndTime,
		Digest:      analysis.Digest,
		Database:    analysis.Database,
		Instance:    analysis.Instance,
		IsDetail:    analysis.IsDetail,
		Summary: codex.ClinicSummary{
			TotalQueries:  analysis.Summary.TotalQueries,
			UniqueDigests: analysis.Summary.UniqueDigests,
			AvgQueryTime:  analysis.Summary.AvgQueryTime,
			MaxQueryTime:  analysis.Summary.MaxQueryTime,
		},
		NoRows: analysis.NoRows,
	}
	for _, row := range analysis.DetailRows {
		ctx.DetailRows = append(ctx.DetailRows, codex.ClinicDetailRow{
			TimeUnix:    row.TimeUnix,
			Digest:      row.Digest,
			PlanDigest:  row.PlanDigest,
			QueryTime:   row.QueryTime,
			ParseTime:   row.ParseTime,
			CompileTime: row.CompileTime,
			CopTime:     row.CopTime,
			ProcessTime: row.ProcessTime,
			WaitTime:    row.WaitTime,
			TotalKeys:   row.TotalKeys,
			ProcessKeys: row.ProcessKeys,
			ResultRows:  row.ResultRows,
			MemBytes:    row.MemBytes,
			DiskBytes:   row.DiskBytes,
			Database:    row.Database,
			Instance:    row.Instance,
			Indexes:     row.Indexes,
			PrevStmt:    row.PrevStmt,
			Plan:        row.Plan,
			DecodedPlan: row.DecodedPlan,
			BinaryPlan:  row.BinaryPlan,
			Query:       row.Query,
		})
	}
	for _, item := range analysis.TopDigests {
		ctx.TopDigests = append(ctx.TopDigests, codex.ClinicDigestSummary{
			Digest:            item.Digest,
			PlanDigest:        item.PlanDigest,
			ExecutionCount:    item.ExecutionCount,
			AvgQueryTime:      item.AvgQueryTime,
			MaxQueryTime:      item.MaxQueryTime,
			MaxTotalKeys:      item.MaxTotalKeys,
			MaxProcessKeys:    item.MaxProcessKeys,
			MaxResultRows:     item.MaxResultRows,
			MaxMemBytes:       item.MaxMemBytes,
			MaxDiskBytes:      item.MaxDiskBytes,
			SampleDB:          item.SampleDB,
			SampleInstance:    item.SampleInstance,
			SampleIndexes:     item.SampleIndexes,
			SamplePrevStmt:    item.SamplePrevStmt,
			SamplePlan:        item.SamplePlan,
			SampleDecodedPlan: item.SampleDecodedPlan,
			SampleBinaryPlan:  item.SampleBinaryPlan,
			SampleSQL:         item.SampleSQL,
		})
	}
	return ctx
}

func UserFacingMessage(err error) string {
	return usererr.Message(err)
}

func buildIntroReply(runtime codex.RuntimeContext, saved bool) string {
	clinic := runtime.Clinic
	if clinic == nil {
		return ""
	}

	var sb strings.Builder
	if saved {
		sb.WriteString("Agent saved this Clinic slow query snapshot locally.\n")
	} else {
		sb.WriteString("Agent fetched this Clinic slow query snapshot for this turn, but couldn't save it locally.\n")
	}
	sb.WriteString("- Cluster: ")
	sb.WriteString(strings.TrimSpace(clinic.ClusterID))
	if clinic.ClusterName != "" {
		sb.WriteString(" (")
		sb.WriteString(strings.TrimSpace(clinic.ClusterName))
		sb.WriteByte(')')
	}
	sb.WriteByte('\n')
	if clinic.IsDetail {
		sb.WriteString("- Scope: detail\n")
	} else {
		sb.WriteString("- Scope: grouped slow-query list\n")
	}
	if !clinic.StartTime.IsZero() && !clinic.EndTime.IsZero() {
		sb.WriteString("- Time Range (UTC): ")
		sb.WriteString(clinic.StartTime.UTC().Format(time.RFC3339))
		sb.WriteString(" to ")
		sb.WriteString(clinic.EndTime.UTC().Format(time.RFC3339))
		sb.WriteByte('\n')
	}
	if clinic.Digest != "" || clinic.Database != "" || clinic.Instance != "" {
		sb.WriteString("- Filters:")
		if clinic.Digest != "" {
			sb.WriteString(" digest=")
			sb.WriteString(clinic.Digest)
		}
		if clinic.Database != "" {
			sb.WriteString(" db=")
			sb.WriteString(clinic.Database)
		}
		if clinic.Instance != "" {
			sb.WriteString(" instance=")
			sb.WriteString(clinic.Instance)
		}
		sb.WriteByte('\n')
	}
	_, _ = fmt.Fprintf(&sb, "- Summary: total_queries=%d avg_query_time_sec=%.6f max_query_time_sec=%.6f\n",
		clinic.Summary.TotalQueries,
		clinic.Summary.AvgQueryTime,
		clinic.Summary.MaxQueryTime,
	)
	if saved && runtime.ClinicLibrary != nil && runtime.ClinicLibrary.ActiveItemName != "" {
		sb.WriteString("- Saved Entry: ")
		sb.WriteString(runtime.ClinicLibrary.ActiveItemName)
		sb.WriteByte('\n')
	}
	sb.WriteString("Tell me what you want to do next with this slow query, for example: root-cause analysis, plan interpretation, bottleneck summary, or optimization suggestions.")
	return sb.String()
}

func buildClinicPersistenceWarning(message string, err error) string {
	classified := usererr.WrapLocalStorage(message, err)
	warning := usererr.OrDefault(classified, message)
	if !strings.Contains(strings.ToLower(warning), "follow-up") {
		warning += " Follow-up turns may not be able to reuse it."
	}
	return warning
}

func classifyClinicFetchError(err error) error {
	if msg := usererr.Message(err); msg != "" {
		return err
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "auth failed"):
		return usererr.Wrap(usererr.KindAuth, "Clinic API authentication failed. Check `CLINIC_API_KEY` and verify the key can access clinic.pingcap.com.", err)
	case strings.Contains(lower, "cluster not found"):
		return usererr.Wrap(usererr.KindInvalidInput, "The Clinic link points to a cluster that is not accessible with the current API key.", err)
	case containsClinicErrorText(lower, "status 429", "too many requests", "rate limit"):
		return usererr.Wrap(usererr.KindRateLimit, "Clinic is rate-limiting requests right now. Please retry in a moment.", err)
	case errors.Is(err, context.DeadlineExceeded), hasNetTimeout(err), containsClinicErrorText(lower, "timeout", "timed out", "deadline exceeded"):
		return usererr.Wrap(usererr.KindTimeout, "Clinic slow query fetch timed out. Please retry.", err)
	case hasNetworkError(err), containsClinicErrorText(lower, "dial tcp", "connection refused", "connection reset", "no such host", "network is unreachable", "temporary failure in name resolution"):
		return usererr.Wrap(usererr.KindNetwork, "Clinic could not be reached because of a network problem. Please retry.", err)
	case containsClinicErrorText(lower, "datasource_not_configured", "does not have slow_query data source configured", "specify datasource manually"):
		return usererr.Wrap(usererr.KindUnavailable, "This Clinic cluster does not expose a usable slow query data source through the relay API yet, so askplanner cannot prefetch this slow query link.", err)
	case containsClinicErrorText(lower, "status 500", "status 502", "status 503", "status 504"):
		return usererr.Wrap(usererr.KindUnavailable, "Clinic is temporarily unavailable. Please retry.", err)
	default:
		return usererr.Wrap(usererr.KindUnavailable, "Clinic slow query prefetch failed. Please retry.", err)
	}
}

func containsClinicErrorText(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func hasNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func hasNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
