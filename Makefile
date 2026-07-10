.PHONY: build server admin all templ run test clean

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

SERVER_BINARY=wally
ADMIN_BINARY=tribble-admin
ADMIN_PATH=./cmd/admin

# Generate templ templates
templ:
	templ generate ./internal/web/templates/

# Build the web server (root package)
server: templ
	$(GOBUILD) -o $(SERVER_BINARY) .

# Build the admin CLI (user bootstrap)
admin:
	$(GOBUILD) -o $(ADMIN_BINARY) $(ADMIN_PATH)

# Build everything
all: server admin

build: server

# Build and run the server
run: server
	./$(SERVER_BINARY)

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(SERVER_BINARY) $(ADMIN_BINARY)

.DEFAULT_GOAL := server
