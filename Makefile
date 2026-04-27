.PHONY: setup build test fmt vet tidy

# `setup` wires the in-repo git hooks. It is idempotent and is a prerequisite
# of `build` and `test`, so any common dev command re-asserts the hook config.
setup:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/* scripts/*.sh
	@echo "hooks active: core.hooksPath = $$(git config core.hooksPath)"

build: setup
	go build ./...

test: setup
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
