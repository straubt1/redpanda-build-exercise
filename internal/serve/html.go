package serve

const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>PR triage</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
</head>
<body>
<main class="container">
  <h1>PR triage</h1>
  <p><a href="/api/triages?sort={{.Sort}}&amp;dir={{.Dir}}">JSON</a></p>
  {{if not .Rows}}
  <p>No triages yet.</p>
  {{else}}
  <div class="overflow-auto">
  <table class="striped">
    <thead>
      <tr>
        <th scope="col"><a href="/?sort=repo&amp;dir={{nextDir .Sort .Dir "repo"}}">Repo</a></th>
        <th scope="col"><a href="/?sort=pr_number&amp;dir={{nextDir .Sort .Dir "pr_number"}}">PR</a></th>
        <th scope="col"><a href="/?sort=title&amp;dir={{nextDir .Sort .Dir "title"}}">Title</a></th>
        <th scope="col"><a href="/?sort=category&amp;dir={{nextDir .Sort .Dir "category"}}">Category</a></th>
        <th scope="col"><a href="/?sort=confidence&amp;dir={{nextDir .Sort .Dir "confidence"}}">Conf.</a></th>
        <th scope="col">Area</th>
        <th scope="col">Rationale</th>
        <th scope="col"><a href="/?sort=source&amp;dir={{nextDir .Sort .Dir "source"}}">Source</a></th>
        <th scope="col"><a href="/?sort=classified_at&amp;dir={{nextDir .Sort .Dir "classified_at"}}">Time</a></th>
      </tr>
    </thead>
    <tbody>
      {{range .Rows}}
      <tr>
        <td>{{.Repo}}</td>
        <td>{{if .PRURL}}<a href="{{.PRURL}}">#{{.PRNumber}}</a>{{else}}#{{.PRNumber}}{{end}}</td>
        <td>{{.Title}}</td>
        <td>{{.Category}}</td>
        <td>{{fmtConf .Confidence}}</td>
        <td>{{.AffectedArea}}</td>
        <td>{{.Rationale}}</td>
        <td>{{.Source}}</td>
        <td>{{fmtTime .ClassifiedAt}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  </div>
  {{end}}
</main>
</body>
</html>
`
