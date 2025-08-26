# Agent Guidelines for Headscale

## Build/Test Commands
- `make build` - Build project with nix
- `make test` - Run unit tests with gotestsum
- `make test_integration` - Run integration tests in Docker
- `make lint` - Run all linters (Go + protobuf)
- `make fmt` - Format all code (Go, docs, protobuf)
- Run single test: `go test -run TestSpecificFunction ./path/to/package`

## Code Style
- Go 1.23.0, uses gofumpt formatter with 88-char line limit via golines
- Import order: stdlib, third-party, local packages (enforced by gci)
- Use `errors.New()` for static errors, `fmt.Errorf()` for dynamic ones
- Package names follow Go conventions (lowercase, no underscores)
- Use zerolog for structured logging: `log.Info().Msg("message")`
- Constants in ALL_CAPS with underscores, variables in camelCase
- Error handling: return errors, don't panic in library code
- Use context.Context for cancellation/timeouts in functions
- Follow golangci-lint rules (.golangci.yaml) - most linters enabled
- Prefer explicit error handling over blind type assertions
- Use meaningful variable names (avoid single letters except for loops)

## Project Structure
- `hscontrol/` - Main application logic
- `cmd/headscale/` - CLI and main entry point
- `proto/` - Protocol buffer definitions
- `integration/` - Integration test suite