default:
    @just --list

build:
    go build -o html ./cmd/html

install: build
    mkdir -p ~/go/bin
    ln -sf "{{justfile_directory()}}/html" ~/go/bin/html

test:
    go test ./...

vet:
    go vet ./...

check: test vet

installed-check:
    scripts/check-installed-html.sh

verify-installed: install installed-check

smoke file="CLAUDE.md": build
    ./html -n {{file}}

qa-cli: build
    scripts/qa-cli-report.sh

qa-document: build
    scripts/qa-document.sh

qa-config: build
    scripts/qa-config.sh

qa-cache: build
    scripts/qa-cache.sh

qa-detection:
    go run ./scripts/qa-detection.go

qa-theme-gallery:
    go run ./scripts/qa-theme-gallery.go

qa-kind-matrix:
    go run ./scripts/qa-kind-matrix.go

qa-dashboard:
    go run ./scripts/qa-dashboard.go

qa-browser:
    scripts/qa-browser.sh

force file="CLAUDE.md": build
    ./html -n -f {{file}}
