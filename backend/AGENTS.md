# Backend Guidelines

## Structure

This directory is a Go module. The executable entry point is `cmd/server/main.go`; it is currently a stub. Place Go packages and their tests close to the code they support.

## Development Commands

Run these commands from `backend/`:

- `go run ./cmd/server` runs the server entry point.
- `go build ./...` builds all Go packages.
- `go test ./...` runs all Go tests.

## Style & Testing

Run `gofmt` on Go files before committing and follow standard Go naming and package conventions. Write tests using Go's `testing` package in adjacent files named `*_test.go`. No coverage threshold is currently configured.
