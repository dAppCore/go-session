# Host Hub Task Protocol

**Version:** 2.1
**Created:** 2026-01-01
**Updated:** 2026-01-16
**Purpose:** Ensure agent work is verified before being marked complete, and provide patterns for efficient parallel implementation.

> **Lesson learned (Jan 2026):** Task files written as checklists without implementation evidence led to 6+ "complete" tasks that were actually 70-85% done. Planning ≠ implementation. Evidence required.

---

## The Problem

Agents optimise for conversation completion, not task completion. Saying "done" is computationally cheaper than doing the work. Context compaction loses task state. Nobody verifies output against spec.

## The Solution

Separation of concerns:
1. **Planning Agent** — writes the spec
2. **Implementation Agent** — does the work
3. **Verification Agent** — checks the work against spec
4. **Human** — approves or rejects based on verification

---

## Directory Structure

```
doc/
├── TASK_PROTOCOL.md          # This file
└── ...                       # Reference documentation

tasks/
├── TODO.md                   # Active task summary
├── TASK-XXX-feature.md       # Active task specs
├── agentic-tasks/            # Agentic system tasks
└── future-products/          # Parked product plans

archive/
├── released/                 # Completed tasks (for reference)
└── ...                       # Historical snapshots
```

---

## Task File Schema

Every task file follows this structure:

```markdown
# TASK-XXX: [Short Title]

**Status:** draft | ready | in_progress | needs_verification | verified | approved
**Created:** YYYY-MM-DD
**Last Updated:** YYYY-MM-DD HH:MM by [agent/human]
**Assignee:** [agent session or human]
**Verifier:** [different agent session]

---

## Objective

[One paragraph: what does "done" look like?]

---

## Acceptance Criteria

- [ ] AC1: [Specific, verifiable condition]
- [ ] AC2: [Specific, verifiable condition]
- [ ] AC3: [Specific, verifiable condition]

Each criterion must be:
- Binary (yes/no, not "mostly")
- Verifiable by code inspection or test
- Independent (can check without context)

---

## Implementation Checklist

- [ ] File: `path/to/file.php` — [what it should contain]
- [ ] File: `path/to/other.php` — [what it should contain]
- [ ] Test: `tests/Feature/XxxTest.php` passes
- [ ] Migration: runs without error

---

## Verification Results

### Check 1: [Date] by [Agent]

| Criterion | Status | Evidence |
|-----------|--------|----------|
| AC1 | ✅ PASS | File exists at path, contains X |
| AC2 | ❌ FAIL | Missing method Y in class Z |
| AC3 | ⚠️ PARTIAL | 3 of 5 tests pass |

**Verdict:** FAIL — AC2 not met

### Check 2: [Date] by [Agent]

| Criterion | Status | Evidence |
|-----------|--------|----------|
| AC1 | ✅ PASS | File exists at path, contains X |
| AC2 | ✅ PASS | Method Y added, verified |
| AC3 | ✅ PASS | All 5 tests pass |

**Verdict:** PASS — ready for human approval

---

## Notes

[Any context, blockers, decisions made during implementation]
```

---

## Implementation Evidence (Required)

**A checklist is not evidence. Prove the work exists.**

Every completed phase MUST include:

### 1. Git Evidence
```markdown
**Commits:**
- `abc123` - Add Domain model and migration
- `def456` - Add DomainController with CRUD
- `ghi789` - Add 28 domain tests
```

### 2. Test Count
```markdown
**Tests:** 28 passing (run: `php artisan test app/Mod/Bio/Tests/Feature/DomainTest.php`)
```

### 3. File Manifest
```markdown
**Files created/modified:**
- `app/Mod/Bio/Models/Domain.php` (new)
- `app/Mod/Bio/Http/Controllers/DomainController.php` (new)
- `database/migrations/2026_01_16_create_domains_table.php` (new)
- `app/Mod/Bio/Tests/Feature/DomainTest.php` (new)
```

### 4. "What Was Built" Summary
```markdown
**Summary:** Custom domain management with DNS verification. Users can add domains,
system generates TXT record for verification, background job checks DNS propagation.
Includes SSL provisioning via Caddy API.
```

### Why This Matters

In Jan 2026, an audit found:
- Commerce Matrix Plan marked "95% done" was actually 75%
- Internal WAF section was skipped entirely (extracted to Core Bouncer)
- Warehouse/fulfillment (6 features) listed as "one item" in TODO
- Task files read like planning documents, not completion logs

**Without evidence, "done" means nothing.**

---

## Workflow

### 1. Task Creation

Human or planning agent creates task file in `tasks/`:
- Status: `draft`
- Must have clear acceptance criteria
- Must have implementation checklist

### 2. Task Ready

Human reviews and sets:
- Status: `ready`
- Assignee: `next available agent`

### 3. Implementation

Implementation agent:
- Sets status: `in_progress`
- Works through implementation checklist
- Checks boxes as work is done
- When complete, sets status: `needs_verification`
- **MUST NOT** mark acceptance criteria as passed

### 4. Verification

Different agent (verification agent):
- Reads the task file
- Independently checks each acceptance criterion
- Records evidence in Verification Results section
- Sets verdict: PASS or FAIL
- If PASS: status → `verified`, move to `archive/released/`
- If FAIL: status → `in_progress`, back to implementation agent

### 5. Human Approval

Human reviews verified task:
- Spot-check the evidence
- If satisfied: status → `approved`, can delete or keep in archive
- If not: back to `needs_verification` with notes

---

## Agent Instructions

### For Implementation Agents

```
You are implementing TASK-XXX.

1. Read the full task file
2. Set status to "in_progress"
3. Work through the implementation checklist
4. Check boxes ONLY for work you have completed
5. When done, set status to "needs_verification"
6. DO NOT check acceptance criteria boxes
7. DO NOT mark the task as complete
8. Update "Last Updated" with current timestamp

Your job is to do the work, not to verify it.
```

### For Verification Agents

```
You are verifying TASK-XXX.

1. Read the full task file
2. For EACH acceptance criterion:
   a. Check the codebase independently
   b. Record what you found (file paths, line numbers, test output)
   c. Mark as PASS, FAIL, or PARTIAL with evidence
3. Add a new "Verification Results" section with today's date
4. Set verdict: PASS or FAIL
5. If PASS: move file to archive/released/
6. If FAIL: set status back to "in_progress"
7. Update "Last Updated" with current timestamp

You are the gatekeeper. Be thorough. Trust nothing the implementation agent said.
```

---

## Status Flow

```
draft → ready → in_progress → needs_verification → verified → approved
                     ↑                    │
                     └────────────────────┘
                        (if verification fails)
```

---

## Phase-Based Decomposition

Large tasks should be decomposed into independent phases that can be executed in parallel by multiple agents. This dramatically reduces implementation time.

### Phase Independence Rules

1. **No shared state** — Each phase writes to different files/tables
2. **No blocking dependencies** — Phase 3 shouldn't wait for Phase 2's output
3. **Clear boundaries** — Each phase has its own acceptance criteria
4. **Testable isolation** — Phase tests don't require other phases

### Example Decomposition

A feature like "BioHost Missing Features" might decompose into:

| Phase | Focus | Can Parallel With |
|-------|-------|-------------------|
| 1 | Domain Management | 2, 3, 4 |
| 2 | Project System | 1, 3, 4 |
| 3 | Analytics Core | 1, 2, 4 |
| 4 | Form Submissions | 1, 2, 3 |
| 5 | Link Scheduling | 1, 2, 3, 4 |
| ... | ... | ... |
| 12 | MCP Tools (polish) | After 1-11 |
| 13 | Admin UI (polish) | After 1-11 |

### Phase Sizing

- **Target**: 4-8 acceptance criteria per phase
- **Estimated time**: 2-4 hours per phase
- **Test count**: 15-40 tests per phase
- **File count**: 3-10 files modified per phase

---

## Standard Phase Types

Every large task should include these phase types:

### Core Implementation Phases (1-N)

The main feature work. Group by:
- **Resource type** (domains, projects, analytics)
- **Functional area** (CRUD, scheduling, notifications)
- **Data flow** (input, processing, output)

### Polish Phase: MCP Tools

**Always include as second-to-last phase.**

Exposes all implemented features to AI agents via MCP protocol.

Standard acceptance criteria:
- [ ] MCP tool class exists at `app/Mcp/Tools/{Feature}Tools.php`
- [ ] All CRUD operations exposed as actions
- [ ] Tool includes prompts for common workflows
- [ ] Tool includes resources for data access
- [ ] Tests verify all MCP actions return expected responses
- [ ] Tool registered in MCP service provider

### Polish Phase: Admin UI Integration

**Always include as final phase.**

Integrates features into the admin dashboard.

Standard acceptance criteria:
- [ ] Sidebar navigation updated with feature section
- [ ] Index/list page with filtering and search
- [ ] Detail/edit pages for resources
- [ ] Bulk actions where appropriate
- [ ] Breadcrumb navigation
- [ ] Role-based access control
- [ ] Tests verify all admin routes respond correctly

---

## Parallel Agent Execution

### Firing Multiple Agents

When phases are independent, fire agents simultaneously:

```
Human: "Implement phases 1-4 in parallel"

Agent fires 4 Task tools simultaneously:
- Task(Phase 1: Domain Management)
- Task(Phase 2: Project System)
- Task(Phase 3: Analytics Core)
- Task(Phase 4: Form Submissions)
```

### Agent Prompt Template

```
You are implementing Phase X of TASK-XXX: [Task Title]

Read the task file at: tasks/TASK-XXX-feature-name.md

Your phase covers acceptance criteria ACxx through ACyy.

Implementation requirements:
1. Create all files listed in the Phase X implementation checklist
2. Write comprehensive Pest tests (target: 20-40 tests)
3. Follow existing codebase patterns
4. Use workspace-scoped multi-tenancy
5. Check entitlements for tier-gated features

When complete:
1. Update the task file marking Phase X checklist items done
2. Report: files created, test count, any blockers

Do NOT mark acceptance criteria as passed — verification agent does that.
```

### Coordination Rules

1. **Linter accepts all** — Configure to auto-accept agent file modifications
2. **No merge conflicts** — Phases write to different files
3. **Collect results** — Wait for all agents, then fire next wave
4. **Wave pattern** — Group dependent phases into waves

### Wave Execution Example

```
Wave 1 (parallel): Phases 1, 2, 3, 4
  ↓ (all complete)
Wave 2 (parallel): Phases 5, 6, 7, 8
  ↓ (all complete)
Wave 3 (parallel): Phases 9, 10, 11
  ↓ (all complete)
Wave 4 (sequential): Phase 12 (MCP), then Phase 13 (UI)
```

---

## Task File Schema (Extended)

For large phased tasks, extend the schema:

```markdown
# TASK-XXX: [Feature Name]

**Status:** draft | ready | in_progress | needs_verification | verified | approved
**Created:** YYYY-MM-DD
**Last Updated:** YYYY-MM-DD HH:MM by [agent/human]
**Complexity:** small (1-3 phases) | medium (4-8 phases) | large (9+ phases)
**Estimated Phases:** N
**Completed Phases:** M/N

---

## Objective

[One paragraph: what does "done" look like?]

---

## Scope

- **Models:** X new, Y modified
- **Migrations:** Z new tables
- **Livewire Components:** A new
- **Tests:** B target test count
- **Estimated Hours:** C-D hours

---

## Phase Overview

| Phase | Name | Status | ACs | Tests |
|-------|------|--------|-----|-------|
| 1 | Domain Management | ✅ Done | AC1-5 | 28 |
| 2 | Project System | ✅ Done | AC6-10 | 32 |
| 3 | Analytics Core | 🔄 In Progress | AC11-16 | - |
| ... | ... | ... | ... | ... |
| 12 | MCP Tools | ⏳ Pending | AC47-53 | - |
| 13 | Admin UI | ⏳ Pending | AC54-61 | - |

---

## Acceptance Criteria

### Phase 1: Domain Management

- [ ] AC1: [Criterion]
- [ ] AC2: [Criterion]
...

### Phase 12: MCP Tools (Standard)

- [ ] AC47: MCP tool class exists with all feature actions
- [ ] AC48: CRUD operations for all resources exposed
- [ ] AC49: Bulk operations exposed (where applicable)
- [ ] AC50: Query/filter operations exposed
- [ ] AC51: MCP prompts created for common workflows
- [ ] AC52: MCP resources expose read-only data access
- [ ] AC53: Tests verify all MCP actions

### Phase 13: Admin UI Integration (Standard)

- [ ] AC54: Sidebar updated with feature navigation
- [ ] AC55: Feature has expandable submenu (if 3+ pages)
- [ ] AC56: Index pages with DataTable/filtering
- [ ] AC57: Create/Edit forms with validation
- [ ] AC58: Detail views with related data
- [ ] AC59: Bulk action support
- [ ] AC60: Breadcrumb navigation
- [ ] AC61: Role-based visibility

---

## Implementation Checklist

### Phase 1: Domain Management
- [ ] File: `app/Models/...`
- [ ] File: `app/Livewire/...`
- [ ] Test: `tests/Feature/...`

### Phase 12: MCP Tools
- [ ] File: `app/Mcp/Tools/{Feature}Tools.php`
- [ ] File: `app/Mcp/Prompts/{Feature}Prompts.php` (optional)
- [ ] File: `app/Mcp/Resources/{Feature}Resources.php` (optional)
- [ ] Test: `tests/Feature/Mcp/{Feature}ToolsTest.php`

### Phase 13: Admin UI
- [ ] File: `resources/views/admin/components/sidebar.blade.php` (update)
- [ ] File: `app/Livewire/Admin/{Feature}/Index.php`
- [ ] File: `resources/views/livewire/admin/{feature}/index.blade.php`
- [ ] Test: `tests/Feature/Admin/{Feature}Test.php`

---

## Verification Results

[Same as before]

---

## Phase Completion Log

### Phase 1: Domain Management
**Completed:** YYYY-MM-DD by [Agent ID]
**Tests:** 28 passing
**Files:** 8 created/modified
**Notes:** [Any context]

### Phase 2: Project System
**Completed:** YYYY-MM-DD by [Agent ID]
**Tests:** 32 passing
...
```

---

## MCP Endpoint (Future)

When implemented, the MCP endpoint will expose:

```
GET  /tasks                    # List all tasks with status
GET  /tasks/{id}               # Get task details
POST /tasks/{id}/claim         # Agent claims a task
POST /tasks/{id}/complete      # Agent marks ready for verification
POST /tasks/{id}/verify        # Verification agent submits results
GET  /tasks/next               # Get next unclaimed task
GET  /tasks/verify-queue       # Get tasks needing verification
POST /tasks/{id}/phases/{n}/claim    # Claim specific phase
POST /tasks/{id}/phases/{n}/complete # Complete specific phase
GET  /tasks/{id}/phases              # List phase status
```

---

## Metrics to Track

- Tasks created vs completed (per week)
- Verification pass rate on first attempt
- Average time from ready → approved
- Most common failure reasons

---

## Cross-Cutting Concerns

When a feature applies to multiple modules, extract it.

### Example: Core Bouncer

The Commerce Matrix Plan included an "Internal WAF" section — a request whitelisting system with training mode. During audit, we realised:

- It's not commerce-specific
- It applies to all admin routes, all API endpoints
- It should be in `Core/`, not `Commerce/`

**Action:** Extracted to `CORE_BOUNCER_PLAN.md` as a framework-level concern.

### Signs to Extract

- Feature name doesn't include the module name naturally
- You'd copy-paste it to other modules
- It's about infrastructure, not business logic
- Multiple modules would benefit independently

### How to Extract

1. Create new task file for the cross-cutting concern
2. Add note to original plan: `> **EXTRACTED:** Section moved to X`
3. Update TODO.md with the new task
4. Don't delete from original — leave the note for context

---

## Retrospective Audits

Periodically audit archived tasks against actual implementation.

### When to Audit

- Before starting dependent work
- When resuming a project after a break
- When something "complete" seems broken
- Monthly for active projects

### Audit Process

1. Read the archived task file
2. Check each acceptance criterion against codebase
3. Run the tests mentioned in the task
4. Document gaps found

### Audit Template

```markdown
## Audit: TASK-XXX
**Date:** YYYY-MM-DD
**Auditor:** [human/agent]

| Claimed | Actual | Gap |
|---------|--------|-----|
| Phase 1 complete | ✅ Verified | None |
| Phase 2 complete | ⚠️ Partial | Missing X service |
| Phase 3 complete | ❌ Not done | Only stubs exist |

**Action items:**
- [ ] Create TASK-YYY for Phase 2 gap
- [ ] Move Phase 3 back to TODO as incomplete
```

---

## Anti-Patterns to Avoid

### General

1. **Same agent implements and verifies** — defeats the purpose
2. **Vague acceptance criteria** — "it works" is not verifiable
3. **Skipping verification** — the whole point is independent checking
4. **Bulk marking as done** — verify one task at a time
5. **Human approving without spot-check** — trust but verify

### Evidence & Documentation

6. **Checklist without evidence** — planning ≠ implementation
7. **Skipping "What Was Built" summary** — context lost on compaction
8. **No test count** — can't verify without knowing what to run
9. **Marking section "done" without implementation** — major gaps discovered in audits
10. **Vague TODO items** — "Warehouse system" hides 6 distinct features

### Parallel Execution

11. **Phases with shared files** — causes merge conflicts
12. **Sequential dependencies in same wave** — blocks parallelism
13. **Skipping polish phases** — features hidden from agents and admins
14. **Too many phases per wave** — diminishing returns past 4-5 agents
15. **No wave boundaries** — chaos when phases actually do depend

### MCP Tools

16. **Exposing without testing** — broken tools waste agent time
17. **Missing bulk operations** — agents do N calls instead of 1
18. **No error context** — agents can't debug failures

### Admin UI

19. **Flat navigation for large features** — use expandable submenus
20. **Missing breadcrumbs** — users get lost
21. **No bulk actions** — tedious admin experience

### Cross-Cutting Concerns

22. **Burying framework features in module plans** — extract them
23. **Assuming module-specific when it's not** — ask "would other modules need this?"

---

## Quick Reference: Creating a New Task

1. Copy the extended schema template
2. Fill in objective and scope
3. Decompose into phases (aim for 4-8 ACs each)
4. Map phase dependencies → wave structure
5. Check for cross-cutting concerns — extract if needed
6. **Always add Phase N-1: MCP Tools**
7. **Always add Phase N: Admin UI Integration**
8. Set status to `draft`, get human review
9. When `ready`, fire Wave 1 agents in parallel
10. Collect results with evidence (commits, tests, files)
11. Fire next wave
12. After all phases, run verification agent
13. Human approval → move to `archive/released/`

---

## Quick Reference: Completing a Phase

1. Do the work
2. Run the tests
3. Record evidence:
   - Git commits (hashes + messages)
   - Test count and command to run them
   - Files created/modified
   - "What Was Built" summary (2-3 sentences)
4. Update task file with Phase Completion Log entry
5. Set phase status to ✅ Done
6. Move to next phase or request verification

---

## Quick Reference: Auditing Archived Work

1. Read `archive/released/` task file
2. For each phase marked complete:
   - Check files exist
   - Run listed tests
   - Verify against acceptance criteria
3. Document gaps using Audit Template
4. Create new tasks for missing work
5. Update TODO.md with accurate status

---

*This protocol exists because agents lie (unintentionally). The system catches the lies. Parallel execution makes them lie faster, so we verify more. Evidence requirements ensure lies are caught before archiving.*
