BINARY_NAME=terraform-provider-passbolt
VERSION=1.5.6
OS=$(shell uname | tr A-Z a-z)
ARCH=amd64
PLUGIN_NAMESPACE=bald1nh0
PLUGIN_PATH=~/.terraform.d/plugins/$(PLUGIN_NAMESPACE)/passbolt/$(VERSION)/$(OS)_$(ARCH)
GO_BIN=$(shell go env GOPATH)/bin
GOLANGCI_LINT_VERSION=v2.11.4
GOLANGCI_LINT=$(GO_BIN)/golangci-lint

.PHONY: all build install lint test docs docs-validate generate setup clean release

all: build

build:
	go build -o $(BINARY_NAME)_v$(VERSION)

install: build
	mkdir -p $(PLUGIN_PATH)
	mv $(BINARY_NAME)_v$(VERSION) $(PLUGIN_PATH)/
	chmod +x $(PLUGIN_PATH)/$(BINARY_NAME)_v$(VERSION)
	@echo "✅ Installed to $(PLUGIN_PATH)"

lint: setup
	@$(GOLANGCI_LINT) run

test:
	TF_ACC=1 go test ./... -v

generate:
	cd tools && go generate ./...

docs: generate docs-validate
	@echo "📚 Docs generated in ./docs"

docs-validate:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate \
		--provider-dir . \
		--provider-name passbolt \
		--allowed-resource-subcategories "Identity,Secrets,Folders & Permissions" \
		--allowed-guide-subcategories "Getting Started,Workflows"

setup:
	@mkdir -p "$(GO_BIN)"
	@if [ ! -x "$(GOLANGCI_LINT)" ] || ! "$(GOLANGCI_LINT)" --version | grep -q "version $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))"; then \
		echo "⏳ Installing golangci-lint $(GOLANGCI_LINT_VERSION) into $(GO_BIN)..."; \
		if command -v curl >/dev/null 2>&1; then \
			curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(GO_BIN)" "$(GOLANGCI_LINT_VERSION)"; \
		elif command -v wget >/dev/null 2>&1; then \
			wget -O- -nv https://golangci-lint.run/install.sh | sh -s -- -b "$(GO_BIN)" "$(GOLANGCI_LINT_VERSION)"; \
		else \
			echo "curl or wget is required to install golangci-lint"; \
			exit 1; \
		fi; \
	fi
	@echo "🔍 Using golangci-lint from $(GOLANGCI_LINT)"
	@$(GOLANGCI_LINT) --version
	@echo "✅ Setup complete."


clean:
	rm -f $(BINARY_NAME)_v$(VERSION)

release:
	goreleaser release --clean
