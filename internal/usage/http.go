package usage

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

type Server struct {
	collector *Collector
	page      *template.Template
}

func NewServer(collector *Collector) (*Server, error) {
	page, err := template.New("usage").Parse(pageHTML)
	if err != nil {
		return nil, fmt.Errorf("parse dashboard template: %w", err)
	}
	return &Server{
		collector: collector,
		page:      page,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/usage", s.handleUsage)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.page.Execute(w, nil)
}

func (s *Server) handleUsage(w http.ResponseWriter, _ *http.Request) {
	snapshot, err := s.collector.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(snapshot)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>askplanner usage</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f1e8;
      --panel: rgba(255,255,255,0.78);
      --panel-strong: rgba(255,255,255,0.92);
      --ink: #18222f;
      --muted: #5f6b77;
      --line: rgba(24,34,47,0.08);
      --accent: #bb4d00;
      --accent-soft: #ffe2cf;
      --warn: #a22d29;
      --shadow: 0 24px 80px rgba(47, 34, 19, 0.12);
      --radius: 20px;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(235, 141, 79, 0.32), transparent 30%),
        radial-gradient(circle at top right, rgba(30, 115, 153, 0.18), transparent 28%),
        linear-gradient(180deg, #fbf8f1 0%, var(--bg) 100%);
      font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif;
    }
    .shell {
      max-width: 1440px;
      margin: 0 auto;
      padding: 28px 20px 40px;
    }
    .hero {
      display: grid;
      gap: 12px;
      margin-bottom: 22px;
    }
    .kicker {
      font-size: 12px;
      letter-spacing: 0.16em;
      text-transform: uppercase;
      color: var(--accent);
      font-weight: 700;
    }
    h1 {
      margin: 0;
      font-size: clamp(32px, 4vw, 64px);
      line-height: 0.96;
      font-weight: 700;
      max-width: 10ch;
    }
    .subhead {
      max-width: 70ch;
      color: var(--muted);
      font-size: 16px;
      line-height: 1.5;
      margin: 0;
    }
    .meta {
      display: flex;
      gap: 16px;
      flex-wrap: wrap;
      color: var(--muted);
      font-size: 13px;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(12, minmax(0, 1fr));
      gap: 16px;
    }
    .panel {
      grid-column: span 12;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: var(--radius);
      box-shadow: var(--shadow);
      backdrop-filter: blur(18px);
      overflow: hidden;
    }
    .panel-inner {
      padding: 18px 18px 20px;
    }
    .panel h2 {
      margin: 0 0 14px;
      font-size: 18px;
    }
    .card-strip {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: 14px;
    }
    .stat {
      background: var(--panel-strong);
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 14px;
    }
    .stat-label {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      margin-bottom: 8px;
    }
    .stat-value {
      font-size: 34px;
      line-height: 1;
      font-weight: 700;
      margin-bottom: 8px;
    }
    .stat-note {
      color: var(--muted);
      font-size: 13px;
    }
    .split-6 { grid-column: span 6; }
    .split-4 { grid-column: span 4; }
    .split-8 { grid-column: span 8; }
    .split-12 { grid-column: span 12; }
    .list {
      display: grid;
      gap: 10px;
    }
    .row {
      display: grid;
      grid-template-columns: minmax(0, 1.2fr) 84px;
      gap: 12px;
      align-items: center;
    }
    .bar-wrap {
      position: relative;
      height: 40px;
      border-radius: 12px;
      border: 1px solid var(--line);
      background: rgba(255,255,255,0.56);
      overflow: hidden;
    }
    .bar {
      position: absolute;
      inset: 0 auto 0 0;
      background: linear-gradient(90deg, #e99054, #bb4d00);
      opacity: 0.9;
    }
    .bar-label {
      position: relative;
      z-index: 1;
      display: flex;
      justify-content: space-between;
      gap: 12px;
      align-items: center;
      height: 100%;
      padding: 0 14px;
      font-size: 14px;
    }
    .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
    }
    th, td {
      text-align: left;
      padding: 10px 8px;
      border-top: 1px solid var(--line);
      vertical-align: top;
    }
    th {
      color: var(--muted);
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      border-top: none;
      padding-top: 0;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      border-radius: 999px;
      padding: 4px 9px;
      font-size: 12px;
      background: var(--accent-soft);
      color: var(--accent);
      font-weight: 700;
    }
    .error {
      color: var(--warn);
      white-space: pre-wrap;
    }
    .empty {
      color: var(--muted);
      font-style: italic;
      padding: 8px 0 4px;
    }
    @media (max-width: 980px) {
      .split-6, .split-4, .split-8 { grid-column: span 12; }
      .shell { padding: 18px 14px 28px; }
      .row { grid-template-columns: minmax(0, 1fr) 64px; }
    }
  </style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <div class="kicker">Operations view</div>
      <h1>askplanner live usage</h1>
      <p class="subhead">This page reads the local session store, workspace metadata, and recent log tail. It is near real-time rather than push-driven, and refreshes automatically every 5 seconds.</p>
      <div class="meta">
        <div id="generatedAt">loading...</div>
        <div id="refreshState">refresh every 5s</div>
      </div>
    </section>

    <section class="grid">
      <div class="panel split-12">
        <div class="panel-inner">
          <h2>Core status</h2>
          <div id="summaryCards" class="card-strip"></div>
        </div>
      </div>

      <div class="panel split-4">
        <div class="panel-inner">
          <h2>Request throughput</h2>
          <div id="requestCards" class="card-strip"></div>
        </div>
      </div>

      <div class="panel split-4">
        <div class="panel-inner">
          <h2>Session source</h2>
          <div id="sourceBreakdown" class="list"></div>
        </div>
      </div>

      <div class="panel split-4">
        <div class="panel-inner">
          <h2>Model override</h2>
          <div id="modelBreakdown" class="list"></div>
        </div>
      </div>

      <div class="panel split-6">
        <div class="panel-inner">
          <h2>Workspace refs</h2>
          <div id="repoBreakdown" class="list"></div>
        </div>
      </div>

      <div class="panel split-6">
        <div class="panel-inner">
          <h2>Recent errors</h2>
          <div id="recentErrors"></div>
        </div>
      </div>

      <div class="panel split-12">
        <div class="panel-inner">
          <h2>Recent sessions</h2>
          <div id="recentSessions"></div>
        </div>
      </div>

      <div class="panel split-12">
        <div class="panel-inner">
          <h2>Recent requests</h2>
          <div id="recentRequests"></div>
        </div>
      </div>
    </section>
  </main>

  <script>
    const summarySpec = [
      ["Total conversations", "summary.total_conversations", ""],
      ["Active 15m", "summary.active_15_min", "recently touched sessions"],
      ["Active 1h", "summary.active_1_hour", "hourly window"],
      ["Active 24h", "summary.active_24_hours", "daily window"],
      ["Resumable sessions", "summary.resumable_sessions", "session id still present"],
      ["Error sessions", "summary.error_sessions", "last error persisted in store"],
      ["Workspace users", "summary.workspace_users", "users with a workspace metadata file"],
      ["Active users 24h", "summary.active_users_24_hours", "from sessions and workspaces"]
    ];
    const requestSpec = [
      ["Requests 5m", "request_stats.requests_5_min"],
      ["Requests 1h", "request_stats.requests_1_hour"],
      ["Errors 1h", "request_stats.errors_1_hour"],
      ["P50 latency ms", "request_stats.p50_latency_ms"],
      ["P95 latency ms", "request_stats.p95_latency_ms"]
    ];

    function get(obj, path) {
      return path.split(".").reduce((acc, key) => acc == null ? acc : acc[key], obj);
    }
    function fmtNumber(v) {
      if (typeof v === "number") {
        return Number.isInteger(v) ? String(v) : String(Math.round(v));
      }
      return v == null || v === "" ? "-" : String(v);
    }
    function fmtTime(value) {
      if (!value) return "-";
      const d = new Date(value);
      return isNaN(d.getTime()) ? "-" : d.toLocaleString();
    }
    function escapeHTML(s) {
      return String(s ?? "").replace(/[&<>"']/g, ch => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"
      }[ch]));
    }
    function renderCards(el, spec, data) {
      el.innerHTML = spec.map(([label, path, note]) =>
        '<article class="stat">' +
          '<div class="stat-label">' + escapeHTML(label) + '</div>' +
          '<div class="stat-value">' + escapeHTML(fmtNumber(get(data, path))) + '</div>' +
          '<div class="stat-note">' + escapeHTML(note || "") + '</div>' +
        '</article>'
      ).join("");
    }
    function renderBreakdown(el, items) {
      if (!items || !items.length) {
        el.innerHTML = '<div class="empty">No data yet.</div>';
        return;
      }
      const max = Math.max(...items.map(item => item.value), 1);
      el.innerHTML = items.map(item => {
        const width = Math.max(8, Math.round(item.value / max * 100));
        return '<div class="row">' +
          '<div class="bar-wrap">' +
            '<div class="bar" style="width:' + width + '%"></div>' +
            '<div class="bar-label">' +
              '<span>' + escapeHTML(item.name) + '</span>' +
              '<span>' + escapeHTML(fmtNumber(item.value)) + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mono">' + escapeHTML(fmtNumber(item.value)) + '</div>' +
        '</div>';
      }).join("");
    }
    function renderRepoBreakdown(el, repos) {
      if (!repos || !repos.length) {
        el.innerHTML = '<div class="empty">No workspace metadata yet.</div>';
        return;
      }
      el.innerHTML = repos.map(repo => {
        const refs = repo.refs.length ? repo.refs.map(ref =>
          '<div style="display:flex;justify-content:space-between;gap:12px;padding:6px 0;border-top:1px solid var(--line)">' +
            '<span class="mono">' + escapeHTML(ref.name) + '</span>' +
            '<span>' + escapeHTML(fmtNumber(ref.value)) + '</span>' +
          '</div>'
        ).join("") : '<div class="empty">No refs.</div>';
        return '<section class="stat" style="margin-bottom:12px">' +
          '<div style="display:flex;justify-content:space-between;gap:12px;align-items:center;margin-bottom:10px">' +
            '<strong>' + escapeHTML(repo.name) + '</strong>' +
            '<span class="pill">' + escapeHTML(fmtNumber(repo.users)) + ' users</span>' +
          '</div>' +
          refs +
        '</section>';
      }).join("");
    }
    function renderTable(el, columns, rows, emptyText) {
      if (!rows || !rows.length) {
        el.innerHTML = '<div class="empty">' + escapeHTML(emptyText) + '</div>';
        return;
      }
      const head = columns.map(col => '<th>' + escapeHTML(col.label) + '</th>').join("");
      const body = rows.map(row =>
        '<tr>' + columns.map(col => '<td>' + col.render(row) + '</td>').join("") + '</tr>'
      ).join("");
      el.innerHTML = '<table><thead><tr>' + head + '</tr></thead><tbody>' + body + '</tbody></table>';
    }
    async function refresh() {
      const res = await fetch('/api/usage', { cache: 'no-store' });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      document.getElementById('generatedAt').textContent = 'generated at ' + fmtTime(data.generated_at);
      renderCards(document.getElementById('summaryCards'), summarySpec, data);
      renderCards(document.getElementById('requestCards'), requestSpec, data);
      renderBreakdown(document.getElementById('sourceBreakdown'), data.source_breakdown || []);
      renderBreakdown(document.getElementById('modelBreakdown'), data.model_breakdown || []);
      renderRepoBreakdown(document.getElementById('repoBreakdown'), data.repo_breakdown || []);
      renderTable(document.getElementById('recentErrors'), [
        { label: 'Time', render: row => escapeHTML(fmtTime(row.time)) },
        { label: 'Source', render: row => '<span class="pill">' + escapeHTML(row.source) + '</span>' },
        { label: 'Message', render: row => '<span class="error">' + escapeHTML(row.message) + '</span>' }
      ], data.recent_errors || [], 'No recent errors found in the scanned log tail.');
      renderTable(document.getElementById('recentSessions'), [
        { label: 'Last Active', render: row => escapeHTML(fmtTime(row.last_active_at)) },
        { label: 'Source', render: row => '<span class="pill">' + escapeHTML(row.source) + '</span>' },
        { label: 'Conversation', render: row => '<span class="mono">' + escapeHTML(row.conversation_key) + '</span>' },
        { label: 'User', render: row => '<span class="mono">' + escapeHTML(row.user_key || '-') + '</span>' },
        { label: 'Model', render: row => escapeHTML(row.model) },
        { label: 'Turns', render: row => escapeHTML(fmtNumber(row.turn_count)) },
        { label: 'Last Question', render: row => escapeHTML(row.last_question || '-') },
        { label: 'Last Error', render: row => row.last_error ? '<span class="error">' + escapeHTML(row.last_error) + '</span>' : '-' }
      ], data.recent_sessions || [], 'No session records found.');
      renderTable(document.getElementById('recentRequests'), [
        { label: 'Time', render: row => escapeHTML(fmtTime(row.time)) },
        { label: 'Source', render: row => '<span class="pill">' + escapeHTML(row.source) + '</span>' },
        { label: 'Conversation', render: row => '<span class="mono">' + escapeHTML(row.conversation_key || '-') + '</span>' },
        { label: 'Elapsed', render: row => escapeHTML(fmtNumber(row.elapsed_ms)) + ' ms' }
      ], data.recent_requests || [], 'No request completion logs found yet.');
      document.getElementById('refreshState').textContent = 'auto refresh OK';
    }
    async function tick() {
      try {
        await refresh();
      } catch (err) {
        document.getElementById('refreshState').textContent = 'refresh failed: ' + err.message;
      }
    }
    tick();
    setInterval(tick, 5000);
  </script>
</body>
</html>`
