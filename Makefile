BINARY_NAME=terraform-provider-passbolt
VERSION=1.0.5
OS=$(shell uname | tr A-Z a-z)
ARCH=amd64
PLUGIN_NAMESPACE=bald1nh0
PLUGIN_PATH=~/.terraform.d/plugins/$(PLUGIN_NAMESPACE)/passbolt/$(VERSION)/$(OS)_$(ARCH)

.PHONY: all build install lint test docs generate setup clean release

all: build

build:
	go build -o $(BINARY_NAME)_v$(VERSION)

install: build
	mkdir -p $(PLUGIN_PATH)
	mv $(BINARY_NAME)_v$(VERSION) $(PLUGIN_PATH)/
	chmod +x $(PLUGIN_PATH)/$(BINARY_NAME)_v$(VERSION)
	@echo "✅ Installed to $(PLUGIN_PATH)"

lint:
	golangci-lint run

test:
	TF_ACC=1 go test ./... -v

generate:
	cd tools && go generate ./...

docs: generate
	@echo "📚 Docs generated in ./docs"

setup:
	@echo "🔍 Checking golangci-lint..."
	@if ! [ -x "$$(command -v golangci-lint)" ]; then \
		echo "⏳ Installing golangci-lint v2.0.0..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.0.0; \
	else \
		version=$$(golangci-lint version | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
		required="v2.0.0"; \
		if [ "$$(printf "%s\n%s\n" "$$required" "$$version" | sort -V | head -1)" != "$$required" ]; then \
			echo "❌ golangci-lint $$version is too old. Upgrading..."; \
			go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6; \
		else \
			echo "✅ golangci-lint $$version is up to date."; \
		fi \
	fi

	@echo "✅ Setup complete."


clean:
	rm -f $(BINARY_NAME)_v$(VERSION)

release:
	goreleaser release --clean
