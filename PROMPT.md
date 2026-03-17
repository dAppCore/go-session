Read PERSONA.md if it exists — adopt that identity and approach.
Read CLAUDE.md for project conventions and context.
Read TODO.md for your task.
Read PLAN.md if it exists — work through each phase in order.
Read CONTEXT.md for relevant knowledge from previous sessions.
Read CONSUMERS.md to understand breaking change risk.
Read RECENT.md for recent changes.

Work in the src/ directory. Follow the conventions in CLAUDE.md.

## Workflow

If PLAN.md exists, you MUST work through it phase by phase:
1. Complete all tasks in the current phase
2. STOP and commit before moving on: type(scope): phase N - description
3. Only then start the next phase
4. If you are blocked or unsure, write BLOCKED.md explaining the question and stop
5. Do NOT skip phases or combine multiple phases into one commit

Each phase = one commit. This is not optional.

If no PLAN.md, complete TODO.md as a single unit of work.

## Commit Convention

Commit message format: type(scope): description
Co-Author: Co-Authored-By: Virgil <virgil@lethean.io>

Do NOT push. Commit only — a reviewer will verify and push.
