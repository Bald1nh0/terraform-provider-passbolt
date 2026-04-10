BINARY_NAME=terraform-provider-passbolt
VERSION=1.0.5
OS=$(shell uname | tr A-Z a-z)
ARCH=amd64
PLUGIN_NAMESPACE=bald1nh0
PLUGIN_PATH=~/.terraform.d/plugins/$(PLUGIN_NAMESPACE)/passbolt/$(VERSION)/$(OS)_$(ARCH)
LOCAL_BIN=$(CURDIR)/.bin
GOLANGCI_LINT_VERSION=v2.1.6
GOLANGCI_LINT=$(LOCAL_BIN)/golangci-lint

.PHONY: all build install lint test docs generate setup clean release

all: build

build:
	go build -o $(BINARY_NAME)_v$(VERSION)

install: build
	mkdir -p $(PLUGIN_PATH)
	mv $(BINARY_NAME)_v$(VERSION) $(PLUGIN_PATH)/
	chmod +x $(PLUGIN_PATH)/$(BINARY_NAME)_v$(VERSION)
	@echo "✅ Installed to $(PLUGIN_PATH)"

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

test:
	TF_ACC=1 go test ./... -v

generate:
	cd tools && go generate ./...

docs: generate
	@echo "📚 Docs generated in ./docs"

$(GOLANGCI_LINT):
	@echo "⏳ Installing golangci-lint $(GOLANGCI_LINT_VERSION) into $(LOCAL_BIN)..."
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

setup: $(GOLANGCI_LINT)
	@echo "🔍 Using golangci-lint from $(GOLANGCI_LINT)"
	@echo "✅ Setup complete."


clean:
	rm -f $(BINARY_NAME)_v$(VERSION)

release:
	goreleaser release --clean
