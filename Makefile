# warehouse-ops-agent — local quality gate.
#
# Mirrors the sensors the five sibling bounded-context repos run (see
# fulfillment-execution/Makefile, the pilot). `make check` is the fast
# self-correction loop; `make check-all` is the fuller pre-push gate. This
# module has no HTTP/gRPC API of its own yet, no persisted aggregate, and no
# inbound adapter (see internal/domain/policy/doc.go), so there is still no
# mutation/bdd target — those land with whichever later T-card first adds an
# inbound adapter and its own acceptance surface. The coverage target lands
# with T2, which is the first slice to add decision-policy/use-case code.

GOLANGCI_LINT_VERSION := v2.13.1
COVERAGE_THRESHOLD    := 90
COVERPKG              := ./internal/domain/...,./internal/application/...

.PHONY: help build vet fmt fmt-check lint test coverage arch-test check check-all

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
	@echo "  coverage      Coverage run + $(COVERAGE_THRESHOLD)% gate (same command as CI)"
	@echo "  arch-test     Hexagonal architecture fitness tests (arch-go)"
	@echo "  check         FAST pre-commit bundle: fmt-check vet build lint test"
	@echo "  check-all     check + coverage arch-test (pre-push gate)"
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

coverage: ## Coverage run plus the CI coverage gate
	go test ./... -race -coverprofile=coverage.out -coverpkg=$(COVERPKG)
	@COVERAGE=$$(go tool cover -func=coverage.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
	echo "Coverage: $${COVERAGE}% (gate: $(COVERAGE_THRESHOLD)%)"; \
	if awk -v c="$$COVERAGE" -v t="$(COVERAGE_THRESHOLD)" 'BEGIN { exit !(c < t) }'; then \
		echo "coverage $${COVERAGE}% is below the $(COVERAGE_THRESHOLD)% gate"; \
		exit 1; \
	fi

arch-test: ## Architecture fitness tests
	go test ./internal/architecture/... -v

check: fmt-check vet build lint test ## Fast pre-commit bundle
	@echo "check: OK"

check-all: check coverage arch-test ## Fuller pre-push gate
	@echo "check-all: OK"
