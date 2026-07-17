# Changelog

All notable changes to this project will be documented in this file.

## v0.2.1 - 2026-07-16

### Quality

- Centralize static embedded stylesheet colors and font stacks into semantic design tokens.
- Enforce design-token usage across static renderer stylesheets with tests.
- Expand browser QA for control wrapping and text contrast, focus visibility, copy feedback, tabs, and slide controls.

## v0.2.0 - 2026-07-16

### Features

- Render GitHub-style Markdown alerts in trusted and safe documents.
- Add accessible horizontal bar-chart reports with bounded fallback diagnostics.
- Promote qualifying ordered-list sections into source-backed report timelines.
- Keep relative Markdown links working from trusted cached file renders.
- Report stable, deduplicated warnings when local images cannot be embedded.

### Fixes

- Match GitHub alert recognition boundaries for empty and nested blockquotes.
- Verify both development and exact-tag versions of the installed CLI.

### Quality

- Add golden, metamorphic, browser, and GitHub Markdown conformance coverage.

## v0.1.0 - 2026-07-15

Initial source release of the `html` CLI, including Markdown and plain-text
rendering, embedded themes and assets, local-image handling, deterministic
report layouts, safe mode, content-aware caching, and portable `go install`
support.
