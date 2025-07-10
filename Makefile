.PHONY: build install

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=tribble-cli
CLI_PATH=./cmd/cli

build:
	$(GOBUILD) -o $(BINARY_NAME) $(CLI_PATH)

install:
	$(GOBUILD) -o $(GOPATH)/bin/$(BINARY_NAME) $(CLI_PATH)

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

.DEFAULT_GOAL := build 