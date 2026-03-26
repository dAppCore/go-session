// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"html"
	"path"
	"time"

	core "dappco.re/go/core"
)

// RenderHTML generates a self-contained HTML timeline from a session.
//
// Example:
// err := session.RenderHTML(sess, "/tmp/session.html")
func RenderHTML(sess *Session, outputPath string) error {
	if !hostFS.IsDir(path.Dir(outputPath)) {
		return core.E("RenderHTML", "parent directory does not exist", nil)
	}

	duration := sess.EndTime.Sub(sess.StartTime)
	toolCount := 0
	errorCount := 0
	for e := range sess.EventsSeq() {
		if e.Type == "tool_use" {
			toolCount++
			if !e.Success {
				errorCount++
			}
		}
	}

	b := core.NewBuilder()
	b.WriteString(core.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Session %s</title>
<style>
:root {
  --bg: #0d1117; --bg2: #161b22; --bg3: #21262d;
  --fg: #c9d1d9; --dim: #8b949e; --accent: #58a6ff;
  --green: #3fb950; --red: #f85149; --yellow: #d29922;
  --border: #30363d; --font: 'SF Mono', 'Cascadia Code', 'JetBrains Mono', monospace;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: var(--bg); color: var(--fg); font-family: var(--font); font-size: 13px; line-height: 1.5; }
.header { background: var(--bg2); border-bottom: 1px solid var(--border); padding: 16px 24px; position: sticky; top: 0; z-index: 10; }
.header h1 { font-size: 16px; font-weight: 600; color: var(--accent); }
.header .meta { color: var(--dim); font-size: 12px; margin-top: 4px; }
.header .stats span { display: inline-block; margin-right: 16px; }
.header .stats .err { color: var(--red); }
.search { margin-top: 8px; display: flex; gap: 8px; }
.search input { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; color: var(--fg); font-family: var(--font); font-size: 12px; padding: 6px 12px; width: 300px; outline: none; }
.search input:focus { border-color: var(--accent); }
.search select { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; color: var(--fg); font-family: var(--font); font-size: 12px; padding: 6px 8px; outline: none; }
.timeline { padding: 16px 24px; }
.event { border: 1px solid var(--border); border-radius: 8px; margin-bottom: 8px; overflow: hidden; transition: border-color 0.15s; }
.event:hover { border-color: var(--accent); }
.event.error { border-color: var(--red); }
.event.hidden { display: none; }
.event-header { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; user-select: none; background: var(--bg2); }
.event-header:hover { background: var(--bg3); }
.event-header .time { color: var(--dim); font-size: 11px; min-width: 70px; }
.event-header .tool { font-weight: 600; color: var(--accent); min-width: 60px; }
.event-header .tool.bash { color: var(--green); }
.event-header .tool.error { color: var(--red); }
.event-header .tool.user { color: var(--yellow); }
.event-header .tool.assistant { color: var(--dim); }
.event-header .input { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.event-header .dur { color: var(--dim); font-size: 11px; min-width: 50px; text-align: right; }
.event-header .status { font-size: 14px; min-width: 20px; text-align: center; }
.event-header .arrow { color: var(--dim); font-size: 10px; transition: transform 0.15s; min-width: 16px; }
.event.open .arrow { transform: rotate(90deg); }
.event-body { display: none; padding: 12px; background: var(--bg); border-top: 1px solid var(--border); }
.event.open .event-body { display: block; }
.event-body pre { white-space: pre-wrap; word-break: break-all; font-size: 12px; max-height: 400px; overflow-y: auto; }
.event-body .label { color: var(--dim); font-size: 11px; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px; }
.event-body .section { margin-bottom: 12px; }
.event-body .output { color: var(--fg); }
.event-body .output.err { color: var(--red); }
</style>
</head>
<body>
<div class="header">
  <h1>Session %s</h1>
  <div class="meta">
    <div class="stats">
      <span>%s</span>
      <span>Duration: %s</span>
      <span>%d tool calls</span>`,
		shortID(sess.ID), shortID(sess.ID),
		sess.StartTime.Format("2006-01-02 15:04:05"),
		formatDuration(duration),
		toolCount))

	if errorCount > 0 {
		b.WriteString(core.Sprintf(`
      <span class="err">%d errors</span>`, errorCount))
	}

	b.WriteString(`
    </div>
  </div>
  <div class="search">
    <input type="text" id="search" placeholder="Search commands, outputs..." oninput="filterEvents()">
    <select id="filter" onchange="filterEvents()">
      <option value="all">All events</option>
      <option value="tool_use">Tool calls only</option>
      <option value='errors'>Errors only</option>
      <option value="Bash">Bash only</option>
      <option value="user">User messages</option>
    </select>
  </div>
</div>
<div class="timeline" id="timeline">
`)

	var i int
	for evt := range sess.EventsSeq() {
		toolClass := core.Lower(evt.Tool)
		if evt.Type == "user" {
			toolClass = "user"
		} else if evt.Type == "assistant" {
			toolClass = "assistant"
		}

		errorClass := ""
		if !evt.Success && evt.Type == "tool_use" {
			errorClass = " error"
		}

		statusIcon := ""
		if evt.Type == "tool_use" {
			if evt.Success {
				statusIcon = `<span style="color:var(--green)">&#10003;</span>`
			} else {
				statusIcon = `<span style="color:var(--red)">&#10007;</span>`
			}
		}

		toolLabel := evt.Tool
		if evt.Type == "user" {
			toolLabel = "User"
		} else if evt.Type == "assistant" {
			toolLabel = "Claude"
		}

		durStr := ""
		if evt.Duration > 0 {
			durStr = formatDuration(evt.Duration)
		}

		b.WriteString(core.Sprintf(`<div class="event%s" data-type="%s" data-tool="%s" data-text="%s" id="evt-%d">
  <div class="event-header" onclick="toggle(%d)">
    <span class="arrow">&#9654;</span>
    <span class="time">%s</span>
    <span class="tool %s">%s</span>
    <span class="input">%s</span>
    <span class="dur">%s</span>
    <span class="status">%s</span>
  </div>
  <div class="event-body">
`,
			errorClass,
			evt.Type,
			evt.Tool,
			html.EscapeString(core.Lower(core.Concat(evt.Input, " ", evt.Output))),
			i,
			i,
			evt.Timestamp.Format("15:04:05"),
			toolClass,
			html.EscapeString(toolLabel),
			html.EscapeString(truncate(evt.Input, 120)),
			durStr,
			statusIcon))

		if evt.Input != "" {
			label := "Command"
			if evt.Type == "user" {
				label = "Message"
			} else if evt.Type == "assistant" {
				label = "Response"
			} else if evt.Tool == "Read" || evt.Tool == "Glob" || evt.Tool == "Grep" {
				label = "Target"
			} else if evt.Tool == "Edit" || evt.Tool == "Write" {
				label = "File"
			}
			b.WriteString(core.Sprintf(`    <div class="section"><div class="label">%s</div><pre>%s</pre></div>
`, label, html.EscapeString(evt.Input)))
		}

		if evt.Output != "" {
			outClass := "output"
			if !evt.Success {
				outClass = "output err"
			}
			b.WriteString(core.Sprintf(`    <div class="section"><div class="label">Output</div><pre class="%s">%s</pre></div>
`, outClass, html.EscapeString(evt.Output)))
		}

		b.WriteString(`  </div>
</div>
`)
		i++
	}

	b.WriteString(`</div>
<script>
function toggle(i) {
  document.getElementById('evt-'+i).classList.toggle('open');
}
function filterEvents() {
  const q = document.getElementById('search').value.toLowerCase();
  const f = document.getElementById('filter').value;
  document.querySelectorAll('.event').forEach(el => {
    const type = el.dataset.type;
    const tool = el.dataset.tool;
    const text = el.dataset.text;
    let show = true;
    if (f === 'tool_use' && type !== 'tool_use') show = false;
    if (f === 'errors' && !el.classList.contains('error')) show = false;
    if (f === 'Bash' && tool !== 'Bash') show = false;
    if (f === 'user' && type !== 'user') show = false;
    if (q && !text.includes(q)) show = false;
    el.classList.toggle('hidden', !show);
  });
}
document.addEventListener('keydown', e => {
  if (e.key === '/' && document.activeElement.tagName !== 'INPUT') {
    e.preventDefault();
    document.getElementById('search').focus();
  }
});
</script>
</body>
</html>
`)

	writeResult := hostFS.Write(outputPath, b.String())
	if !writeResult.OK {
		return core.E("RenderHTML", "write html", resultError(writeResult))
	}

	return nil
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return core.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return core.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return core.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return core.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
