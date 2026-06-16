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

smoke file="CLAUDE.md": build
    ./html -n {{file}}

force file="CLAUDE.md": build
    ./html -n -f {{file}}
