.PHONY: all build clean install cross-build

BINARY      = autoxmate
VERSION     = 0.1.0
BUILD_DIR   = dist

all: build

build:
	GOFLAGS="" GONOSUMCHECK="*" GONOSUMDB="*" go build -o $(BINARY) -ldflags="-s -w" .

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)

cross-build:
	mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64   -ldflags="-s -w" .
	GOOS=linux   GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-linux-arm64   -ldflags="-s -w" .
	GOOS=darwin  GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  -ldflags="-s -w" .
	GOOS=darwin  GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  -ldflags="-s -w" .
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe -ldflags="-s -w" .
	cd $(BUILD_DIR) && sha256sum * > checksums.txt

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed to /usr/local/bin/$(BINARY)"

uninstall:
	rm -f /usr/local/bin/$(BINARY)
	@echo "Removed /usr/local/bin/$(BINARY)"

run: build
	./$(BINARY)

test:
	go test ./... -v

lint:
	go vet ./...
