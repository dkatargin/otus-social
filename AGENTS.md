# AGENTS.md

## Build, Lint, and Test Commands
- **Build:** `cd src && go build -o server .`
- **Run all tests:** `cd src && go test ./tests -v`
- **Run a single test:** `cd src && go test ./tests -run TestName -v`
- **Test with coverage:** `cd src && go test ./tests -cover`
- **Lint:** Use `gofmt -w .` for formatting. Optionally, use `golangci-lint run ./...` if installed.

## Code Style Guidelines
- Use `gofmt` for formatting; keep imports grouped: stdlib, third-party, local (blank lines between).
- Types: Use idiomatic Go types and struct tags for JSON/DB.
- Naming: CamelCase for exported types/functions; lowerCamelCase for locals.
- Error handling: Always check errors, return wrapped errors with context (`fmt.Errorf("...: %w", err)`).
- Comments: Use full sentences; exported items start with the name.
- Use `context.Context` as the first argument for service methods.
- Keep handler logic thin; business logic in services.
- Use idiomatic Go for slices, maps, and concurrency.
- Tests live in `src/tests/`, named `*_test.go`.

No Cursor or Copilot rules are present. Follow Go community best practices throughout.