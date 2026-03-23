package clinic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
)

func TestPrefetcherDisabledKeepsRuntimeContext(t *testing.T) {
	prefetcher, err := NewPrefetcher(&config.Config{ClinicStoreDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	runtimeCtx := codex.RuntimeContext{
		Attachment: codex.AttachmentContext{RootDir: "/tmp/user-a"},
	}

	enriched, err := prefetcher.Enrich(context.Background(), "user-a", "no link here", runtimeCtx)
	if err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}
	if enriched.RuntimeContext.Attachment.RootDir != "/tmp/user-a" || enriched.RuntimeContext.Clinic != nil {
		t.Fatalf("unexpected runtime context: %+v", enriched.RuntimeContext)
	}
}

func TestPrefetcherReturnsUserErrorWhenAPIKeyMissing(t *testing.T) {
	prefetcher, err := NewPrefetcher(&config.Config{
		ClinicEnableAutoSlowQuery: true,
		ClinicHTTPTimeoutSec:      5,
		ClinicStoreDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}

	_, err = prefetcher.Enrich(context.Background(), "user-a", "https://clinic.pingcap.com/#/slowquery?clusterId=123&startTime=2026-03-20T01:02:03Z&endTime=2026-03-20T02:02:03Z", codex.RuntimeContext{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := UserFacingMessage(err); got == "" {
		t.Fatalf("expected user-facing error, got %v", err)
	}
}

func TestClassifyClinicFetchErrorRateLimit(t *testing.T) {
	err := classifyClinicFetchError(errors.New("Clinic API returned status 429: too many requests"))
	if got := UserFacingMessage(err); !strings.Contains(got, "rate-limiting") {
		t.Fatalf("user-facing message = %q", got)
	}
}

func TestBuildIntroReplyUnsavedDoesNotClaimSaved(t *testing.T) {
	reply := buildIntroReply(codex.RuntimeContext{
		Clinic: &codex.ClinicContext{
			ClusterID: "123",
			Summary: codex.ClinicSummary{
				TotalQueries: 1,
			},
		},
	}, false)
	if !strings.Contains(reply, "couldn't save it locally") {
		t.Fatalf("unexpected intro reply: %q", reply)
	}
	if strings.Contains(reply, "Saved Entry:") {
		t.Fatalf("unsaved intro should not claim a saved entry: %q", reply)
	}
}

func TestShowcaseClinicUserFacingMessages(t *testing.T) {
	prefetcher, err := NewPrefetcher(&config.Config{
		ClinicEnableAutoSlowQuery: true,
		ClinicHTTPTimeoutSec:      5,
		ClinicStoreDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}

	_, err = prefetcher.Enrich(
		context.Background(),
		"user-a",
		"https://clinic.pingcap.com/#/slowquery?clusterId=123&startTime=2026-03-20T01:02:03Z&endTime=2026-03-20T02:02:03Z",
		codex.RuntimeContext{},
	)
	if got := UserFacingMessage(err); got != "Clinic slow query auto-analysis is enabled, but `CLINIC_API_KEY` is not configured." {
		t.Fatalf("missing_api_key message = %q", got)
	} else {
		t.Logf("missing_api_key => %s", got)
	}

	testCases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "auth",
			err:  errors.New("auth failed: invalid api key"),
			want: "Clinic API authentication failed. Check `CLINIC_API_KEY` and verify the key can access clinic.pingcap.com.",
		},
		{
			name: "rate_limit",
			err:  errors.New("Clinic API returned status 429: too many requests"),
			want: "Clinic is rate-limiting requests right now. Please retry in a moment.",
		},
		{
			name: "timeout",
			err:  context.DeadlineExceeded,
			want: "Clinic slow query fetch timed out. Please retry.",
		},
		{
			name: "network",
			err:  errors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
			want: "Clinic could not be reached because of a network problem. Please retry.",
		},
		{
			name: "unavailable",
			err:  errors.New("Clinic API returned status 503"),
			want: "Clinic is temporarily unavailable. Please retry.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := UserFacingMessage(classifyClinicFetchError(tc.err))
			if got != tc.want {
				t.Fatalf("user-facing message = %q, want %q", got, tc.want)
			}
			t.Logf("%s => %s", tc.name, got)
		})
	}
}

func TestPrefetcherLoadsLatestStoredClinicContextWithoutNewLink(t *testing.T) {
	prefetcher, err := NewPrefetcher(&config.Config{
		ClinicEnableAutoSlowQuery: true,
		ClinicStoreDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	analysis := &AnalysisContext{
		SourceURL:   "https://clinic.pingcap.com/#/slowquery?clusterId=123",
		ClusterID:   "123",
		ClusterName: "prod-a",
		StartTime:   time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		Digest:      "digest-1",
		IsDetail:    true,
		Summary: Summary{
			TotalQueries: 1,
			AvgQueryTime: 1.2,
			MaxQueryTime: 1.2,
		},
		DetailRows: []SlowQueryDetailRow{{
			TimeUnix:  1774000800,
			Digest:    "digest-1",
			QueryTime: 1.2,
			Query:     "select * from t",
		}},
	}
	if err := prefetcher.saveAnalysis("user-a", analysis); err != nil {
		t.Fatalf("saveAnalysis: %v", err)
	}

	enriched, err := prefetcher.Enrich(context.Background(), "user-a", "what should I tune next?", codex.RuntimeContext{})
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if enriched.RuntimeContext.Clinic == nil || enriched.RuntimeContext.Clinic.ClusterID != "123" || enriched.RuntimeContext.Clinic.Digest != "digest-1" {
		t.Fatalf("unexpected clinic context: %+v", enriched.RuntimeContext.Clinic)
	}
	if enriched.RuntimeContext.ClinicLibrary == nil || enriched.RuntimeContext.ClinicLibrary.ActiveItemName == "" || len(enriched.RuntimeContext.ClinicLibrary.Items) != 1 {
		t.Fatalf("unexpected clinic library context: %+v", enriched.RuntimeContext.ClinicLibrary)
	}
	if enriched.IntroReply != "" {
		t.Fatalf("expected no intro reply for follow-up turn, got %q", enriched.IntroReply)
	}
}

func TestPrefetcherReturnsIntroReplyForNewClinicLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clinic/api/v1/dashboard/clusters":
			io.WriteString(w, `{"items":[{"clusterID":"123","clusterName":"prod-a","tenantName":"Acme","clusterDeployTypeV2":"premium"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/data-proxy/query":
			io.WriteString(w, `{"columns":["time","digest","plan_digest","query_time","parse_time","compile_time","cop_time","process_time","wait_time","total_keys","process_keys","result_rows","mem_max","disk_max","db","instance","index_names","prev_stmt","plan","decoded_plan","binary_plan","query"],"rows":[[1773973859.727374,"digest-1","plan-digest-1",7.5,0.1,0.2,2.5,1.5,0.3,1000,800,10,2048,0,"app","tidb-0","idx_a","begin","IndexLookUp_1 root 10.00","IndexLookUp(Build)","binary-plan-text","select * from t where a = 1"]]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	prefetcher, err := NewPrefetcher(&config.Config{
		ClinicEnableAutoSlowQuery: true,
		ClinicHTTPTimeoutSec:      5,
		ClinicAPIKey:              "token",
		ClinicStoreDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	prefetcher.client.APIBaseURL = server.URL + "/clinic/api/v1"
	prefetcher.client.DataProxyBase = server.URL

	enriched, err := prefetcher.Enrich(context.Background(), "user-a", "please inspect https://clinic.pingcap.com/#/slow_query/detail?clusterId=123&digest=digest-1&timestamp=1773973859.727374", codex.RuntimeContext{})
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if enriched.RuntimeContext.Clinic == nil {
		t.Fatalf("expected clinic context")
	}
	if !strings.Contains(enriched.IntroReply, "I saved this Clinic slow query snapshot locally.") {
		t.Fatalf("unexpected intro reply: %q", enriched.IntroReply)
	}
	if !strings.Contains(enriched.IntroReply, "Tell me what you want to do next") {
		t.Fatalf("intro reply should ask next action: %q", enriched.IntroReply)
	}
}

func TestPrefetcherSkipsIntroReplyWhenQuestionIncludesAnalysisIntent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clinic/api/v1/dashboard/clusters":
			io.WriteString(w, `{"items":[{"clusterID":"123","clusterName":"prod-a","tenantName":"Acme","clusterDeployTypeV2":"premium"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/data-proxy/query":
			io.WriteString(w, `{"columns":["time","digest","plan_digest","query_time","parse_time","compile_time","cop_time","process_time","wait_time","total_keys","process_keys","result_rows","mem_max","disk_max","db","instance","index_names","prev_stmt","plan","decoded_plan","binary_plan","query"],"rows":[[1773973859.727374,"digest-1","plan-digest-1",7.5,0.1,0.2,2.5,1.5,0.3,1000,800,10,2048,0,"app","tidb-0","idx_a","begin","IndexLookUp_1 root 10.00","IndexLookUp(Build)","binary-plan-text","select * from t where a = 1"]]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	prefetcher, err := NewPrefetcher(&config.Config{
		ClinicEnableAutoSlowQuery: true,
		ClinicHTTPTimeoutSec:      5,
		ClinicAPIKey:              "token",
		ClinicStoreDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	prefetcher.client.APIBaseURL = server.URL + "/clinic/api/v1"
	prefetcher.client.DataProxyBase = server.URL

	enriched, err := prefetcher.Enrich(context.Background(), "user-a", "show me the original decoded_plan of this slow query https://clinic.pingcap.com/#/slow_query/detail?clusterId=123&digest=digest-1&timestamp=1773973859.727374", codex.RuntimeContext{})
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if enriched.RuntimeContext.Clinic == nil {
		t.Fatalf("expected clinic context")
	}
	if enriched.IntroReply != "" {
		t.Fatalf("expected direct analysis path without intro reply, got %q", enriched.IntroReply)
	}
}

func TestPartitionDatesSingleDay(t *testing.T) {
	got := partitionDates(
		time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
	)
	if len(got) != 1 || got[0] != "20260320" {
		t.Fatalf("partitionDates = %v", got)
	}
}
