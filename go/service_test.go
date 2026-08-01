// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"context"

	core "dappco.re/go"
)

// --- AX-7 compliance triplets ---

func TestService_NewService_Good(t *core.T) {
	cfg := SessionConfig{ProjectsDir: t.TempDir()}
	factory := NewService(cfg)
	core.AssertNotNil(t, factory)
}

func TestService_NewService_Bad(t *core.T) {
	// NewService alone is a factory; resolution happens at c.Service().
	cfg := SessionConfig{}
	factory := NewService(cfg)
	core.AssertNotNil(t, factory)
}

func TestService_NewService_Ugly(t *core.T) {
	a := NewService(SessionConfig{ProjectsDir: t.TempDir()})
	b := NewService(SessionConfig{ProjectsDir: t.TempDir()})
	core.AssertNotNil(t, a)
	core.AssertNotNil(t, b)
}

func serviceForTest(t *core.T) *Service {
	t.Helper()
	c := core.New()
	r := NewService(SessionConfig{ProjectsDir: t.TempDir()})(c)
	core.RequireTrue(t, r.OK)
	return r.Value.(*Service)
}

func TestService_Service_OnStartup_Good(t *core.T) {
	svc := serviceForTest(t)
	startup := svc.OnStartup(t.Context())
	core.AssertTrue(t, startup.OK)
}

func TestService_Service_OnStartup_Bad(t *core.T) {
	var s *Service
	r := s.OnStartup(t.Context())
	core.AssertTrue(t, r.OK)
}

func TestService_Service_OnStartup_Ugly(t *core.T) {
	svc := serviceForTest(t)
	svc.OnStartup(t.Context())
	again := svc.OnStartup(t.Context())
	core.AssertTrue(t, again.OK)
}

func TestService_Service_OnShutdown_Good(t *core.T) {
	svc := serviceForTest(t)
	shutdown := svc.OnShutdown(t.Context())
	core.AssertTrue(t, shutdown.OK)
}

func TestService_Service_OnShutdown_Bad(t *core.T) {
	var s *Service
	r := s.OnShutdown(t.Context())
	core.AssertTrue(t, r.OK)
}

func TestService_Service_OnShutdown_Ugly(t *core.T) {
	svc := serviceForTest(t)
	svc.OnShutdown(t.Context())
	again := svc.OnShutdown(t.Context())
	core.AssertTrue(t, again.OK)
}

// --- end AX-7 compliance triplets ---

// serviceWithSession returns a service bound to a temp projects dir that
// already contains one transcript with the given id. The returned dir is
// the configured ProjectsDir so opts may omit projects_dir.
func serviceWithSession(t *core.T, id, text string) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	writeJSONL(t, dir, id+".jsonl",
		userTextEntry(text),
		toolUseEntry("Bash", "tu-1", map[string]any{"command": "echo hi"}),
		toolResultEntry("tu-1", "hi", false),
	)
	c := core.New()
	r := NewService(SessionConfig{ProjectsDir: dir})(c)
	core.RequireTrue(t, r.OK)
	return r.Value.(*Service), dir
}

func ctx() core.Context { return context.Background() }

// --- handleList ---

func TestService_Service_handleList_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleList(ctx(), core.NewOptions())

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertLen(t, r.Value.([]Session), 1)
}

func TestService_Service_handleList_Bad(t *core.T) {
	c := core.New()
	svc := NewService(SessionConfig{})(c).Value.(*Service)

	r := svc.handleList(ctx(), core.NewOptions())

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "projects_dir is required")
}

func TestService_Service_handleList_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")
	override := t.TempDir()

	r := svc.handleList(ctx(), core.NewOptions(core.Option{Key: "projects_dir", Value: override}))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertEmpty(t, r.Value.([]Session))
}

// --- handleFetch ---

func TestService_Service_handleFetch_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleFetch(ctx(), core.NewOptions(core.Option{Key: "id", Value: "abc"}))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertEqual(t, "abc", r.Value.(ParsedSession).Session.ID)
}

func TestService_Service_handleFetch_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleFetch(ctx(), core.NewOptions())

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "id is required")
}

func TestService_Service_handleFetch_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleFetch(ctx(), core.NewOptions(core.Option{Key: "id", Value: "../escape"}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "invalid session id")
}

// --- handleParse ---

func TestService_Service_handleParse_Good(t *core.T) {
	svc, dir := serviceWithSession(t, "abc", "hello")

	r := svc.handleParse(ctx(), core.NewOptions(core.Option{Key: "path", Value: core.PathJoin(dir, "abc.jsonl")}))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertEqual(t, "abc", r.Value.(ParsedSession).Session.ID)
}

func TestService_Service_handleParse_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleParse(ctx(), core.NewOptions())

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "path is required")
}

func TestService_Service_handleParse_Ugly(t *core.T) {
	svc, dir := serviceWithSession(t, "abc", "hello")

	r := svc.handleParse(ctx(), core.NewOptions(core.Option{Key: "path", Value: core.PathJoin(dir, "missing.jsonl")}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "open transcript")
}

// --- handlePrune ---

func TestService_Service_handlePrune_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handlePrune(ctx(), core.NewOptions(core.Option{Key: "max_age_seconds", Value: 48 * 3600}))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertEqual(t, 0, r.Value.(int))
}

func TestService_Service_handlePrune_Bad(t *core.T) {
	c := core.New()
	svc := NewService(SessionConfig{})(c).Value.(*Service)

	r := svc.handlePrune(ctx(), core.NewOptions(core.Option{Key: "max_age_seconds", Value: 3600}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "projects_dir is required")
}

func TestService_Service_handlePrune_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handlePrune(ctx(), core.NewOptions(core.Option{Key: "max_age_seconds", Value: 0}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "max_age_seconds must be > 0")
}

// --- handleSearch ---

func TestService_Service_handleSearch_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleSearch(ctx(), core.NewOptions(core.Option{Key: "query", Value: "echo hi"}))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertLen(t, r.Value.([]SearchResult), 1)
}

func TestService_Service_handleSearch_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleSearch(ctx(), core.NewOptions())

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "query is required")
}

func TestService_Service_handleSearch_Ugly(t *core.T) {
	c := core.New()
	svc := NewService(SessionConfig{})(c).Value.(*Service)

	r := svc.handleSearch(ctx(), core.NewOptions(core.Option{Key: "query", Value: "x"}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "projects_dir is required")
}

// --- handleAnalyse ---

func TestService_Service_handleAnalyse_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleAnalyse(ctx(), core.NewOptions(core.Option{Key: "id", Value: "abc"}))

	core.RequireTrue(t, r.OK, r.Error())
	got := r.Value.(*SessionAnalytics)
	core.AssertEqual(t, 2, got.EventCount)
	core.AssertEqual(t, 1, got.ToolCounts["Bash"])
}

func TestService_Service_handleAnalyse_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleAnalyse(ctx(), core.NewOptions())

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "id is required")
}

func TestService_Service_handleAnalyse_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleAnalyse(ctx(), core.NewOptions(core.Option{Key: "id", Value: "missing"}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "open transcript")
}

// --- handleRenderHTML ---

func TestService_Service_handleRenderHTML_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")
	out := core.PathJoin(t.TempDir(), "out.html")

	r := svc.handleRenderHTML(ctx(), core.NewOptions(
		core.Option{Key: "id", Value: "abc"},
		core.Option{Key: "output_path", Value: out},
	))

	core.RequireTrue(t, r.OK, r.Error())
	core.AssertTrue(t, hostFS.Exists(out).OK)
}

func TestService_Service_handleRenderHTML_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleRenderHTML(ctx(), core.NewOptions(core.Option{Key: "id", Value: "abc"}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "id and output_path are required")
}

func TestService_Service_handleRenderHTML_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleRenderHTML(ctx(), core.NewOptions(
		core.Option{Key: "id", Value: "missing"},
		core.Option{Key: "output_path", Value: core.PathJoin(t.TempDir(), "out.html")},
	))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "open transcript")
}

// --- handleRenderMP4 ---

func TestService_Service_handleRenderMP4_Good(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")
	out := core.PathJoin(t.TempDir(), "out.mp4")

	r := svc.handleRenderMP4(ctx(), core.NewOptions(
		core.Option{Key: "id", Value: "abc"},
		core.Option{Key: "output_path", Value: out},
	))

	// vhs is not installed in CI: the fetch+cast path must succeed and the
	// failure must come from the renderer, never from "fetched session is
	// invalid". When vhs is present the render succeeds outright.
	if lookupExecutable("vhs") == "" {
		core.AssertFalse(t, r.OK)
		core.AssertContains(t, r.Error(), "vhs not installed")
		core.AssertNotContains(t, r.Error(), "fetched session is invalid")
		return
	}
	core.AssertTrue(t, r.OK, r.Error())
}

func TestService_Service_handleRenderMP4_Bad(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleRenderMP4(ctx(), core.NewOptions(core.Option{Key: "id", Value: "abc"}))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "id and output_path are required")
}

func TestService_Service_handleRenderMP4_Ugly(t *core.T) {
	svc, _ := serviceWithSession(t, "abc", "hello")

	r := svc.handleRenderMP4(ctx(), core.NewOptions(
		core.Option{Key: "id", Value: "missing"},
		core.Option{Key: "output_path", Value: core.PathJoin(t.TempDir(), "out.mp4")},
	))

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "open transcript")
}

// --- projectsDir fallback ---

func TestService_Service_projectsDir_Good(t *core.T) {
	dir := t.TempDir()
	svc := NewService(SessionConfig{ProjectsDir: dir})(core.New()).Value.(*Service)

	core.AssertEqual(t, dir, svc.projectsDir(core.NewOptions()))
}

func TestService_Service_projectsDir_Bad(t *core.T) {
	var s *Service

	core.AssertEqual(t, "", s.projectsDir(core.NewOptions()))
}

func TestService_Service_projectsDir_Ugly(t *core.T) {
	svc := NewService(SessionConfig{ProjectsDir: "/default"})(core.New()).Value.(*Service)

	got := svc.projectsDir(core.NewOptions(core.Option{Key: "projects_dir", Value: "/override"}))

	core.AssertEqual(t, "/override", got)
}
