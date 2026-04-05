//go:build darwin

package clinic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

func newChromeStatementPlanFetcher(timeout time.Duration) StatementPlanFetcher {
	return func(ctx context.Context, spec LinkSpec) ([]StatementPlanRow, error) {
		return fetchStatementPlansFromChrome(ctx, spec)
	}
}

func fetchStatementPlansFromChrome(ctx context.Context, spec LinkSpec) ([]StatementPlanRow, error) {
	if strings.TrimSpace(spec.Provider) == "" || strings.TrimSpace(spec.Region) == "" || strings.TrimSpace(spec.OrgID) == "" || strings.TrimSpace(spec.ClusterID) == "" {
		return nil, nil
	}

	jsFile, err := os.CreateTemp("", "askplanner-clinic-*.js")
	if err != nil {
		return nil, fmt.Errorf("create statement-detail JS temp file: %w", err)
	}
	defer func() { _ = os.Remove(jsFile.Name()) }()

	appleScriptFile, err := os.CreateTemp("", "askplanner-clinic-*.applescript")
	if err != nil {
		return nil, fmt.Errorf("create statement-detail AppleScript temp file: %w", err)
	}
	defer func() { _ = os.Remove(appleScriptFile.Name()) }()

	queryPath := fmt.Sprintf(
		"/ngm/api/v1/statements/plans?begin_time=%d&end_time=%d&digest=%s&schema_name=%s",
		spec.StartTime.UTC().Unix(),
		spec.EndTime.UTC().Unix(),
		url.QueryEscape(spec.Digest),
		url.QueryEscape(spec.Database),
	)

	js := fmt.Sprintf(`(() => {
  const headers = {
    'x-csrf-token': localStorage.getItem('clinic.auth.csrf_token') || '',
    'x-provider': %q,
    'x-region': %q,
    'x-org-id': %q,
    'x-project-id': %q,
    'x-cluster-id': %q,
    'x-deploy-type': %q,
    'content-type': 'application/json'
  };
  const xhr = new XMLHttpRequest();
  xhr.open('GET', %q, false);
  Object.entries(headers).forEach(([k, v]) => xhr.setRequestHeader(k, v));
  xhr.send(null);
  return JSON.stringify({status: xhr.status, text: xhr.responseText});
})()`,
		spec.Provider,
		spec.Region,
		spec.OrgID,
		spec.ProjectID,
		spec.ClusterID,
		spec.DeployType,
		queryPath,
	)
	if _, err := jsFile.WriteString(js); err != nil {
		return nil, fmt.Errorf("write statement-detail JS temp file: %w", err)
	}
	if err := jsFile.Close(); err != nil {
		return nil, fmt.Errorf("close statement-detail JS temp file: %w", err)
	}

	appleScript := `on run argv
  set jsPath to item 1 of argv
  set jsText to read POSIX file jsPath
  tell application "Google Chrome"
    execute active tab of front window javascript jsText
  end tell
end run
`
	if _, err := appleScriptFile.WriteString(appleScript); err != nil {
		return nil, fmt.Errorf("write statement-detail AppleScript temp file: %w", err)
	}
	if err := appleScriptFile.Close(); err != nil {
		return nil, fmt.Errorf("close statement-detail AppleScript temp file: %w", err)
	}

	cmd := exec.CommandContext(ctx, "osascript", appleScriptFile.Name(), jsFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fetch Clinic statement detail from Chrome session: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var envelope struct {
		Status int    `json:"status"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return nil, fmt.Errorf("decode Chrome statement-detail response envelope: %w", err)
	}
	if envelope.Status != 200 {
		return nil, fmt.Errorf("clinic statement-detail browser fetch returned status %d: %s", envelope.Status, strings.TrimSpace(envelope.Text))
	}

	var rawRows []struct {
		AvgLatency    float64 `json:"avg_latency"`
		Digest        string  `json:"digest"`
		DigestText    string  `json:"digest_text"`
		ExecCount     int64   `json:"exec_count"`
		FirstSeen     int64   `json:"first_seen"`
		LastSeen      int64   `json:"last_seen"`
		MaxLatency    float64 `json:"max_latency"`
		PlanDigest    string  `json:"plan_digest"`
		PlanHint      string  `json:"plan_hint"`
		PlanInBinding int64   `json:"plan_in_binding"`
		SchemaName    string  `json:"schema_name"`
		StmtType      string  `json:"stmt_type"`
		SumLatency    float64 `json:"sum_latency"`
	}
	if err := json.Unmarshal([]byte(envelope.Text), &rawRows); err != nil {
		return nil, fmt.Errorf("decode Chrome statement-detail rows: %w", err)
	}

	rows := make([]StatementPlanRow, 0, len(rawRows))
	for _, row := range rawRows {
		rows = append(rows, StatementPlanRow{
			Digest:         strings.TrimSpace(row.Digest),
			PlanDigest:     strings.TrimSpace(row.PlanDigest),
			DigestText:     strings.TrimSpace(row.DigestText),
			StatementType:  strings.TrimSpace(row.StmtType),
			Database:       strings.TrimSpace(row.SchemaName),
			ExecutionCount: row.ExecCount,
			SumLatencySec:  row.SumLatency / float64(time.Second),
			AvgLatencySec:  row.AvgLatency / float64(time.Second),
			MaxLatencySec:  row.MaxLatency / float64(time.Second),
			PlanHint:       strings.TrimSpace(row.PlanHint),
			PlanInBinding:  row.PlanInBinding != 0,
			FirstSeen:      unixTime(row.FirstSeen),
			LastSeen:       unixTime(row.LastSeen),
		})
	}
	return rows, nil
}

func unixTime(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}
