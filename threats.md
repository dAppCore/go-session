## 1. Parser DoS

Status: Findings landed

Question: Can an attacker force unbounded parser memory with many large JSONL lines or unmatched tool calls?

Finding: Partial yes. The scanner is bounded to 8 MiB per token, and it now starts with a 64 KiB buffer instead of allocating 8 MiB up front (`parser.go:18`, `parser.go:357-358`). It does not retain N scanner buffers for N lines. However, unmatched `tool_use` records were previously retained in `pendingTools` until EOF and had no count limit; this is now capped at 4096 pending calls (`parser.go:22-23`, `parser.go:430-433`). Tool inputs are now truncated before they are stored in `pendingTools`, so an unmatched Bash command cannot keep an entire scanner-sized line resident (`parser.go:435-439`).

Severity: Medium before fix. Requires attacker-controlled transcript content, but memory growth was linear in unmatched tool_use count and input size.

Coverage: Added `TestParser_ParseTranscriptToolUseInputTruncated_Bad` and `TestParser_ParseTranscriptPendingToolLimit_Bad` (`parser_test.go:1099`, `parser_test.go:1115`).

## 2. Malformed JSONL

Status: No exploitable finding; coverage added

Question: Do malformed or adversarial JSONL records panic or bypass type handling?

Finding: No exploitable parser bug found. Bad top-level JSON is skipped with stats (`parser.go:376-386`), malformed assistant/user messages and content blocks are skipped (`parser.go:404-413`, `parser.go:445-454`), and unexpected tool result/input types fall through type switches without panicking (`parser.go:568-576`, `parser.go:579-598`). Deeply nested JSON is handled through `encoding/json` via core helpers and returned as a normal unmarshal failure, not a panic.

Severity: Low. The remaining cost is bounded by the per-line scanner maximum and the JSON decoder's own validation.

Coverage: Added tests for deeply nested JSON, unexpected tool input/result types, and lone UTF-16 surrogate halves (`parser_test.go:1133`, `parser_test.go:1147`, `parser_test.go:1161`).

## 3. Path traversal

Status: Finding landed

Question: Can FetchSession or ListSessions escape projectsDir through encoded traversal, symlinks, case-insensitive paths, or Windows-style paths?

Finding: Yes for symlinks before fix. `FetchSession` rejected literal `..`, `/`, and `\` in IDs (`parser.go:284-286`), so URL-encoded `..` remains a literal filename unless a caller decodes it before calling; if decoded first, the existing check rejects it. The real gap was that a `linked.jsonl` symlink inside projectsDir could point outside and still be opened/listed because normal stat/open operations follow symlinks. FetchSession now rejects symlink targets (`parser.go:289-292`, `parser.go:616-617`), and ListSessions skips symlink matches before stat/open (`parser.go:156-162`). The local path style is still POSIX-oriented via `path.Join`; Windows UNC behavior is not fully addressed in this package.

Severity: Medium before fix. Exploitation requires ability to place a symlink in projectsDir, but then reads can escape the intended session directory.

Coverage: Added URL-encoded traversal, FetchSession symlink traversal, and ListSessions symlink traversal tests (`parser_test.go:1508`, `parser_test.go:1516`, `parser_test.go:1562`).
