BINARY := brionic-remote
WEB := web

.PHONY: all build web go run dev tidy clean test cross bundle

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
	GOOS=linux   GOARCH=arm64 go build -o dist/$(BINARY)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe .

## bundle: assemble the portable "app folder" (copy to a USB drive and run)
bundle: cross
	rm -rf dist/BrionicRemote
	mkdir -p dist/BrionicRemote/bin
	cp dist/$(BINARY)-darwin-arm64 dist/$(BINARY)-darwin-amd64 \
	   dist/$(BINARY)-linux-amd64 dist/$(BINARY)-linux-arm64 \
	   dist/$(BINARY)-windows-amd64.exe dist/BrionicRemote/bin/
	cp packaging/Start-Mac.command packaging/Start-Windows.bat \
	   packaging/Start-Linux.sh packaging/README.txt dist/BrionicRemote/
	chmod +x dist/BrionicRemote/Start-Mac.command dist/BrionicRemote/Start-Linux.sh dist/BrionicRemote/bin/*
	@echo ""
	@echo "Portable app folder ready:  dist/BrionicRemote/"
	@echo "Copy that whole folder to a USB drive and double-click the launcher for your OS."

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist $(WEB)/dist
