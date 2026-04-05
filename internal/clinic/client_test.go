package clinic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildWhereClauseIncludesPartitionsAndFilters(t *testing.T) {
	spec := LinkSpec{
		ClusterID: "123",
		StartTime: time.Date(2026, 3, 20, 23, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 3, 21, 1, 0, 0, 0, time.UTC),
		Digest:    "digest-1",
		Database:  "app",
		Instance:  "tidb-0",
	}

	where := buildWhereClause(spec)
	wantSnippets := []string{
		`date IN ('20260320','20260321')`,
		`digest = 'digest-1'`,
		`db = 'app'`,
		`instance = 'tidb-0'`,
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(where, snippet) {
			t.Fatalf("where clause missing %q: %s", snippet, where)
		}
	}
}

func TestBuildDetailRowsSQLUsesMySQLStandardIdentifiers(t *testing.T) {
	spec := LinkSpec{
		ClusterID: "123",
		StartTime: time.Date(2026, 4, 5, 8, 17, 27, 0, time.UTC),
		EndTime:   time.Date(2026, 4, 5, 10, 17, 27, 0, time.UTC),
		Digest:    "digest-1",
		Database:  "app",
		IsDetail:  true,
	}

	sql := buildDetailRowsSQL(spec)
	wantSnippets := []string{
		"FROM clinic_data_proxy.slow_query_logs",
		"date IN ('20260405')",
		"digest = 'digest-1'",
		"db = 'app'",
		"ORDER BY time DESC, query_time DESC",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(sql, snippet) {
			t.Fatalf("detail SQL missing %q:\n%s", snippet, sql)
		}
	}
	if strings.Contains(sql, `"clinic_data_proxy"."slow_query_logs"`) {
		t.Fatalf("detail SQL should not use quoted identifiers:\n%s", sql)
	}
}

func TestFetchSlowQueryContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clinic/api/v1/dashboard/clusters":
			if _, err := io.WriteString(w, `{"items":[{"clusterID":"123","clusterName":"prod-a","tenantName":"Acme","clusterDeployTypeV2":"premium"}]}`); err != nil {
				t.Fatalf("write clusters response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/data-proxy/query":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			sql, _ := payload["sql"].(string)
			if strings.Contains(sql, "COUNT(*) AS total_queries") {
				if _, err := io.WriteString(w, `{"columns":["total_queries","avg_query_time","max_query_time"],"rows":[[24,1.25,7.5]]}`); err != nil {
					t.Fatalf("write summary response: %v", err)
				}
				return
			}
			if _, err := io.WriteString(w, `{"columns":["digest","exec_count","avg_query_time","max_query_time","max_result_rows","max_mem_bytes","max_disk_bytes","sample_db","sample_instance","sample_indexes","sample_plan_digest","sample_prev_stmt","sample_plan","sample_decoded_plan","sample_binary_plan","sample_sql"],"rows":[["digest-1",12,1.2,7.5,10,2048,0,"app","tidb-0","idx_a","plan-digest-1","set tidb_mem_quota_query=1073741824","IndexLookUp_1 root 10.00","IndexLookUp(Build) -> TableRowIDScan(Probe)","binary-plan-text","select * from t where a = 1"]]}`); err != nil {
				t.Fatalf("write digest response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("token", 5*time.Second)
	client.APIBaseURL = server.URL + "/clinic/api/v1"
	client.DataProxyBase = server.URL

	result, err := client.FetchSlowQueryContext(context.Background(), LinkSpec{
		RawURL:    "https://clinic.pingcap.com/#/slowquery?clusterId=123",
		ClusterID: "123",
		StartTime: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("FetchSlowQueryContext returned error: %v", err)
	}
	if result.ClusterName != "prod-a" || result.OrgName != "Acme" || result.DeployType != "premium" {
		t.Fatalf("unexpected cluster metadata: %+v", result)
	}
	if result.Summary.TotalQueries != 24 || len(result.TopDigests) != 1 {
		t.Fatalf("unexpected query result: %+v", result)
	}
	if result.TopDigests[0].PlanDigest != "plan-digest-1" || result.TopDigests[0].SamplePlan == "" || result.TopDigests[0].SampleDecodedPlan == "" || result.TopDigests[0].SampleBinaryPlan == "" || result.TopDigests[0].SamplePrevStmt == "" {
		t.Fatalf("expected plan fields in top digest: %+v", result.TopDigests[0])
	}
	if result.NoRows {
		t.Fatalf("expected NoRows=false")
	}
}

func TestFetchSlowQueryContextForDetailQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clinic/api/v1/dashboard/clusters":
			if _, err := io.WriteString(w, `{"items":[{"clusterID":"123","clusterName":"prod-a","tenantName":"Acme","clusterDeployTypeV2":"premium"}]}`); err != nil {
				t.Fatalf("write clusters response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/data-proxy/query":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			sql, _ := payload["sql"].(string)
			if !strings.Contains(sql, "ORDER BY time DESC") {
				t.Fatalf("expected detail SQL, got %s", sql)
			}
			if _, err := io.WriteString(w, `{"columns":["time","digest","plan_digest","query_time","parse_time","compile_time","cop_time","process_time","wait_time","total_keys","process_keys","result_rows","mem_max","disk_max","db","instance","index_names","prev_stmt","plan","decoded_plan","binary_plan","query"],"rows":[[1773973859.727374,"digest-1","plan-digest-1",7.5,0.1,0.2,2.5,1.5,0.3,1000,800,10,2048,0,"app","tidb-0","idx_a","begin","IndexLookUp_1 root 10.00","IndexLookUp(Build) -> TableRowIDScan(Probe)","binary-plan-text","select * from t where a = 1"]]}`); err != nil {
				t.Fatalf("write detail response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("token", 5*time.Second)
	client.APIBaseURL = server.URL + "/clinic/api/v1"
	client.DataProxyBase = server.URL

	result, err := client.FetchSlowQueryContext(context.Background(), LinkSpec{
		RawURL:     "https://clinic.pingcap.com/#/slow_query/detail?clusterId=123&digest=digest-1",
		ClusterID:  "123",
		StartTime:  time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		EndTime:    time.Date(2026, 3, 20, 10, 10, 0, 0, time.UTC),
		Digest:     "digest-1",
		IsDetail:   true,
		AnchorTime: time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("FetchSlowQueryContext returned error: %v", err)
	}
	if !result.IsDetail {
		t.Fatalf("expected detail mode")
	}
	if len(result.DetailRows) != 1 || len(result.TopDigests) != 0 {
		t.Fatalf("unexpected detail query result: %+v", result)
	}
	if result.DetailRows[0].PlanDigest != "plan-digest-1" || result.DetailRows[0].Plan == "" || result.DetailRows[0].DecodedPlan == "" || result.DetailRows[0].BinaryPlan == "" || result.DetailRows[0].PrevStmt != "begin" {
		t.Fatalf("expected plan fields in detail row: %+v", result.DetailRows[0])
	}
	if result.Summary.TotalQueries != 1 || result.Summary.UniqueDigests != 1 {
		t.Fatalf("unexpected detail summary: %+v", result.Summary)
	}
}

func TestFetchSlowQueryContextFallsBackToStatementPlans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clinic/api/v1/dashboard/clusters":
			if _, err := io.WriteString(w, `{"items":[{"clusterID":"123","clusterName":"prod-a","tenantName":"Acme","clusterDeployTypeV2":"premium"}]}`); err != nil {
				t.Fatalf("write clusters response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/data-proxy/query":
			if _, err := io.WriteString(w, `{"columns":["time","digest","plan_digest","query_time","parse_time","compile_time","cop_time","process_time","wait_time","total_keys","process_keys","result_rows","mem_max","disk_max","db","instance","index_names","prev_stmt","plan","decoded_plan","binary_plan","query"],"rows":[]}`); err != nil {
				t.Fatalf("write empty detail response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("token", 5*time.Second)
	client.APIBaseURL = server.URL + "/clinic/api/v1"
	client.DataProxyBase = server.URL
	client.StatementPlanFetcher = func(context.Context, LinkSpec) ([]StatementPlanRow, error) {
		return []StatementPlanRow{{
			Digest:         "digest-1",
			DigestText:     "set names `utf8mb4`",
			StatementType:  "Set",
			Database:       "app",
			ExecutionCount: 10825,
			SumLatencySec:  1.028727611,
			AvgLatencySec:  0.000095032,
			MaxLatencySec:  0.000304185,
			LastSeen:       time.Date(2026, 4, 5, 10, 29, 35, 0, time.UTC),
		}}, nil
	}

	result, err := client.FetchSlowQueryContext(context.Background(), LinkSpec{
		RawURL:            "https://clinic.pingcap.com/portal/dashboard/cloud/ngm.html?provider=test&region=test&orgId=1&clusterId=123&deployType=premium#/statement/detail?query=%7B%22digest%22%3A%22digest-1%22%2C%22schema%22%3A%22app%22%2C%22beginTime%22%3A1774222540%2C%22endTime%22%3A1774229740%7D",
		ClusterID:         "123",
		StartTime:         time.Date(2026, 4, 5, 8, 0, 0, 0, time.UTC),
		EndTime:           time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
		Digest:            "digest-1",
		Database:          "app",
		IsDetail:          true,
		IsStatementDetail: true,
	})
	if err != nil {
		t.Fatalf("FetchSlowQueryContext returned error: %v", err)
	}
	if len(result.StatementPlans) != 1 {
		t.Fatalf("expected statement-plan fallback, got %+v", result)
	}
	if result.StatementPlans[0].DigestText != "set names `utf8mb4`" {
		t.Fatalf("unexpected statement digest text: %+v", result.StatementPlans[0])
	}
	if result.Summary.TotalQueries != 10825 || result.Summary.MaxQueryTime <= 0 {
		t.Fatalf("unexpected derived summary: %+v", result.Summary)
	}
	if result.NoRows {
		t.Fatalf("expected NoRows=false when statement fallback succeeds")
	}
}

func TestDoJSONReturnsAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := io.WriteString(w, `{"error":"unauthorized"}`); err != nil {
			t.Fatalf("write auth failure response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient("bad-token", 5*time.Second)
	client.APIBaseURL = server.URL

	var out map[string]any
	err := client.doJSON(context.Background(), http.MethodGet, server.URL, nil, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("expected auth failure, got %v", err)
	}
}
