# Tasks: Update & Execute

Quick reference for common development tasks in this project.

## Prerequisites

- Go 1.24.0+
- (Optional) [sqlc](https://sqlc.dev) for regenerating DB code
- (Optional) [GoReleaser](https://goreleaser.com) for snapshot/release builds

## Build & Run

```bash
# Build binary
go build -o dhriti

# Run interactively
./dhriti

# Run with debug logging
./dhriti -d

# Run in a specific directory
./dhriti -c /path/to/project

# Non-interactive prompt
./dhriti -p "Explain the use of context in Go"

# JSON output, no spinner
./dhriti -p "Explain context in Go" -f json -q

# Print version
./dhriti -v
```

## Test

```bash
# All tests
go test ./...

# Single package
go test ./internal/llm/tools/...

# Verbose
go test -v ./internal/tui/theme/...
```

## Vet / Lint

```bash
go vet ./...
```

## Database (sqlc + migrations)

```bash
# Regenerate Go code from SQL queries
sqlc generate

# Migrations live in:
#   internal/db/migrations/*.sql
# Queries live in:
#   internal/db/sql/*.sql
# Generated output:
#   internal/db/*.sql.go
```

Migrations are embedded (`internal/db/embed.go`) and run automatically on connect.

## Config Schema

```bash
# Regenerate dhriti-schema.json
go run cmd/schema/main.go > dhriti-schema.json
```

See `cmd/schema/README.md` for details.

## Snapshot Build (local binaries)

```bash
./scripts/snapshot
# equivalent to: goreleaser build --clean --snapshot --skip validate
```

## Release

```bash
# Patch bump (x.y.Z -> x.y.(Z+1))
./scripts/release

# Minor bump (x.Y.0 -> x.(Y+1).0)
./scripts/release --minor
```

Tags trigger `.github/workflows/release.yml`.

## Check for Hidden Characters

```bash
./scripts/check_hidden_chars.sh
```

## Install (end user)

```bash
curl -fsSL ... | bash   # see ./install script
# or from source:
go build -o dhriti && ./dhriti
```

## Project Layout (task-relevant)

| Path | Purpose |
| ---- | ------- |
| `cmd/` | CLI entry (Cobra), schema generator |
| `internal/app/` | Core app lifecycle |
| `internal/config/` | Config load/merge |
| `internal/db/` | SQLite, sqlc, migrations |
| `internal/llm/` | Providers, agents, tools, prompts |
| `internal/tui/` | Bubble Tea UI |
| `internal/lsp/` | LSP client |
| `scripts/` | Release, snapshot, hygiene checks |
| `.github/workflows/` | CI build + release |

## Updating Dependencies

```bash
go get -u ./...
go mod tidy
```

## Typical Update Flow

1. Create branch: `git checkout -b feature/...`
2. Edit code / SQL / config schema sources
3. If SQL changed: `sqlc generate`
4. If config struct changed: `go run cmd/schema/main.go > dhriti-schema.json`
5. `go vet ./... && go test ./...`
6. `go build -o dhriti` and smoke-test
7. Commit, push, open PR
