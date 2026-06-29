// SPDX-License-Identifier: EUPL-1.2

package session_test

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/session"
)

// ExampleNewService constructs the session service factory through
// `NewService` for go-session Core service registration. The factory
// produces a *session.Service ready for c.Service() — OnStartup wires
// the session.* action handlers, OnShutdown is a no-op.
//
// Usage example: `c.Service("session", session.NewService(session.SessionConfig{ProjectsDir: "/Users/snider/.claude/projects"}))`
func ExampleNewService() {
	factory := session.NewService(session.SessionConfig{})
	core.Println(factory != nil)
	// Output: true
}

// ExampleService_OnStartup registers the session.* action handlers on
// the attached Core through `Service.OnStartup` for go-session Core
// service registration. Idempotent — multiple startups won't
// double-register.
//
// Usage example: `r := svc.OnStartup(ctx)`
func ExampleService_OnStartup() {
	c := core.New()
	r := session.NewService(session.SessionConfig{})(c)
	if !r.OK {
		core.Println("startup-init-failed")
		return
	}
	svc := r.Value.(*session.Service)
	startup := svc.OnStartup(context.Background())
	core.Println(startup.OK)
	// Output: true
}

// ExampleService_OnShutdown drains the service through
// `Service.OnShutdown` for go-session Core service registration. The
// session service holds no live handles requiring teardown — Shutdown is
// a no-op returning Ok for shape parity.
//
// Usage example: `r := svc.OnShutdown(ctx)`
func ExampleService_OnShutdown() {
	c := core.New()
	r := session.NewService(session.SessionConfig{})(c)
	if !r.OK {
		core.Println("startup-init-failed")
		return
	}
	svc := r.Value.(*session.Service)
	shutdown := svc.OnShutdown(context.Background())
	core.Println(shutdown.OK)
	// Output: true
}
