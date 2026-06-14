# Contributing

## Commit convention

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification:

```
<type>(<scope>): <description>

[optional body]
```

### Types

| Type | When |
|------|------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `test` | Adding or updating tests |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf` | Performance improvement |
| `style` | Formatting, lint fixes (no logic change) |
| `chore` | Build, CI, tooling changes |
| `revert` | Reverting a previous commit |

### Examples

```
feat(tui): add mode selection screen
fix(midi): drain channel on every Update to prevent deadlock
test(sysex): add known-good checksum vectors from cbix repo
docs(readme): add troubleshooting section for device busy
refactor(send): run all config sends in goroutine
chore(lint): add golangci-lint config
```

### Description format

- Use imperative mood: "add" not "added" or "adds"
- First line max 72 characters
- Separate subject from body with a blank line
- Reference issues with `#123` when applicable

### Branch names

Use descriptive kebab-case branch names:
```
feat/auto-detect-device
fix/channel-deadlock
docs/sysex-protocol
test/e2e-real-device
```

## Development workflow

```bash
# Build
go build -o mvave-chocolate-tui .

# Test (all)
go test -count=1 -timeout 30s ./...

# Test (headless E2E, requires device in U mode)
go test -v -run TestHeadless ./...

# Lint
golangci-lint run ./...

# Run
go run .
```

## Before submitting a PR

1. Tests pass: `go test -count=1 ./...`
2. Lint clean: `golangci-lint run ./...`
3. Build succeeds: `go build .`
4. Commit message follows conventional commits
