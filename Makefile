.PHONY: build install clean run templ admin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=tribble-cli
ADMIN_BINARY=tribble-admin
CLI_PATH=./cmd/cli
ADMIN_PATH=./cmd/admin

# Generate templ templates
templ:
	templ generate ./internal/web/templates/

# Build the CLI
build: templ
	$(GOBUILD) -o $(BINARY_NAME) $(CLI_PATH)

# Build the admin CLI
admin: templ
	$(GOBUILD) -o $(ADMIN_BINARY) $(ADMIN_PATH)

# Build all binaries
all: templ
	$(GOBUILD) -o $(BINARY_NAME) $(CLI_PATH)
	$(GOBUILD) -o $(ADMIN_BINARY) $(ADMIN_PATH)

install:
	$(GOBUILD) -o $(GOPATH)/bin/$(BINARY_NAME) $(CLI_PATH)
	$(GOBUILD) -o $(GOPATH)/bin/$(ADMIN_BINARY) $(ADMIN_PATH)

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME) $(ADMIN_BINARY)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

.DEFAULT_GOAL := build