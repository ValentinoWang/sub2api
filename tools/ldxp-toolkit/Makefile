.PHONY: test build-darwin build-linux

GO ?= go

test:
	$(GO) test ./...

# Build checks intentionally discard the executable so this module remains source-only.
build-darwin:
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -o /dev/null .

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o /dev/null .
