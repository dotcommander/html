# Contributing

Thank you for contributing to `html`.

1. Open an issue before substantial behavior or architecture changes.
2. Keep changes focused and preserve the package boundaries described in
   `AGENTS.md`.
3. Use Go 1.26.3 or newer and avoid new dependencies unless they are necessary.
4. Add package-local tests for behavior changes.
5. Run `just check` and `git diff --check` before submitting a pull request.

For rendered UI changes, also run `just qa-browser` and include representative
screenshots or generated HTML. Never include secrets, private source documents,
or machine-specific paths in fixtures or reports.
