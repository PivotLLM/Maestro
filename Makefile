# Maestro — build and test entry points.
#
#   make          run the full test suite, then build (binaries only if tests pass)
#   make test     the one gate: vet, go test -race, and the MCP regression (test.sh)
#   make build    build the maestro binary at the project root (no tests)
#   make clean    remove build output, test artifacts and the Go test cache
#   make install  install the built binary: /usr/local/bin when run as root,
#                 otherwise ~/bin (which must already exist)
#
# test.sh needs probe (MCPProbe), jq and zip on PATH; it reports the missing
# tool and exits non-zero rather than skipping.

.PHONY: all test build clean install fmt vet lint

BINARY   := maestro
GO       ?= go
PROBE    ?= probe
TEST_LOG := .test-go.log

# Install destination: machine-wide for root, per-user otherwise. Override with
# INSTALL_DIR=... to install elsewhere.
ifeq ($(shell id -u),0)
INSTALL_DIR ?= /usr/local/bin
else
INSTALL_DIR ?= $(HOME)/bin
endif

all: test build

## build: compile the maestro binary at the project root.
build:
	$(GO) build -o $(BINARY) .

## test: the full regression suite. Exits non-zero on any failure.
test:
	@echo "== go vet"
	@$(GO) vet ./...
	@echo "== go test -race"
	@$(GO) test -race -count=1 ./... > $(TEST_LOG) 2>&1; rc=$$?; \
	  grep -v '^Info:\|^Warning:' $(TEST_LOG); \
	  if [ $$rc -ne 0 ]; then echo "FAIL: go test exited $$rc (see above)"; rm -f $(TEST_LOG); exit $$rc; fi; \
	  echo "go test: $$(grep -c '^ok' $(TEST_LOG)) packages passed, 0 failed"; rm -f $(TEST_LOG)
	@echo "== test.sh (MCP regression)"
	@PROBE="$(PROBE)" ./test.sh
	@echo ""
	@echo "== SUMMARY: go vet clean; go test all packages passed; test.sh all checks passed (counts above)"

## clean: build output, test artifacts and the Go test cache.
clean:
	rm -f $(BINARY) $(TEST_LOG) .config-test-runtime.json
	rm -rf maestro-test
	$(GO) clean -testcache

## install: install the built binary into INSTALL_DIR (depends on build).
install: build
	@test -d "$(INSTALL_DIR)" || { echo "install: $(INSTALL_DIR) does not exist; create it or set INSTALL_DIR"; exit 1; }
	install -m 0755 $(BINARY) "$(INSTALL_DIR)/$(BINARY)"
	@echo "installed $(INSTALL_DIR)/$(BINARY)"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...
