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
	startup := svc.OnStartup(context.Background())
	core.AssertTrue(t, startup.OK)
}

func TestService_Service_OnStartup_Bad(t *core.T) {
	var s *Service
	r := s.OnStartup(context.Background())
	core.AssertTrue(t, r.OK)
}

func TestService_Service_OnStartup_Ugly(t *core.T) {
	svc := serviceForTest(t)
	svc.OnStartup(context.Background())
	again := svc.OnStartup(context.Background())
	core.AssertTrue(t, again.OK)
}

func TestService_Service_OnShutdown_Good(t *core.T) {
	svc := serviceForTest(t)
	shutdown := svc.OnShutdown(context.Background())
	core.AssertTrue(t, shutdown.OK)
}

func TestService_Service_OnShutdown_Bad(t *core.T) {
	var s *Service
	r := s.OnShutdown(context.Background())
	core.AssertTrue(t, r.OK)
}

func TestService_Service_OnShutdown_Ugly(t *core.T) {
	svc := serviceForTest(t)
	svc.OnShutdown(context.Background())
	again := svc.OnShutdown(context.Background())
	core.AssertTrue(t, again.OK)
}

// --- end AX-7 compliance triplets ---
