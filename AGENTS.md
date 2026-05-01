# AGENTS.md

This repository follows the v0.9.0 core/go audit contract. Go source lives in
the `go/` subtree, and local development uses `go.work` with `./go` plus the
core dependency under `./external/go`.

Use core/go primitives directly instead of banned stdlib imports. Public
symbols require file-local Good, Bad, and Ugly tests plus examples.
