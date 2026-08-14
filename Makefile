BIN_DIR          := dist/bin
BRIDGE           := $(BIN_DIR)/bridge
ORCHESTRATOR     := $(BIN_DIR)/orchestrator
CLAUDE_APP_SERVER := $(BIN_DIR)/claude-app-server
SERVER           := $(BIN_DIR)/server
WSL_SUPERVISOR   := dist/windows/agent-grid-wsl-supervisor.exe
WEB              := $(BIN_DIR)/web-host
CLI              := $(BIN_DIR)/agent-grid
SERVICES_DIR     := services
WEB_HOST_DIR     := clients/web-host
GOCLIENT_DIR     := clients/goclient
CLI_DIR          := clients/cli
# golangci-lint is pinned as a tool dependency of the services module. The
# independent client modules cannot resolve it through `go tool` from their own
# module, so the pinned binary is built once and run inside each of them. This
# keeps one pinned linter version for the whole repository.
GOLANGCI         := $(CURDIR)/$(BIN_DIR)/golangci-lint
INSTALL_DIR    := $(HOME)/.local/bin
LIBEXEC_DIR    := $(HOME)/.local/lib/agent-grid
SYSTEMD_USER_DIR := $(HOME)/.config/systemd/user
AGENT_GRID_STATE_DIR := $(if $(XDG_STATE_HOME),$(XDG_STATE_HOME),$(HOME)/.local/state)/agent-grid
CODEX_MANAGED_BIN := $(HOME)/.codex/packages/standalone/current/codex

# install-systemd renames the production binaries to their server/web roles so
# the systemd unit file ExecStart= lines and `journalctl --user -u
# agent-grid-…` namespacing read in service vocabulary rather than the
# generic binary name. The binaries themselves are unchanged.
SERVER_BIN  := agent-grid-server
WEB_BIN     := agent-grid-web

CODEX_SCHEMA_DIR := $(SERVICES_DIR)/platform/agent/codexschema
CODEX_SCHEMA_TMP := /tmp/codex-schema-gen

.PHONY: build build-orchestrator build-claude-app-server build-server build-wsl-supervisor-windows build-web build-cli build-all \
        build-web-frontend \
        run-dev install install-systemd install-codex-remote-control-systemd install-web install-server install-cli update-web update-server clean test test-race vet lint \
        verify-bridge-deps \
        codex-schema-update codex-schema-check generate-sdks protocol-compat protocol-simserver \
        verify-save verify-pre-push verify-pr verify-nightly verify-compatibility

# build builds the main backend (cmd/server) — the merged daemon + HTTP/WS
# gateway — plus the in-container helper binary.
build: build-server
	mkdir -p $(BIN_DIR)
	cd $(SERVICES_DIR) && go build -o ../$(BRIDGE) ./cmd/bridge

build-orchestrator:
	mkdir -p $(BIN_DIR)
	cd $(SERVICES_DIR) && go build -o ../$(ORCHESTRATOR) ./cmd/orchestrator

build-claude-app-server:
	mkdir -p $(BIN_DIR)
	cd $(SERVICES_DIR) && go build -o ../$(CLAUDE_APP_SERVER) ./cmd/claude-app-server

# build-server builds the headless backend (cmd/server): the merged daemon +
# REST/WebSocket gateway that browser/CLI clients connect to. It serves no UI.
build-server:
	mkdir -p $(BIN_DIR)
	cd $(SERVICES_DIR) && go build -o ../$(SERVER) ./cmd/server

# Build the Windows SCM supervisor from any Go host. Copy the resulting binary
# to a Windows-local, administrator-controlled directory before registration.
build-wsl-supervisor-windows:
	mkdir -p dist/windows
	cd $(SERVICES_DIR) && GOOS=windows GOARCH=amd64 go build -o ../$(WSL_SUPERVISOR) ./cmd/agent-grid-wsl-supervisor

WEB_DIR             := clients/ui
WEB_DIST            := $(WEB_DIR)/dist/index.html
WEB_HOST_DIST       := $(WEB_HOST_DIR)/internal/webhost/dist/index.html
WEB_NPM_STAMP       := $(WEB_DIR)/node_modules/.install-stamp
# -prune skips the descent into node_modules / dist entirely (a post-filter
# would still walk those subtrees — ~50k files for a React app — on every
# make invocation, since $(shell …) evaluates at parse time).
WEB_SRC_FILES       := $(shell find $(WEB_DIR) \
                         \( -path '$(WEB_DIR)/node_modules' -o -path '$(WEB_DIR)/dist' \) -prune \
                         -o -type f -print 2>/dev/null)

# Stamp file owned by us — directory mtime is touched by IDE / npm internals
# and can't be trusted as a "last install" signal.
$(WEB_NPM_STAMP): $(WEB_DIR)/package-lock.json
	cd $(WEB_DIR) && npm ci
	@touch $@

$(WEB_DIST): $(WEB_NPM_STAMP) $(WEB_SRC_FILES)
	cd $(WEB_DIR) && npm run build

# Sync the Vite build output into the embed dir consumed by clients/web-host.
# .gitkeep is the only tracked file
# there; everything else is derived from clients/ui.
$(WEB_HOST_DIST): $(WEB_DIST)
	find $(WEB_HOST_DIR)/internal/webhost/dist -mindepth 1 ! -name .gitkeep -delete
	cp -r $(WEB_DIR)/dist/. $(WEB_HOST_DIR)/internal/webhost/dist/

# build-web-frontend: alias for $(WEB_HOST_DIST). Stays in the .PHONY list as a
# discoverable target name; the real work lives in the file-target rules above.
build-web-frontend: $(WEB_HOST_DIST)

# build-web builds the independently versioned Web client host. It loads the
# shared client configuration and serves server-qualified same-origin routes.
build-web: $(WEB_HOST_DIST)
	node clients/config/generate-github-oauth.mjs --check
	mkdir -p $(BIN_DIR)
	cd $(WEB_HOST_DIR) && GOWORK=off go build -o ../../$(WEB) ./cmd/web-host

# build-cli builds the terminal client. It is an independent module with no
# dependency edge to services (adr-20260814-cli-independent-go-module), so it is
# built from its own directory with the workspace disabled.
build-cli:
	mkdir -p $(BIN_DIR)
	cd $(CLI_DIR) && GOWORK=off go build -o ../../$(CLI) ./cmd/agent-grid

build-all: build build-orchestrator build-claude-app-server build-server build-web build-cli

# run-dev builds and launches the backend + web-client together for local dev.
run-dev: build-server build-web
	./scripts/run-dev.sh

install: build
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR)
	install -m 755 $(SERVER) $(INSTALL_DIR)/$(notdir $(SERVER))
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(notdir $(BRIDGE))

# install-cli installs the terminal client under its canonical name,
# agent-grid, which is the module path's command name and the built artifact's
# name (adr-20260814-cli-canonical-binary-name). No short alias is installed:
# an installer that claims a two-letter name shadows whatever already owns it
# on the operator's PATH, and an alias the operator adds is theirs to remove.
install-cli: build-cli
	install -d $(INSTALL_DIR)
	install -m 755 $(CLI) $(INSTALL_DIR)/$(notdir $(CLI))
	@echo
	@echo "Installed $(INSTALL_DIR)/$(notdir $(CLI))."
	@echo "For a shorter name, alias it yourself: alias ag=agent-grid"

# install-systemd composes install-server (server + bridge binaries) and
# install-web (web binary) into the user-scope locations consumed by
# deploy/systemd/agent-grid-{server,web}.service, then copies those unit
# files into ~/.config/systemd/user/. Binaries are renamed to their service
# role on disk (agent-grid-server / agent-grid-web) so unit-file
# ExecStart= lines and journald output stay in server/web vocabulary.
#
# After `make install-systemd`, run `systemctl --user daemon-reload && \
# systemctl --user enable --now agent-grid-web.service` to start the stack;
# see docs/note/note-20260624-user-systemd.md for the full procedure.
install-systemd: install-server install-web
	install -d $(SYSTEMD_USER_DIR)
	install -d -m 700 $(AGENT_GRID_STATE_DIR)
	@test -s $(AGENT_GRID_STATE_DIR)/server.token || { \
		command -v openssl >/dev/null || { echo "openssl is required to provision server.token"; exit 1; }; \
		umask 077; openssl rand -hex 24 > $(AGENT_GRID_STATE_DIR)/server.token; \
		echo "Provisioned $(AGENT_GRID_STATE_DIR)/server.token"; \
	}
	chmod 600 $(AGENT_GRID_STATE_DIR)/server.token
	install -m 644 deploy/systemd/agent-grid-server.service  $(SYSTEMD_USER_DIR)/
	install -m 644 deploy/systemd/agent-grid-web.service     $(SYSTEMD_USER_DIR)/
	systemctl --user daemon-reload
	@echo
	@echo "Installed. Next:"
	@echo "  systemctl --user enable --now agent-grid-web.service"
	@echo "  loginctl enable-linger $$USER   # boot-time autostart"
	@echo "See docs/note/note-20260624-user-systemd.md for the full guide."

# Install and start the host-scoped Codex Remote Control daemon. This is kept
# separate from install-systemd because Remote Control is optional and its
# daemon lifecycle requires the official managed standalone Codex install.
# Device pairing is deliberately manual and must not run at every boot.
install-codex-remote-control-systemd:
	@test -x "$(CODEX_MANAGED_BIN)" || { \
		echo "Managed standalone Codex not found: $(CODEX_MANAGED_BIN)"; \
		echo "Install Codex with the official installer, then retry."; \
		exit 1; \
	}
	install -d $(SYSTEMD_USER_DIR)
	install -m 644 deploy/systemd/codex-remote-control.service $(SYSTEMD_USER_DIR)/
	systemctl --user daemon-reload
	systemctl --user enable --now codex-remote-control.service
	@echo
	@echo "Codex Remote Control is enabled for this host."
	@echo "Pair a new device with: codex remote-control pair"
	@echo "Pairing is not required again after normal daemon or host restarts."
	@echo "For boot without login: loginctl enable-linger $$USER"

# install-web builds and installs the web binary only (no unit files, no server).
# Use after front-end-only changes when the server binary is already up to date.
install-web: build-web
	install -d $(INSTALL_DIR)
	install -m 755 $(WEB) $(INSTALL_DIR)/$(WEB_BIN)

# install-server builds and installs the server binary + bridge only
# (no unit files, no web binary). Use after backend-only changes when the web
# binary is already up to date.
install-server: build
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR)
	install -m 755 $(SERVER) $(INSTALL_DIR)/$(SERVER_BIN)
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(notdir $(BRIDGE))

# update-web installs the web binary and restarts the running service.
# daemon-reload is cheap and idempotent — covers the case where the unit file
# or a drop-in changed since the last start.
update-web: install-web
	systemctl --user daemon-reload
	systemctl --user restart agent-grid-web.service

# update-server installs the server binary + bridge and restarts the
# server unit. agent-grid-web is BindsTo= server, so it cascades down with
# server's stop and back up after server's start — no need to restart it
# separately. WARNING: this drops every attached browser tab and kills every
# in-container session (their docker exec child of server is reaped); the
# daemon restores the session list from disk on the next start, but agent
# processes inside containers do not survive.
update-server: install-server
	systemctl --user daemon-reload
	systemctl --user restart agent-grid-server.service

test:
	cd $(SERVICES_DIR) && GOWORK=off go test ./...
	cd $(WEB_HOST_DIR) && GOWORK=off go test ./...
	cd $(GOCLIENT_DIR) && GOWORK=off go test ./...
	cd $(CLI_DIR) && GOWORK=off go test ./...

# test-race runs the concurrency-sensitive subtrees under the race detector.
# Scoped to platform/termvt (Session actor model + emulator drain) and
# host/runtime (single-dispatcher event loop + ipc fan-out) because the
# rest of the tree adds noise (third-party stubs, large startup paths) we
# haven't validated under -race yet. Add subtrees here as they're audited.
# See docs/note/note-20260624-agent-testing.md for the rollout plan.
test-race:
	cd $(SERVICES_DIR) && GOWORK=off go test -race -count=1 ./platform/termvt/... ./host/runtime/...

vet:
	cd $(SERVICES_DIR) && GOWORK=off go vet ./...
	cd $(WEB_HOST_DIR) && GOWORK=off go vet ./...
	cd $(GOCLIENT_DIR) && GOWORK=off go vet ./...
	cd $(CLI_DIR) && GOWORK=off go vet ./...

# $(GOLANGCI) materializes the pinned linter so an independent client module can
# be linted with the same version as services.
$(GOLANGCI):
	mkdir -p $(BIN_DIR)
	cd $(SERVICES_DIR) && GOWORK=off go build -o $(GOLANGCI) github.com/golangci/golangci-lint/v2/cmd/golangci-lint

lint: $(GOLANGCI)
	cd $(SERVICES_DIR) && GOWORK=off go tool golangci-lint run ./...
	cd $(CLI_DIR) && GOWORK=off $(GOLANGCI) run ./...

# Opt-in fidelity backstop: real-CLI e2e for the fakes.
#
# Configure via AG_E2E_CODEX_BIN / AG_E2E_CLAUDE_BIN / AG_E2E_GROK_BIN /
# AG_E2E_APPSERVER_BIN; each subtest skips when its binary is not set,
# so partial configurations are fine. Every fake in this list is validated
# against the wire form of a real CLI — see:
#
#   - docs/adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md
#     (WebSocket routing, stream/fake)
#   - docs/adr/adr-20260704-cli-fake-validated-by-real-cli-e2e.md
#     (Claude CLI stream-json + hook, Codex stdio; fakeclaude + fakecodex)
test-e2e:
	./scripts/run-go-e2e.sh

# Windows Shell T3 e2e against scripts/run-dev.sh (opt-in).
# Stages: backend fixture → xUnit (incl. RunDev facts) → WinUI layout + launch smoke.
# Requires Windows powershell/dotnet for the WinUI stage (or pass --skip-winui).
# See clients/windows-shell/docs/e2e.md
test-windows-shell-e2e:
	./clients/windows-shell/scripts/e2e.sh --start-run-dev

verify-save:
	./scripts/run-verification-profile.sh save

verify-pre-push:
	./scripts/run-verification-profile.sh pre-push

verify-pr:
	./scripts/run-verification-profile.sh pr

verify-nightly:
	./scripts/run-verification-profile.sh nightly

verify-bridge-deps:
	@echo "Checking that bridge does not import host/state, host/uiproc, or platform/features..."
	@cd $(SERVICES_DIR) && GOWORK=off go list -deps ./cmd/bridge | grep -E 'takezoh/agent-grid/services/(host/(state|uiproc)|platform/features)$$' && echo "FAIL: bridge imports forbidden packages" && exit 1 || echo "OK: bridge deps are clean"

# generate-sdks: quicktype models from protocol/*.schema.json into clients/sdk.
# openapi.yaml is REST annex only (not an input). Pin: clients/sdk package-lock
# + .quicktype-version. Transport under each language is hand-written.
generate-sdks:
	./scripts/generate-sdks.sh

# protocol-compat runs the fail-closed compatibility gate (also CI job).
protocol-compat verify-compatibility:
	./scripts/check-protocol-compatibility.sh

# protocol-simserver: stdlib-only Go replay server for protocol/simulator
# recordings (sim-server-language-choice = Go). Outside root go.work on purpose.
PROTOCOL_SIMSERVER := protocol-simserver
protocol-simserver:
	cd protocol/simulator/server && GOWORK=off go build -o $(CURDIR)/$(PROTOCOL_SIMSERVER) .
# Note: if ~/.cache/go-build is unwritable, run with GOCACHE=/tmp/gocache-agent-grid.

clean:
	rm -f $(SERVER) $(WEB) $(CLI) $(BRIDGE) $(ORCHESTRATOR) $(CLAUDE_APP_SERVER) $(PROTOCOL_SIMSERVER) $(GOLANGCI)

# codex-schema-check — verify committed bundle files match current codex output.
# Comparison is done with sorted keys so JSON object ordering doesn't matter.
# Requires the schema-pinned codex and jq in PATH (install with
# `scripts/install-pinned-codex.sh schema`).
codex-schema-check:
	@echo "Generating codex JSON Schema into $(CODEX_SCHEMA_TMP)..."
	@rm -rf $(CODEX_SCHEMA_TMP)
	codex app-server generate-json-schema --out $(CODEX_SCHEMA_TMP)
	@echo "Diffing committed bundles against generated output (sorted keys)..."
	jq --sort-keys . $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.schemas.json > /tmp/_schema_committed.json
	jq --sort-keys . $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.schemas.json > /tmp/_schema_generated.json
	diff /tmp/_schema_committed.json /tmp/_schema_generated.json
	jq --sort-keys . $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.v2.schemas.json > /tmp/_schemav2_committed.json
	jq --sort-keys . $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.v2.schemas.json > /tmp/_schemav2_generated.json
	diff /tmp/_schemav2_committed.json /tmp/_schemav2_generated.json
	@echo "OK: schema bundles are in sync with the schema pin in test-harness/tool-versions.json"

# codex-schema-update — regenerate pinned schema bundles and Go types.
# Run this when upgrading codex. Requires codex and npx (quicktype) in PATH.
# After running: update the version line in services/platform/agent/codexschema/README.md.
codex-schema-update:
	@echo "Generating codex JSON Schema into $(CODEX_SCHEMA_TMP)..."
	@rm -rf $(CODEX_SCHEMA_TMP)
	codex app-server generate-json-schema --out $(CODEX_SCHEMA_TMP)
	@echo "Copying bundle files..."
	cp $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.schemas.json \
	   $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.schemas.json
	cp $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.v2.schemas.json \
	   $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.v2.schemas.json
	@echo "Regenerating v1 Go types..."
	npx quicktype --lang go --package codexschemav1 --src-lang schema \
	    -o $(CODEX_SCHEMA_DIR)/v1/types.gen.go $(CODEX_SCHEMA_TMP)/v1/*.json
	@echo "Regenerating v2 Go types..."
	npx quicktype --lang go --package codexschemav2 --src-lang schema \
	    -o $(CODEX_SCHEMA_DIR)/v2/types.gen.go $(CODEX_SCHEMA_TMP)/v2/*.json
	@echo "Done. Update the pinned version in $(CODEX_SCHEMA_DIR)/README.md."
