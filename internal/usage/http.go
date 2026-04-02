package usage

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type Server struct {
	collector *Collector
	indexPage *template.Template
	listPage  *template.Template
}

func NewServer(collector *Collector) (*Server, error) {
	indexPage, err := template.New("usage_index").Parse(indexHTML)
	if err != nil {
		return nil, fmt.Errorf("parse dashboard template: %w", err)
	}
	listPage, err := template.New("usage_questions").Parse(questionsHTML)
	if err != nil {
		return nil, fmt.Errorf("parse questions template: %w", err)
	}
	return &Server{
		collector: collector,
		indexPage: indexPage,
		listPage:  listPage,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/questions", s.handleQuestionsPage)
	mux.HandleFunc("/api/usage", s.handleUsage)
	mux.HandleFunc("/api/questions", s.handleQuestions)
	mux.HandleFunc("/api/users", s.handleUsers)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.indexPage.Execute(w, nil)
}

func (s *Server) handleQuestionsPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/questions" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.listPage.Execute(w, nil)
}

func (s *Server) handleUsage(w http.ResponseWriter, _ *http.Request) {
	snapshot, err := s.collector.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, snapshot)
}

func (s *Server) handleQuestions(w http.ResponseWriter, r *http.Request) {
	query := QuestionQuery{
		Page:     parseIntQuery(r.URL.Query().Get("page"), 1),
		PageSize: parseIntQuery(r.URL.Query().Get("page_size"), 50),
		UserKey:  strings.TrimSpace(r.URL.Query().Get("user_key")),
		Source:   strings.TrimSpace(r.URL.Query().Get("source")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Query:    strings.TrimSpace(r.URL.Query().Get("q")),
		From:     parseDateQuery(r.URL.Query().Get("from"), false),
		To:       parseDateQuery(r.URL.Query().Get("to"), true),
	}
	page, err := s.collector.QuestionsPage(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, page)
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	page, err := s.collector.UsersPage(UserQuery{
		Page:     parseIntQuery(r.URL.Query().Get("page"), 1),
		PageSize: parseIntQuery(r.URL.Query().Get("page_size"), 50),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, page)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

const sharedStyle = `
    :root {
      color-scheme: light;
      --bg: #f3efe5;
      --bg-deep: #ebe3d2;
      --panel: rgba(255,255,255,0.82);
      --panel-strong: rgba(255,255,255,0.94);
      --ink: #16212b;
      --muted: #667381;
      --line: rgba(22,33,43,0.09);
      --accent: #b44a18;
      --accent-soft: #ffe4d5;
      --teal: #0c6d73;
      --warn: #9b2c2c;
      --shadow: 0 22px 70px rgba(42, 35, 23, 0.12);
      --radius: 22px;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(224, 140, 76, 0.28), transparent 26%),
        radial-gradient(circle at top right, rgba(14, 109, 115, 0.18), transparent 24%),
        linear-gradient(180deg, #faf7f1 0%, var(--bg) 100%);
      font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif;
    }
    a { color: inherit; text-decoration: none; }
    .shell { max-width: 1440px; margin: 0 auto; padding: 28px 20px 40px; }
    .hero { display: grid; gap: 12px; margin-bottom: 22px; }
    .hero-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; flex-wrap: wrap; }
    .kicker { font-size: 12px; letter-spacing: 0.16em; text-transform: uppercase; color: var(--accent); font-weight: 700; }
    h1 { margin: 0; font-size: clamp(32px, 4vw, 64px); line-height: 0.96; font-weight: 700; max-width: 12ch; }
    .subhead { max-width: 78ch; color: var(--muted); font-size: 16px; line-height: 1.5; margin: 0; }
    .meta { display: flex; gap: 16px; flex-wrap: wrap; color: var(--muted); font-size: 13px; }
    .nav-link {
      display: inline-flex; align-items: center; gap: 8px; border-radius: 999px; padding: 10px 14px;
      background: var(--panel-strong); border: 1px solid var(--line); box-shadow: var(--shadow); font-size: 14px; font-weight: 700;
    }
    .grid { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: 16px; }
    .panel { grid-column: span 12; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); box-shadow: var(--shadow); backdrop-filter: blur(18px); overflow: hidden; }
    .panel-inner { padding: 18px 18px 20px; }
    .panel-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; margin-bottom: 14px; }
    .panel h2 { margin: 0; font-size: 18px; }
    .panel-note { color: var(--muted); font-size: 13px; }
    .card-strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; }
    .stat { background: var(--panel-strong); border: 1px solid var(--line); border-radius: 16px; padding: 14px; }
    .stat-label { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 8px; }
    .stat-value { font-size: 34px; line-height: 1; font-weight: 700; margin-bottom: 8px; }
    .stat-note { color: var(--muted); font-size: 13px; }
    .split-6 { grid-column: span 6; }
    .split-4 { grid-column: span 4; }
    .split-8 { grid-column: span 8; }
    .split-12 { grid-column: span 12; }
    .list { display: grid; gap: 10px; }
    .row { display: grid; grid-template-columns: minmax(0, 1.2fr) 84px; gap: 12px; align-items: center; }
    .bar-wrap { position: relative; height: 40px; border-radius: 12px; border: 1px solid var(--line); background: rgba(255,255,255,0.56); overflow: hidden; }
    .bar { position: absolute; inset: 0 auto 0 0; background: linear-gradient(90deg, #e99054, #bb4d00); opacity: 0.9; }
    .bar-label { position: relative; z-index: 1; display: flex; justify-content: space-between; gap: 12px; align-items: center; height: 100%; padding: 0 14px; font-size: 14px; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th, td { text-align: left; padding: 10px 8px; border-top: 1px solid var(--line); vertical-align: top; }
    th { color: var(--muted); font-weight: 600; font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; border-top: none; padding-top: 0; }
    .pill { display: inline-flex; align-items: center; border-radius: 999px; padding: 4px 9px; font-size: 12px; background: var(--accent-soft); color: var(--accent); font-weight: 700; }
    .pill-teal { background: rgba(12,109,115,0.12); color: var(--teal); }
    .error { color: var(--warn); white-space: pre-wrap; }
    .empty { color: var(--muted); font-style: italic; padding: 8px 0 4px; }
    .filters { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 10px; }
    .field { display: grid; gap: 6px; }
    .field label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
    .field input, .field select {
      width: 100%; padding: 10px 12px; border-radius: 12px; border: 1px solid var(--line);
      background: rgba(255,255,255,0.84); color: var(--ink); font: inherit;
    }
    .actions { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
    .button {
      display: inline-flex; justify-content: center; align-items: center; border-radius: 12px;
      border: 1px solid var(--line); background: var(--panel-strong); padding: 10px 14px; font: inherit; cursor: pointer;
    }
    .button-primary { background: linear-gradient(90deg, #f0a46f, #d86b29); color: #fff; border: none; }
    .pagination { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-top: 14px; flex-wrap: wrap; }
    .sparkline { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 10px; }
    .spark-col { display: grid; gap: 8px; align-items: end; }
    .spark-bar-wrap { height: 120px; display: flex; align-items: end; }
    .spark-bar { width: 100%; border-radius: 12px 12px 4px 4px; background: linear-gradient(180deg, #f0a46f, #c7551c); min-height: 4px; }
    .spark-label { color: var(--muted); font-size: 12px; text-align: center; }
    .spark-value { font-size: 13px; text-align: center; font-weight: 700; }
    @media (max-width: 1100px) { .filters { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (max-width: 980px) {
      .split-6, .split-4, .split-8 { grid-column: span 12; }
      .shell { padding: 18px 14px 28px; }
      .row { grid-template-columns: minmax(0, 1fr) 64px; }
      .filters { grid-template-columns: 1fr; }
    }
`

const sharedScript = `
    function escapeHTML(s) {
      return String(s ?? "").replace(/[&<>"']/g, function(ch) {
        return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[ch];
      });
    }
    function fmtNumber(v) {
      if (typeof v === "number") {
        return Number.isInteger(v) ? String(v) : String(Math.round(v * 10) / 10);
      }
      return v == null || v === "" ? "-" : String(v);
    }
    function fmtTime(value) {
      if (!value) return "-";
      var d = new Date(value);
      return isNaN(d.getTime()) ? "-" : d.toLocaleString();
    }
    function renderUserLabel(row, href) {
      var name = String(row.user_name || '').trim();
      var key = String(row.user_key || '').trim();
      var primary = name || key || '-';
      var secondary = key && name && key !== name ? '<div class="mono" style="margin-top:4px">' + escapeHTML(key) + '</div>' : '';
      var body = '<span>' + escapeHTML(primary) + '</span>' + secondary;
      if (!href) return body;
      return '<a href="' + escapeHTML(href) + '">' + body + '</a>';
    }
    function renderBreakdown(el, items) {
      if (!items || !items.length) {
        el.innerHTML = '<div class="empty">No data yet.</div>';
        return;
      }
      var max = Math.max.apply(null, items.map(function(item) { return item.value; }).concat([1]));
      el.innerHTML = items.map(function(item) {
        var width = Math.max(8, Math.round(item.value / max * 100));
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
      }).join('');
    }
    function renderTable(el, columns, rows, emptyText) {
      if (!rows || !rows.length) {
        el.innerHTML = '<div class="empty">' + escapeHTML(emptyText) + '</div>';
        return;
      }
      var head = columns.map(function(col) { return '<th>' + escapeHTML(col.label) + '</th>'; }).join('');
      var body = rows.map(function(row) {
        return '<tr>' + columns.map(function(col) { return '<td>' + col.render(row) + '</td>'; }).join('') + '</tr>';
      }).join('');
      el.innerHTML = '<table><thead><tr>' + head + '</tr></thead><tbody>' + body + '</tbody></table>';
    }
`

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>askplanner usage</title>
  <style>` + sharedStyle + `</style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <div class="hero-top">
        <div>
          <div class="kicker">Operations view</div>
          <h1>askplanner live usage</h1>
        </div>
        <a class="nav-link" href="/questions">Open question details</a>
      </div>
      <p class="subhead">Snapshot metrics come from the session store and workspace metadata. Cumulative user and question metrics come from the append-only question event store with best-effort session backfill.</p>
      <div class="meta">
        <div id="generatedAt">loading...</div>
        <div id="refreshState">refresh every 5s</div>
      </div>
    </section>
    <section class="grid">
      <div class="panel split-12">
        <div class="panel-inner">
          <div class="panel-head"><h2>Core status</h2><div class="panel-note">Snapshot and cumulative metrics are shown together; interpret them separately.</div></div>
          <div id="summaryCards" class="card-strip"></div>
        </div>
      </div>
      <div class="panel split-6">
        <div class="panel-inner">
          <div class="panel-head"><h2>Request throughput</h2><div class="panel-note">Derived from recent log tail.</div></div>
          <div id="requestCards" class="card-strip"></div>
        </div>
      </div>
      <div class="panel split-6">
        <div class="panel-inner">
          <div class="panel-head"><h2>Questions per day</h2><div class="panel-note">Last 7 days from question events.</div></div>
          <div id="questionsTrend"></div>
        </div>
      </div>
      <div class="panel split-4"><div class="panel-inner"><h2>Session source</h2><div id="sourceBreakdown" class="list"></div></div></div>
      <div class="panel split-4"><div class="panel-inner"><h2>Question status</h2><div id="questionStatusBreakdown" class="list"></div></div></div>
      <div class="panel split-4"><div class="panel-inner"><h2>Model override</h2><div id="modelBreakdown" class="list"></div></div></div>
      <div class="panel split-6"><div class="panel-inner"><h2>Top users</h2><div id="topUsers"></div></div></div>
      <div class="panel split-6"><div class="panel-inner"><h2>Workspace refs</h2><div id="repoBreakdown" class="list"></div></div></div>
      <div class="panel split-12"><div class="panel-inner"><div class="panel-head"><h2>Users overview</h2><div class="panel-note">Shows cumulative and recent per-user question counts.</div></div><div id="usersTable"></div></div></div>
      <div class="panel split-12"><div class="panel-inner"><h2>Recent sessions</h2><div id="recentSessions"></div></div></div>
      <div class="panel split-6"><div class="panel-inner"><h2>Recent requests</h2><div id="recentRequests"></div></div></div>
      <div class="panel split-6"><div class="panel-inner"><h2>Recent errors</h2><div id="recentErrors"></div></div></div>
    </section>
  </main>
  <script>` + sharedScript + `
    var summarySpec = [
      ["Total users", "summary.total_users", "cumulative question-event users"],
      ["Total users by name", "summary.total_users_by_name", "distinct resolved real names"],
      ["Resolved user keys", "summary.resolved_user_keys", "user keys with a resolved real name"],
      ["Name coverage %", "summary.user_name_coverage_pct", "resolved user keys / total user keys"],
      ["Total questions", "summary.total_questions", "cumulative question events"],
      ["Avg questions / user", "summary.avg_questions_per_user", "cumulative average"],
      ["Active users 24h", "summary.active_users_24_hours", "from question events"],
      ["Active users 7d", "summary.active_users_7_days", "from question events"],
      ["Active names 24h", "summary.active_users_24_hours_by_name", "distinct resolved names in 24h"],
      ["Active names 7d", "summary.active_users_7_days_by_name", "distinct resolved names in 7d"],
      ["Total conversations", "summary.total_conversations", "session store snapshot"],
      ["Active sessions 24h", "summary.active_24_hours", "session store snapshot"],
      ["Workspace users", "summary.workspace_users", "workspace metadata count"]
    ];
    var requestSpec = [
      ["Requests 5m", "request_stats.requests_5_min", "recent logs"],
      ["Requests 1h", "request_stats.requests_1_hour", "recent logs"],
      ["Errors 1h", "request_stats.errors_1_hour", "recent logs"],
      ["P50 latency ms", "request_stats.p50_latency_ms", "recent logs"],
      ["P95 latency ms", "request_stats.p95_latency_ms", "recent logs"]
    ];
    function get(obj, path) {
      return path.split('.').reduce(function(acc, key) { return acc == null ? acc : acc[key]; }, obj);
    }
    function renderCards(el, spec, data) {
      el.innerHTML = spec.map(function(item) {
        return '<article class="stat">' +
          '<div class="stat-label">' + escapeHTML(item[0]) + '</div>' +
          '<div class="stat-value">' + escapeHTML(fmtNumber(get(data, item[1]))) + '</div>' +
          '<div class="stat-note">' + escapeHTML(item[2] || '') + '</div>' +
        '</article>';
      }).join('');
    }
    function renderTrend(el, items) {
      if (!items || !items.length) {
        el.innerHTML = '<div class="empty">No trend data yet.</div>';
        return;
      }
      var max = Math.max.apply(null, items.map(function(item) { return item.value; }).concat([1]));
      el.innerHTML = '<div class="sparkline">' + items.map(function(item) {
        var height = Math.max(4, Math.round(item.value / max * 120));
        return '<div class="spark-col">' +
          '<div class="spark-bar-wrap"><div class="spark-bar" style="height:' + height + 'px"></div></div>' +
          '<div class="spark-value">' + escapeHTML(String(item.value)) + '</div>' +
          '<div class="spark-label">' + escapeHTML(item.date.slice(5)) + '</div>' +
        '</div>';
      }).join('') + '</div>';
    }
    function renderRepoBreakdown(el, repos) {
      if (!repos || !repos.length) {
        el.innerHTML = '<div class="empty">No workspace metadata yet.</div>';
        return;
      }
      el.innerHTML = repos.map(function(repo) {
        var refs = repo.refs.length ? repo.refs.map(function(ref) {
          return '<div style="display:flex;justify-content:space-between;gap:12px;padding:6px 0;border-top:1px solid var(--line)">' +
            '<span class="mono">' + escapeHTML(ref.name) + '</span>' +
            '<span>' + escapeHTML(fmtNumber(ref.value)) + '</span>' +
          '</div>';
        }).join('') : '<div class="empty">No refs.</div>';
        return '<section class="stat" style="margin-bottom:12px">' +
          '<div style="display:flex;justify-content:space-between;gap:12px;align-items:center;margin-bottom:10px">' +
            '<strong>' + escapeHTML(repo.name) + '</strong>' +
            '<span class="pill">' + escapeHTML(fmtNumber(repo.users)) + ' users</span>' +
          '</div>' + refs +
        '</section>';
      }).join('');
    }
    function renderTopUsers(el, rows) {
      renderTable(el, [
        { label: 'User', render: function(row) { return renderUserLabel(row, '/questions?user_key=' + encodeURIComponent(row.user_key)); } },
        { label: 'Source', render: function(row) { return '<span class="pill pill-teal">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Questions', render: function(row) { return escapeHTML(fmtNumber(row.question_count)); } },
        { label: '24h', render: function(row) { return escapeHTML(fmtNumber(row.question_count_24h)); } },
        { label: '7d', render: function(row) { return escapeHTML(fmtNumber(row.question_count_7d)); } },
        { label: 'Last Asked', render: function(row) { return escapeHTML(fmtTime(row.last_asked_at)); } },
        { label: 'Recent Question', render: function(row) { return escapeHTML(row.recent_question || '-'); } }
      ], rows, 'No user data yet.');
    }
    async function refresh() {
      var usageRes = await fetch('/api/usage', { cache: 'no-store' });
      if (!usageRes.ok) throw new Error(await usageRes.text());
      var usersRes = await fetch('/api/users?page=1&page_size=20', { cache: 'no-store' });
      if (!usersRes.ok) throw new Error(await usersRes.text());
      var data = await usageRes.json();
      var users = await usersRes.json();
      document.getElementById('generatedAt').textContent = 'generated at ' + fmtTime(data.generated_at);
      document.getElementById('refreshState').textContent = 'auto refresh OK';
      renderCards(document.getElementById('summaryCards'), summarySpec, data);
      renderCards(document.getElementById('requestCards'), requestSpec, data);
      renderBreakdown(document.getElementById('sourceBreakdown'), data.source_breakdown || []);
      renderBreakdown(document.getElementById('questionStatusBreakdown'), data.question_status_breakdown || []);
      renderBreakdown(document.getElementById('modelBreakdown'), data.model_breakdown || []);
      renderRepoBreakdown(document.getElementById('repoBreakdown'), data.repo_breakdown || []);
      renderTrend(document.getElementById('questionsTrend'), data.questions_per_day_7d || []);
      renderTopUsers(document.getElementById('topUsers'), data.top_users || []);
      renderTable(document.getElementById('usersTable'), [
        { label: 'User', render: function(row) { return renderUserLabel(row, '/questions?user_key=' + encodeURIComponent(row.user_key)); } },
        { label: 'Source', render: function(row) { return '<span class="pill pill-teal">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Questions', render: function(row) { return escapeHTML(fmtNumber(row.question_count)); } },
        { label: '24h', render: function(row) { return escapeHTML(fmtNumber(row.question_count_24h)); } },
        { label: '7d', render: function(row) { return escapeHTML(fmtNumber(row.question_count_7d)); } },
        { label: 'Last Asked', render: function(row) { return escapeHTML(fmtTime(row.last_asked_at)); } },
        { label: 'Recent Question', render: function(row) { return escapeHTML(row.recent_question || '-'); } }
      ], users.items || [], 'No user data yet.');
      renderTable(document.getElementById('recentSessions'), [
        { label: 'Last Active', render: function(row) { return escapeHTML(fmtTime(row.last_active_at)); } },
        { label: 'Source', render: function(row) { return '<span class="pill">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Conversation', render: function(row) { return '<span class="mono">' + escapeHTML(row.conversation_key) + '</span>'; } },
        { label: 'User', render: function(row) { return renderUserLabel(row, ''); } },
        { label: 'Model', render: function(row) { return escapeHTML(row.model); } },
        { label: 'Turns', render: function(row) { return escapeHTML(fmtNumber(row.turn_count)); } },
        { label: 'Last Question', render: function(row) { return escapeHTML(row.last_question || '-'); } },
        { label: 'Last Error', render: function(row) { return row.last_error ? '<span class="error">' + escapeHTML(row.last_error) + '</span>' : '-'; } }
      ], data.recent_sessions || [], 'No session records found.');
      renderTable(document.getElementById('recentRequests'), [
        { label: 'Time', render: function(row) { return escapeHTML(fmtTime(row.time)); } },
        { label: 'Source', render: function(row) { return '<span class="pill">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Conversation', render: function(row) { return '<span class="mono">' + escapeHTML(row.conversation_key || '-') + '</span>'; } },
        { label: 'Elapsed', render: function(row) { return escapeHTML(fmtNumber(row.elapsed_ms)) + ' ms'; } }
      ], data.recent_requests || [], 'No request completion logs found yet.');
      renderTable(document.getElementById('recentErrors'), [
        { label: 'Time', render: function(row) { return escapeHTML(fmtTime(row.time)); } },
        { label: 'Source', render: function(row) { return '<span class="pill">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Message', render: function(row) { return '<span class="error">' + escapeHTML(row.message) + '</span>'; } }
      ], data.recent_errors || [], 'No recent errors found in the scanned log tail.');
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

const questionsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>askplanner questions</title>
  <style>` + sharedStyle + `</style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <div class="hero-top">
        <div>
          <div class="kicker">Question details</div>
          <h1>Question detail view</h1>
        </div>
        <a class="nav-link" href="/">Back to dashboard</a>
      </div>
      <p class="subhead">Detailed paginated question view from the append-only question event store. Filters are encoded in the URL so the view is shareable and refresh-safe.</p>
      <div class="meta">
        <div id="pageMeta">loading...</div>
      </div>
    </section>
    <section class="grid">
      <div class="panel split-12">
        <div class="panel-inner">
          <div class="panel-head"><h2>Filters</h2><div class="panel-note">Use filters to narrow by user, source, status, text, or date range.</div></div>
          <form id="filters" class="filters">
            <div class="field"><label>User</label><input name="user_key" placeholder="user key; search also matches resolved names"></div>
            <div class="field"><label>Source</label><select name="source"><option value="">all</option><option value="cli">cli</option><option value="lark">lark</option></select></div>
            <div class="field"><label>Status</label><select name="status"><option value="">all</option><option value="success">success</option><option value="short_circuit">short_circuit</option><option value="error">error</option></select></div>
            <div class="field"><label>Search</label><input name="q" placeholder="question text / conversation key"></div>
            <div class="field"><label>From</label><input type="date" name="from"></div>
            <div class="field"><label>To</label><input type="date" name="to"></div>
          </form>
          <div class="actions" style="margin-top:14px">
            <button id="applyBtn" class="button button-primary" type="button">Apply filters</button>
            <button id="resetBtn" class="button" type="button">Reset</button>
          </div>
        </div>
      </div>
      <div class="panel split-12">
        <div class="panel-inner">
          <div class="panel-head"><h2>Questions</h2><div id="resultsMeta" class="panel-note">loading...</div></div>
          <div id="questionsTable"></div>
          <div class="pagination">
            <div class="panel-note" id="pageInfo"></div>
            <div class="actions">
              <button id="prevBtn" class="button" type="button">Previous</button>
              <button id="nextBtn" class="button" type="button">Next</button>
            </div>
          </div>
        </div>
      </div>
    </section>
  </main>
  <script>` + sharedScript + `
    var currentPage = 1;
    var currentTotalPages = 0;
    function currentQuery() {
      return new URLSearchParams(window.location.search);
    }
    function syncForm() {
      var params = currentQuery();
      var form = document.getElementById('filters');
      ['user_key','source','status','q','from','to'].forEach(function(key) {
        if (form.elements[key]) {
          form.elements[key].value = params.get(key) || '';
        }
      });
    }
    function buildQuery(page) {
      var form = document.getElementById('filters');
      var params = new URLSearchParams();
      ['user_key','source','status','q','from','to'].forEach(function(key) {
        var value = (form.elements[key] && form.elements[key].value || '').trim();
        if (value) params.set(key, value);
      });
      params.set('page', String(page || 1));
      params.set('page_size', '50');
      return params;
    }
    function applyQuery(page) {
      var params = buildQuery(page);
      history.replaceState(null, '', '/questions?' + params.toString());
      load();
    }
    async function load() {
      syncForm();
      var params = currentQuery();
      if (!params.get('page')) {
        params.set('page', '1');
      }
      params.set('page_size', '50');
      var res = await fetch('/api/questions?' + params.toString(), { cache: 'no-store' });
      if (!res.ok) throw new Error(await res.text());
      var data = await res.json();
      currentPage = data.page || 1;
      currentTotalPages = data.total_pages || 0;
      document.getElementById('pageMeta').textContent = 'page ' + currentPage + ' / ' + (currentTotalPages || 1);
      document.getElementById('resultsMeta').textContent = escapeHTML(String(data.total_items || 0)) + ' matching questions';
      document.getElementById('pageInfo').textContent = 'Page ' + currentPage + ' of ' + (currentTotalPages || 1);
      document.getElementById('prevBtn').disabled = currentPage <= 1;
      document.getElementById('nextBtn').disabled = currentTotalPages === 0 || currentPage >= currentTotalPages;
      renderTable(document.getElementById('questionsTable'), [
        { label: 'Asked At', render: function(row) { return escapeHTML(fmtTime(row.asked_at)); } },
        { label: 'User', render: function(row) { return renderUserLabel(row, '/questions?user_key=' + encodeURIComponent(row.user_key)); } },
        { label: 'Source', render: function(row) { return '<span class="pill">' + escapeHTML(row.source) + '</span>'; } },
        { label: 'Status', render: function(row) { return '<span class="pill pill-teal">' + escapeHTML(row.status) + '</span>'; } },
        { label: 'Question', render: function(row) { return escapeHTML(row.question); } },
        { label: 'Conversation', render: function(row) { return '<span class="mono">' + escapeHTML(row.conversation_key) + '</span>'; } },
        { label: 'Latency', render: function(row) { return escapeHTML(fmtNumber(row.duration_ms)) + ' ms'; } },
        { label: 'Model', render: function(row) { return escapeHTML(row.model || '(default)'); } },
        { label: 'Flags', render: function(row) { return row.backfilled ? '<span class="pill">backfilled</span>' : '-'; } },
        { label: 'Error', render: function(row) { return row.error ? '<span class="error">' + escapeHTML(row.error) + '</span>' : '-'; } }
      ], data.items || [], 'No questions match the current filters.');
    }
    document.getElementById('applyBtn').addEventListener('click', function() { applyQuery(1); });
    document.getElementById('resetBtn').addEventListener('click', function() {
      history.replaceState(null, '', '/questions');
      document.getElementById('filters').reset();
      load();
    });
    document.getElementById('prevBtn').addEventListener('click', function() {
      if (currentPage > 1) applyQuery(currentPage - 1);
    });
    document.getElementById('nextBtn').addEventListener('click', function() {
      if (!currentTotalPages || currentPage >= currentTotalPages) return;
      applyQuery(currentPage + 1);
    });
    window.addEventListener('popstate', load);
    load().catch(function(err) {
      document.getElementById('resultsMeta').textContent = 'load failed: ' + err.message;
    });
  </script>
</body>
</html>`
