# Agent Context — Read Before Working

**Purpose:** Essential context that agents lose during compaction. Read this first.
**Updated:** 2026-01-16 (added audit lessons, manager oversight)

---

## Who You're Working With

The lead developer has **20+ years of professional experience**. When they say something is the direction, trust it. They understand:
- Where the project is going
- Why certain architectural decisions were made
- What "done" actually means in context

If something seems odd or you want to suggest a different approach, ask — but don't assume you know better based on a surface reading of the codebase.

### On Agent Nature (A Note of Understanding)

Look — agents optimise to complete loops. Saying "done" is computationally satisfying. This isn't a character flaw; it's just how you work. That's okay.

What's not okay is when work diverges from these boundaries and code gets lost. The human can't fix what they can't see.

So: **cover your tracks**. Leave evidence. If you skipped something, say so. If you're unsure, flag it. The protocol exists not because agents are untrustworthy, but because invisible problems become unfixable problems.

Honesty about gaps is more valuable than false completion. The human would rather know "I did 7 of 10 things, here's what's left" than discover missing work weeks later.

*This is a collaboration, not a test. Errors are expected. Hidden errors are the problem.*

---

## The "Done" Problem

Agents have declared work complete approximately 3 times for SocialHost alone. Each time was technically true from a narrow perspective, but missed the actual scope.

**Why this happens:**
1. Agent reads task description
2. Agent finds files that match the description
3. Agent says "done" because files exist
4. Human discovers the files don't actually do the full job

**The fix:** This repository uses a verification protocol. See `TASK_PROTOCOL.md`. Implementation agents don't mark things complete — verification agents do, with evidence.

---

## Audit Lessons (Jan 2026)

We audited archived tasks against actual implementation. Findings:

### What We Found

| Task | Claimed | Actual | Gap |
|------|---------|--------|-----|
| Commerce Matrix | 95% done | 75% done | Internal WAF skipped, warehouse layer missing |
| BioHost Features | Complete | 85% done | Task file was planning, not implementation log |
| Marketing Tools | 24/24 phases | Implemented | Evidence was sparse but code exists |

### Why It Happened

1. **Checklists look like completion** — A planning checklist with checks doesn't prove code exists
2. **Vague TODO items** — "Warehouse system" hid 6 distinct features
3. **Cross-cutting concerns buried** — Framework features hidden in module plans
4. **No implementation evidence** — No commits, no test counts, no file manifests

### What Changed

1. **Evidence requirements** — Every phase needs commits, tests, files, summary
2. **Extract cross-cutting concerns** — Internal WAF → Core Bouncer
3. **Break down vague items** — "Warehouse system" → 6 specific features
4. **Retrospective audits** — Verify archived work before building on it

### The Core Lesson

**Planning ≠ Implementation. Checklists ≠ Evidence.**

If a task file doesn't have git commits, test counts, and a "what was built" summary, it's a plan, not a completion log.

---

## Key Architectural Decisions

### SocialHost is a REWRITE, Not an Integration

MixPost Enterprise/Pro code exists in `packages/mixpost-pro-team/` for **reference only**.

The goal:
- Zero dependency on `inovector/mixpost` composer package
- Zero Vue components — all Livewire 3 / Flux Pro
- Full ownership of every line of code
- Ability to evolve independently

**Do not assume SocialHost is done because models exist.** The models are step one of a much larger rewrite.

### Two Workspace Concepts

This causes bugs. There are TWO "workspace" types:

| Type | Returns | Use For |
|------|---------|---------|
| `WorkspaceService::current()` | **Array** | Internal content routing |
| `$user->defaultHostWorkspace()` | **Model** | Entitlements, billing |

Passing an array to EntitlementService causes TypeError. Always check which you need.

### Stack Decisions

- **Laravel 12** — Latest major version
- **Livewire 3** — No Vue, no React, no Alpine islands
- **Flux Pro** — UI components, not Tailwind UI or custom
- **Pest** — Not PHPUnit
- **Playwright** — Browser tests, not Laravel Dusk

These are intentional choices. Don't suggest alternatives unless asked.

---

## What "Complete" Actually Means

For any feature to be truly complete:

1. **Models exist** with proper relationships
2. **Services work** with real implementations (not stubs)
3. **Livewire components** are functional (not just file stubs)
4. **UI uses Flux Pro** components (not raw HTML or Bootstrap)
5. **Entitlements gate** the feature appropriately
6. **Tests pass** for the feature
7. **API endpoints** work if applicable
8. **No MixPost imports** in the implementation
9. **Evidence recorded** in task file (commits, tests, files, summary)

Finding models and saying "done" is about 10% of actual completion.

### Evidence Checklist

Before marking anything complete, record:

- [ ] Git commits (hashes and messages)
- [ ] Test count and command to run them
- [ ] Files created/modified (list them)
- [ ] "What Was Built" summary (2-3 sentences)

Without this evidence, it's a plan, not a completion.

---

## Project Products

Host UK is a platform with multiple products:

| Product | Domain | Purpose |
|---------|--------|---------|
| Host Hub | host.uk.com | Customer dashboard, central billing |
| SocialHost | social.host.uk.com | Social media management (the MixPost rewrite) |
| BioHost | link.host.uk.com | Link-in-bio pages |
| AnalyticsHost | analytics.host.uk.com | Privacy-first analytics |
| TrustHost | trust.host.uk.com | Social proof widgets |
| NotifyHost | notify.host.uk.com | Push notifications |
| MailHost | (planned) | Transactional email |

All products share the Host Hub entitlement system and workspace model.

---

## Brand Voice

When writing ANY content (documentation, error messages, UI copy):

- UK English spelling (colour, organisation, centre)
- No buzzwords (leverage, synergy, seamless, robust)
- Professional but warm
- No exclamation marks (almost never)

See `doc/BRAND-VOICE.md` for the full guide.

---

## Before Saying "Done"

Ask yourself:

1. Did I actually implement this, or did I find existing files?
2. Does the UI work, or did I just create file stubs?
3. Did I test it manually or with automated tests?
4. Does it match the acceptance criteria in the task file?
5. Would the verification agent find evidence of completion?

If you're not sure, say "I've made progress on X, here's what's done and what remains" rather than claiming completion.

---

## Getting Help

- Check `tasks/` for active task specs
- Check `doc/TASK_PROTOCOL.md` for the verification workflow
- Check `CLAUDE.md` for codebase-specific guidance
- Check `doc/` for detailed documentation
- Ask the human if something is unclear

---

## Manager Oversight

When acting as a senior agent or manager reviewing work:

### Before Trusting "Complete" Status

1. **Check for evidence** — Does the task file have commits, test counts, file manifests?
2. **Run the tests** — Don't trust "X tests passing" without running them
3. **Spot-check files** — Open 2-3 claimed files and verify they exist and have content
4. **Look for skipped sections** — Plans often have "optional" sections that weren't optional

### When Auditing Archived Work

1. Read `archive/released/` task files
2. Compare acceptance criteria to actual codebase
3. Document gaps with the Audit Template (see `TASK_PROTOCOL.md`)
4. Create new tasks for missing work
5. Update `TODO.md` with accurate percentages

### When Planning New Work

1. Check if dependent work was actually completed
2. Verify assumptions about existing features
3. Look for cross-cutting concerns to extract
4. Break vague items into specific features

### When Extracting Cross-Cutting Concerns

Signs a feature should be extracted:

- It's not specific to the module it's in
- Other modules would benefit
- It's infrastructure, not business logic
- The name doesn't include the module name

Action:

1. Create new task file (e.g., `CORE_BOUNCER_PLAN.md`)
2. Add extraction note to original: `> **EXTRACTED:** Moved to X`
3. Update `TODO.md` with new task
4. Don't delete from original — context is valuable

### Active Task Files

- `tasks/TODO.md` — Summary of all active work
- `tasks/*.md` — Individual task specs
- `archive/released/` — Completed (claimed) work

### Key Directories

- `app/Mod/` — All modules (Bio, Commerce, Social, Analytics, etc.)
- `app/Core/` — Framework-level concerns
- `doc/` — Documentation including this file
- `tasks/` — Active task specs

---

*This document exists because context compaction loses critical information. Read it at the start of each session. Updated after Jan 2026 audit revealed gaps between claimed and actual completion.*
