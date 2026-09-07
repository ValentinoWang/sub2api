.PHONY: test build-darwin build-linux package-linux-amd64 clean

GO ?= go
DIST_DIR ?= dist
TOOLKIT_VERSION ?= dev
LINUX_AMD64_DIR := $(DIST_DIR)/linux-amd64
LINUX_AMD64_BINARY := $(LINUX_AMD64_DIR)/ldxp-toolkit
LINUX_AMD64_MANIFEST := $(LINUX_AMD64_DIR)/ldxp-toolkit-release.json

test:
	$(GO) test ./...

build-darwin:
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -o /dev/null .

build-linux:
	@$(MAKE) package-linux-amd64 DIST_DIR=$(DIST_DIR) TOOLKIT_VERSION=$(TOOLKIT_VERSION)

# The release payload is generated, never checked into source. The manifest is
# consumed by the server runtime as an explicit image/release assertion.
package-linux-amd64:
	@toolkit_version="$(TOOLKIT_VERSION)"; \
	case "$$toolkit_version" in \
		""|dev|unpackaged|[!0-9A-Za-z]*|*[!0-9A-Za-z._+-]*) echo "TOOLKIT_VERSION must be an explicit release version" >&2; exit 2;; \
	esac; \
	if [ $${#toolkit_version} -gt 128 ]; then echo "TOOLKIT_VERSION is too long" >&2; exit 2; fi
	@rm -rf "$(LINUX_AMD64_DIR)"
	@mkdir -p "$(LINUX_AMD64_DIR)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w -X main.toolkitVersion=$(TOOLKIT_VERSION)" -o "$(LINUX_AMD64_BINARY)" .
	@test -f "$(LINUX_AMD64_BINARY)" && test -x "$(LINUX_AMD64_BINARY)"
	@digest="$$(shasum -a 256 "$(LINUX_AMD64_BINARY)" | awk '{print $$1}')"; \
	printf '{"schema_version":1,"program":"ldxp-toolkit","version":"%s","os":"linux","arch":"amd64","sha256":"%s"}\\n' "$(TOOLKIT_VERSION)" "$$digest" > "$(LINUX_AMD64_MANIFEST)"
	@chmod 0555 "$(LINUX_AMD64_BINARY)" && chmod 0444 "$(LINUX_AMD64_MANIFEST)"

clean:
	rm -rf "$(DIST_DIR)"
