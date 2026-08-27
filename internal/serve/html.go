package serve

const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pull Request Triage</title>
  <style>
    :root { color-scheme: light; font-family: ui-sans-serif, system-ui, sans-serif; }
    body { margin: 2rem; color: #122; background: #f7f8f5; }
    h1 { font-size: 1.4rem; margin: 0 0 0.25rem; }
    p.sub { color: #456; margin: 0 0 1.25rem; }
    .stats { display: flex; gap: 0.75rem; margin: 0 0 1.25rem; flex-wrap: wrap; }
    .stat { background: #fff; padding: 0.55rem 0.85rem; border-bottom: 1px solid #e4e6df; min-width: 7.5rem; }
    .stat span { display: block; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.04em; color: #567; }
    .stat b { font-size: 1.15rem; font-weight: 600; }
    table { width: 100%; border-collapse: collapse; background: #fff; }
    th, td { text-align: left; padding: 0.55rem 0.7rem; border-bottom: 1px solid #e4e6df; vertical-align: top; font-size: 0.92rem; }
    th { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.04em; color: #567; }
    th a { color: inherit; text-decoration: none; }
    th a:hover { text-decoration: underline; }
    .cat { font-weight: 600; white-space: nowrap; }
    .muted { color: #678; }
    time.when { white-space: nowrap; }
    a { color: #0b5; }
  </style>
</head>
<body>
  <h1>GitHub Pull Request Triage</h1>
  <p class="sub">API: <a href="/api/triages?sort={{.Sort}}&amp;dir={{.Dir}}">/api/triages</a> · Showing {{.Cap}} records</p>
  <div class="stats">
    <div class="stat"><span>Total</span><b>{{.Stats.Total}}</b></div>
    <div class="stat"><span>Not reasoned</span><b>{{.Stats.Pending}}</b></div>
    <div class="stat"><span>Model</span><b>{{.Stats.Model}}</b></div>
    <div class="stat"><span>Rule</span><b>{{.Stats.Rule}}</b></div>
  </div>
  <table>
    <thead>
      <tr>
        <th><a href="/?sort=classified_at&amp;dir={{nextDir .Sort .Dir "classified_at"}}">When</a></th>
        <th><a href="/?sort=repo&amp;dir={{nextDir .Sort .Dir "repo"}}">Repo</a></th>
        <th><a href="/?sort=pr_number&amp;dir={{nextDir .Sort .Dir "pr_number"}}">PR</a></th>
        <th><a href="/?sort=category&amp;dir={{nextDir .Sort .Dir "category"}}">Category</a></th>
        <th><a href="/?sort=confidence&amp;dir={{nextDir .Sort .Dir "confidence"}}">Conf</a></th>
        <th><a href="/?sort=source&amp;dir={{nextDir .Sort .Dir "source"}}">Source</a></th>
        <th>Area</th>
        <th>Summary</th>
        <th>Rationale</th>
      </tr>
    </thead>
    <tbody>
      {{if not .Rows}}
      <tr><td colspan="9" class="muted">No triages yet — wait for Connect to poll GitHub and the worker to classify.</td></tr>
      {{else}}
      {{range .Rows}}
      <tr>
        <td class="muted" title="{{fmtRFC3339 .ClassifiedAt}}">{{if .ClassifiedAt.IsZero}}{{else}}<time class="when" datetime="{{fmtRFC3339 .ClassifiedAt}}">{{fmtRFC3339 .ClassifiedAt}}</time>{{end}}</td>
        <td>{{.Repo}}</td>
        <td>{{if .PRURL}}<a href="{{.PRURL}}" target="_blank" rel="noopener">#{{.PRNumber}}</a>{{else}}#{{.PRNumber}}{{end}}{{if .Title}}<br/><span class="muted">{{.Title}}</span>{{end}}</td>
        <td class="cat">{{.Category}}</td>
        <td>{{fmtConf .Confidence}}</td>
        <td>{{.Source}}</td>
        <td>{{.AffectedArea}}</td>
        <td>{{.Summary}}</td>
        <td>{{.Rationale}}</td>
      </tr>
      {{end}}
      {{end}}
    </tbody>
  </table>
  <script>
    document.querySelectorAll("time.when").forEach(function (el) {
      var iso = el.getAttribute("datetime");
      var d = new Date(iso);
      if (isNaN(d.getTime())) return;
      el.textContent = d.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" })
        .replace(" AM", " am").replace(" PM", " pm");
      var full;
      try {
        full = d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "medium" });
      } catch (e) {
        full = d.toString();
      }
      el.removeAttribute("title");
      var cell = el.parentElement;
      if (cell) cell.setAttribute("title", full);
    });
  </script>
</body>
</html>
`
