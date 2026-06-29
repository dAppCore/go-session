// SPDX-License-Identifier: EUPL-1.2

// Service registration for the session package. Exposes the session
// surface (list, fetch, parse, prune, search, analyse, render-html,
// render-mp4) as a Core service with action handlers so consumers
// can wire session operations through the same plumbing as every
// other core service.
//
// Usage example: `c, _ := core.New(core.WithName("session", session.NewService(session.SessionConfig{ProjectsDir: "/Users/snider/.claude/projects"})))`

package session

import (
	"context"
	"time"

	core "dappco.re/go"
)

// SessionConfig is the typed-options struct for the session service.
// ProjectsDir is the base directory containing per-project transcript
// folders (e.g. `~/.claude/projects`).
//
// Usage example: `cfg := session.SessionConfig{ProjectsDir: "/Users/snider/.claude/projects"}`
type SessionConfig struct {
	// ProjectsDir is the root directory holding per-project transcripts.
	// Empty → caller must pass projectsDir on every action invocation.
	ProjectsDir string
}

// Service is the registerable handle for the session package — embeds
// *core.ServiceRuntime[SessionConfig] for typed options access.
//
// Usage example: `svc := core.MustServiceFor[*session.Service](c, "session"); svc.OnStartup(ctx)`
type Service struct {
	*core.ServiceRuntime[SessionConfig]
	registrations core.Once
}

// NewService returns a factory that produces a *Service ready for
// c.Service() registration. Use through core.WithName so the framework
// wires lifecycle (OnStartup registers actions).
//
// Usage example: `c, _ := core.New(core.WithName("session", session.NewService(session.SessionConfig{ProjectsDir: "/Users/snider/.claude/projects"})))`
func NewService(config SessionConfig) func(*core.Core) core.Result {
	return func(c *core.Core) core.Result {
		return core.Ok(&Service{
			ServiceRuntime: core.NewServiceRuntime(c, config),
		})
	}
}

// OnStartup registers the session action handlers on the attached Core.
// Implements core.Startable. Idempotent via core.Once.
//
// Usage example: `r := svc.OnStartup(ctx)`
func (s *Service) OnStartup(context.Context) core.Result {
	if s == nil {
		return core.Ok(nil)
	}
	s.registrations.Do(func() {
		c := s.Core()
		if c == nil {
			return
		}
		c.Action("session.list", s.handleList)
		c.Action("session.fetch", s.handleFetch)
		c.Action("session.parse", s.handleParse)
		c.Action("session.prune", s.handlePrune)
		c.Action("session.search", s.handleSearch)
		c.Action("session.analyse", s.handleAnalyse)
		c.Action("session.render_html", s.handleRenderHTML)
		c.Action("session.render_mp4", s.handleRenderMP4)
	})
	return core.Ok(nil)
}

// OnShutdown is a no-op for the session service — package-level
// functions hold no live handles. Implements core.Stoppable for shape
// parity with other services.
//
// Usage example: `r := svc.OnShutdown(ctx)`
func (s *Service) OnShutdown(context.Context) core.Result {
	return core.Ok(nil)
}

// projectsDir returns opts.projects_dir if set, else falls back to the
// service-level default from SessionConfig.
func (s *Service) projectsDir(opts core.Options) string {
	if dir := opts.String("projects_dir"); dir != "" {
		return dir
	}
	if s != nil && s.ServiceRuntime != nil {
		return s.Options().ProjectsDir
	}
	return ""
}

// handleList — `session.list` action handler. Uses opts.projects_dir
// (or service default) and returns []Session in r.Value.
//
//	r := c.Action("session.list").Run(ctx, core.NewOptions(
//	    core.Option{Key: "projects_dir", Value: "/Users/snider/.claude/projects"},
//	))
//	sessions, _ := r.Value.([]Session)
func (s *Service) handleList(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.list", "projects_dir is required", nil))
	}
	return ListSessions(dir)
}

// handleFetch — `session.fetch` action handler. Reads opts.id (and
// optionally opts.projects_dir) and returns *Session in r.Value.
//
//	r := c.Action("session.fetch").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "abc-123"},
//	))
func (s *Service) handleFetch(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.fetch", "projects_dir is required", nil))
	}
	id := opts.String("id")
	if id == "" {
		return core.Fail(core.E("session.fetch", "id is required", nil))
	}
	return FetchSession(dir, id)
}

// handleParse — `session.parse` action handler. Reads opts.path and
// returns the parsed *ParsedSession in r.Value.
//
//	r := c.Action("session.parse").Run(ctx, core.NewOptions(
//	    core.Option{Key: "path", Value: "/path/to/session.jsonl"},
//	))
func (s *Service) handleParse(_ core.Context, opts core.Options) core.Result {
	path := opts.String("path")
	if path == "" {
		return core.Fail(core.E("session.parse", "path is required", nil))
	}
	return ParseTranscript(path)
}

// handlePrune — `session.prune` action handler. Reads
// opts.max_age_seconds (or opts.projects_dir) and removes every
// transcript older than the threshold. Returns deleted count in r.Value.
//
//	r := c.Action("session.prune").Run(ctx, core.NewOptions(
//	    core.Option{Key: "max_age_seconds", Value: int64(86400 * 30)},
//	))
func (s *Service) handlePrune(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.prune", "projects_dir is required", nil))
	}
	secs := opts.Int("max_age_seconds")
	if secs <= 0 {
		return core.Fail(core.E("session.prune", "max_age_seconds must be > 0", nil))
	}
	return PruneSessions(dir, time.Duration(secs)*time.Second)
}

// handleSearch — `session.search` action handler. Reads opts.query (and
// optionally opts.projects_dir) and returns []SearchResult in r.Value.
//
//	r := c.Action("session.search").Run(ctx, core.NewOptions(
//	    core.Option{Key: "query", Value: "core.E"},
//	))
func (s *Service) handleSearch(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.search", "projects_dir is required", nil))
	}
	query := opts.String("query")
	if query == "" {
		return core.Fail(core.E("session.search", "query is required", nil))
	}
	return Search(dir, query)
}

// handleAnalyse — `session.analyse` action handler. Reads opts.id and
// optionally opts.projects_dir, then returns the *SessionAnalytics
// in r.Value.
//
//	r := c.Action("session.analyse").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "abc-123"},
//	))
func (s *Service) handleAnalyse(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.analyse", "projects_dir is required", nil))
	}
	id := opts.String("id")
	if id == "" {
		return core.Fail(core.E("session.analyse", "id is required", nil))
	}
	fetchR := FetchSession(dir, id)
	if !fetchR.OK {
		return fetchR
	}
	parsed, ok := fetchR.Value.(ParsedSession)
	if !ok || parsed.Session == nil {
		return core.Fail(core.E("session.analyse", "fetched session is invalid", nil))
	}
	return core.Ok(Analyse(parsed.Session))
}

// handleRenderHTML — `session.render_html` action handler. Reads
// opts.id + opts.output_path (and optionally opts.projects_dir) and
// renders the session to HTML at output_path.
//
//	r := c.Action("session.render_html").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "abc-123"},
//	    core.Option{Key: "output_path", Value: "/tmp/session.html"},
//	))
func (s *Service) handleRenderHTML(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.render_html", "projects_dir is required", nil))
	}
	id := opts.String("id")
	output := opts.String("output_path")
	if id == "" || output == "" {
		return core.Fail(core.E("session.render_html", "id and output_path are required", nil))
	}
	fetchR := FetchSession(dir, id)
	if !fetchR.OK {
		return fetchR
	}
	parsed, ok := fetchR.Value.(ParsedSession)
	if !ok || parsed.Session == nil {
		return core.Fail(core.E("session.render_html", "fetched session is invalid", nil))
	}
	return RenderHTML(parsed.Session, output)
}

// handleRenderMP4 — `session.render_mp4` action handler. Reads opts.id
// + opts.output_path (and optionally opts.projects_dir) and renders the
// session to an MP4 via vhs at output_path.
//
//	r := c.Action("session.render_mp4").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "abc-123"},
//	    core.Option{Key: "output_path", Value: "/tmp/session.mp4"},
//	))
func (s *Service) handleRenderMP4(_ core.Context, opts core.Options) core.Result {
	dir := s.projectsDir(opts)
	if dir == "" {
		return core.Fail(core.E("session.render_mp4", "projects_dir is required", nil))
	}
	id := opts.String("id")
	output := opts.String("output_path")
	if id == "" || output == "" {
		return core.Fail(core.E("session.render_mp4", "id and output_path are required", nil))
	}
	fetchR := FetchSession(dir, id)
	if !fetchR.OK {
		return fetchR
	}
	parsed, ok := fetchR.Value.(ParsedSession)
	if !ok || parsed.Session == nil {
		return core.Fail(core.E("session.render_mp4", "fetched session is invalid", nil))
	}
	return RenderMP4(parsed.Session, output)
}
