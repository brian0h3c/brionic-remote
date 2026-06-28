BINARY := brionic-remote
WEB := web

.PHONY: all build web go run dev tidy clean test cross

all: build

## build: build the web UI and compile the single binary
build: web go

## web: install deps (first run) and build the frontend into web/dist
web:
	rm -rf $(WEB)/dist/assets $(WEB)/dist/index.html
	cd $(WEB) && (test -d node_modules || npm install) && npm run build

## go: compile the Go binary (embeds web/dist)
go:
	go build -o $(BINARY) .

## run: build everything then run
run: build
	./$(BINARY)

## dev: run the Go backend without opening a browser (use `npm run dev` in web/ alongside)
dev:
	go run . --no-browser

## tidy: tidy go modules
tidy:
	go mod tidy

## test: run Go tests
test:
	go test ./...

## cross: build release binaries for mac/linux/windows
cross: web
	GOOS=darwin  GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64 .
	GOOS=linux   GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe .

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist $(WEB)/dist
