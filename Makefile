# warehouse-ops-agent — local quality gate.
#
# Mirrors the sensors the five sibling bounded-context repos run (see
# fulfillment-execution/Makefile, the pilot). `make check` is the fast
# self-correction loop; `make check-all` is the fuller pre-push gate. This
# module has no HTTP/gRPC API of its own yet and no domain aggregate at all
# (see internal/domain/policy/doc.go), so there is no coverage/mutation/bdd
# target — those land with whichever T-card first adds decision-policy logic
# and its own inbound adapter.

GOLANGCI_LINT_VERSION := v2.13.1

.PHONY: help build vet fmt fmt-check lint test arch-test check check-all

help: ## Show the available targets
	@echo "warehouse-ops-agent — make targets"
	@echo ""
	@echo "  help          Print this message (default target)"
	@echo "  build         go build ./..."
	@echo "  vet           go vet ./..."
	@echo "  fmt           gofmt -w . (formats in place)"
	@echo "  fmt-check     Fail if any file is not gofmt-clean"
	@echo "  lint          golangci-lint run ./... (pinned $(GOLANGCI_LINT_VERSION) in CI)"
	@echo "  test          go test ./... -race"
	@echo "  arch-test     Hexagonal architecture fitness tests (arch-go)"
	@echo "  check         FAST pre-commit bundle: fmt-check vet build lint test"
	@echo "  check-all     check + arch-test (pre-push gate)"
	@echo ""
	@echo "  Hooks: run 'lefthook install' once to activate the pre-commit/pre-push hooks."

build: ## go build ./...
	go build ./...

vet: ## go vet ./...
	go vet ./...

fmt: ## Format the tree in place
	gofmt -w .

fmt-check: ## Fail if the tree is not gofmt-clean
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt: the following files are not formatted:"; \
		echo "$$out"; \
		echo "run 'make fmt' to fix"; \
		exit 1; \
	fi; \
	echo "gofmt: clean"

lint: ## golangci-lint run ./...
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint is not installed."; \
		echo "install the version CI pins with:"; \
		echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"; \
		exit 1; \
	fi
	golangci-lint run ./...

test: ## Unit tests (no DB, no HTTP server yet — see arch-test)
	go test ./... -race

arch-test: ## Architecture fitness tests
	go test ./internal/architecture/... -v

check: fmt-check vet build lint test ## Fast pre-commit bundle
	@echo "check: OK"

check-all: check arch-test ## Fuller pre-push gate
	@echo "check-all: OK"
