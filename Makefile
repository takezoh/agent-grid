BINARY      := roost
BRIDGE      := roost-bridge
SOCKBRIDGE  := sockbridge
NOTIFY_PS1  := notify.ps1
SRC_DIR     := src
INSTALL_DIR    := $(HOME)/.local/bin
LIBEXEC_DIR    := $(HOME)/.local/lib/roost

.PHONY: build build-experimental install clean test vet lint verify-bridge-deps

build:
	cd $(SRC_DIR) && go build -o ../$(BINARY) ./cmd/roost
	cd $(SRC_DIR) && go build -o ../$(BRIDGE) ./cmd/roost-bridge
	cd $(SRC_DIR) && go build -o ../$(SOCKBRIDGE) github.com/takezoh/credproxy/cmd/sockbridge
	cp $(SRC_DIR)/platform/lib/notify/notify.ps1 ./$(NOTIFY_PS1)

build-experimental:
	cd $(SRC_DIR) && go build -tags experimental -o ../$(BINARY) ./cmd/roost

install: build
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(BRIDGE)
	install -m 755 $(SOCKBRIDGE) $(LIBEXEC_DIR)/$(SOCKBRIDGE)
	install -m 644 $(NOTIFY_PS1) $(LIBEXEC_DIR)/$(NOTIFY_PS1)

test:
	cd $(SRC_DIR) && go test ./...

vet:
	cd $(SRC_DIR) && go vet ./...

lint:
	cd $(SRC_DIR) && go tool golangci-lint run ./...

verify-bridge-deps:
	@echo "Checking that roost-bridge does not import client/state, client/uiproc, or platform/features..."
	@cd $(SRC_DIR) && go list -deps ./cmd/roost-bridge | grep -E 'takezoh/agent-roost/(client/(state|uiproc)|platform/features)$$' && echo "FAIL: bridge imports forbidden packages" && exit 1 || echo "OK: bridge deps are clean"

clean:
	rm -f $(BINARY) $(BRIDGE) $(SOCKBRIDGE) $(NOTIFY_PS1)
