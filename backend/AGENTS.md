# Backend Guidelines

## Structure

This directory is a Go module. The executable entry point is `cmd/server/main.go`; it is currently a stub. Place Go packages and their tests close to the code they support.

## Development Commands

Run these commands from `backend/`:

- `go run ./cmd/server` runs the server entry point.
- `go build ./...` builds all Go packages.
- `go test ./...` runs all Go tests.

## Development Database

Use the `uptime_postgres` MCP server to inspect the development database. Discover its available tools in the current session, then use `list_schemas`, `list_objects`, or `get_object_details` to inspect the schema and `execute_sql` for read-only queries. For example, `SELECT COUNT(*) FROM public.users` counts users. Check the current schema before relying on table or column names. The MCP server uses restricted access; do not use it to modify data.

## API Documentation

Add swaggo annotations for every API route. When adding, changing, or removing a route or its request or response contract, update the corresponding annotations and regenerate both OpenAPI files from `backend/`:

```sh
go tool swag init -g cmd/server/main.go --parseInternal --output docs --outputTypes json,yaml
```

Then regenerate the ReDoc HTML (requires Node.js and npm):

```sh
npx --yes @redocly/cli@1.34.5 build-docs docs/swagger.json --output docs/redoc.html
```

Include the generated `docs/swagger.json`, `docs/swagger.yaml`, and `docs/redoc.html` in the change.

## Style & Testing

Run `gofmt` on Go files before committing and follow standard Go naming and package conventions. Write tests using Go's `testing` package in adjacent files named `*_test.go`. No coverage threshold is currently configured.
