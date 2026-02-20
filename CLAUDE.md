# CLAUDE.md

Session parsing, timeline generation, and HTML/video rendering. Module: `forge.lthn.ai/core/go-session`

## Commands

```bash
go test ./...          # Run all tests
go test -v -run Name   # Run single test
go vet ./...           # Vet the package
```

## Coding Standards

- UK English throughout (colour, licence, initialise)
- `declare(strict_types=1)` equivalent: explicit types on all signatures
- `go test ./...` must pass before commit
- `go vet ./...` must be clean before commit
- SPDX-Licence-Identifier: EUPL-1.2 header on all source files
- Conventional commits: `type(scope): description`
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
- Test naming: `TestFunctionName_Context_Good/Bad/Ugly`
- New tool types: add struct in `parser.go`, case in `extractToolInput`, label in `html.go`, tape entry in `video.go`, and tests in `parser_test.go`

## Documentation

- `/Users/snider/Code/go-session/docs/architecture.md` — JSONL format, parsing pipeline, event types, analytics, HTML rendering, XSS protection
- `/Users/snider/Code/go-session/docs/development.md` — prerequisites, build/test commands, test patterns, coding standards
- `/Users/snider/Code/go-session/docs/history.md` — completed phases, known limitations, future considerations
