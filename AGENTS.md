## Building, testing, and linting

- Build: `make build`
- Test: `make test` (uses `gotestsum`)
- Test a specific function: `go test -run TestMyFunction ./...`
- Lint: `make lint`
- Format: `make fmt`

## Code style

- Follow standard Go conventions.
- Use `gofumpt` for formatting and `golangci-lint` for linting.
- Keep line length under 88 characters.
- Errors should be handled explicitly, not ignored.
- No special import ordering, but `goimports` is recommended.
- No special naming conventions, but follow existing code.
- All new features must have integration tests and good unit test coverage.
- All new features must be discussed with maintainers before implementation.
- All new features must start with a design document.
- The contributor should help to maintain the feature over time.
- Bug fixes and documentation changes are welcome without discussion.
