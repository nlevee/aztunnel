build-all: build-macos build-linux

build-macos:
	env GOOS=darwin GOARCH=arm64 go build -o aztunnel-darwin-arm64 .

build-linux:
	env GOOS=linux GOARCH=amd64 go build -o aztunnel-linux-amd64 .
