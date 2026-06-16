# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go CLI module, `github.com/dotcommander/html`, for rendering Markdown or plain input into self-contained HTML. The executable entry point lives in `cmd/html/main.go`. Core behavior is under `internal/`: `cli` wires Cobra flags, `actions` orchestrates input/cache/render/open flow, `config` loads optional user config, `open` launches the browser, and `render` owns Markdown/plain rendering plus embedded assets. CSS and browser-side scripts are embedded from `internal/render/assets/`. Tests sit beside the package code as `*_test.go`. Static prototype examples live under `prototypes/`; shell verification helpers live in `scripts/`.

## Build, Test, and Development Commands

- `just build` or `go build -o html ./cmd/html`: build the local CLI binary.
- `just install`: build and symlink `./html` into `~/go/bin/html`.
- `just test` or `go test ./...`: run the full Go test suite.
- `just vet` or `go vet ./...`: run Go static checks.
- `just check`: run tests and vet.
- `just smoke file=README.md`: build and render a file with `./html -n`.
- `just verify-installed`: install, then verify the PATH-visible `html` binary with `scripts/check-installed-html.sh`.

## Coding Style & Naming Conventions

Use standard Go formatting (`gofmt`) on touched Go files. Keep package boundaries small and aligned with existing `internal/*` responsibilities. Prefer standard-library solutions unless `go.mod` already carries the needed dependency. Keep CLI behavior in `internal/cli`, orchestration in `internal/actions`, rendering in `internal/render`, and user configuration in `internal/config`; avoid hardcoding user-facing defaults in unrelated packages. Name tests after behavior, for example `TestRun...` or `TestRender...`.

## Testing Guidelines

Add or update package-local `*_test.go` files when behavior changes. Existing tests use Go's `testing` package plus `testify` where helpful. Prefer `t.TempDir()` and `t.Cleanup()` for filesystem tests. Run `go test ./...` before finishing changes; add `go test -race ./...` for concurrency or shared-state changes.

## Commit & Pull Request Guidelines

Recent commits use conventional prefixes such as `feat(render): ...`, `test(internal): ...`, `docs: ...`, and `chore(internal): ...`. Keep commit subjects imperative and scoped when useful. Pull requests should include the user-visible change, touched packages, exact verification commands, and screenshots or sample rendered HTML when UI output changes.

## Security & Configuration Tips

Raw HTML passthrough is allowed for trusted local Markdown; use `--safe` for untrusted input. Optional config belongs in `~/.config/html/config.json`; new behavior knobs should be documented there and threaded through the existing config path.
