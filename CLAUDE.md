# mifind - Claude Code Context

This file provides context for Claude Code to work effectively on this codebase.

---

## Project Structure

```
mifind/
├── cmd/                    # Executable entry points
│   ├── mifind/            # Main API server
│   ├── mifind-mcp/        # MCP server for AI agents
│   └── filesystem-api/    # Standalone filesystem search service
├── internal/              # Private Go packages
│   ├── api/               # HTTP handlers for mifind API
│   ├── provider/          # Provider interface and manager
│   ├── provider/          # Provider implementations (mock, etc.)
│   ├── search/            # Search, ranking, filters
│   └── types/             # Core entity type system
├── pkg/                   # Public packages (can be imported externally)
│   └── provider/          # Provider implementations (filesystem, immich)
├── config/                # Configuration files
│   └── examples/          # Example configs (templates)
├── docs/                  # Documentation
└── CLAUDE.md             # This file
```

---

## Architecture Rules

### Provider Pattern

**Entity ID Format:**
- Services return *raw* IDs only (e.g., just the file ID)
- Provider code adds the prefix: `providerType:instanceID:entityID`
- This keeps services independent and reusable

Example:
- `filesystem-api` returns: `{id: "abc123"}`
- Provider translates to: `filesystem:myfs:abc123`

**Provider Interface:**
- All providers implement `internal/provider.Provider`
- Register via `providerRegistry.Register()`
- Initialize with config map containing `instance_id`

### Service Independence

Services should be self-contained APIs that:
- Work independently of mifind
- Have their own configs in `config/examples/`
- Follow the pattern: look in `config/` first, then fallback locations

### Config Organization

- `config/examples/*.yaml` - Template configs (committed)
- `config/*.yaml` - Local configs (gitignored)
- Both `mifind` and `filesystem-api` look in `config/` dir first

When adding new config:
1. Add example to `config/examples/`
2. Update `README.md` with setup instructions
3. Keep example complete (don't trim useful exclusions/options)

---

## Code Patterns

### HTTP Handlers

Follow `internal/api/handlers.go` pattern:
- Use `ServiceInterface` for testability
- Consistent error responses: `{"error": "message"}`
- Use `mux.Vars()` for path parameters
- Middleware for auth, logging, CORS

### Configuration

Use Viper with this pattern:
```go
viper.SetConfigName("servicename")
viper.SetConfigType("yaml")
viper.AddConfigPath("config")        // Check first
viper.AddConfigPath(".")              // Fallback
viper.AddConfigPath("/etc/mifind")   // System
viper.AddConfigPath("$HOME/.mifind") // User
```

### Entity IDs

When creating entity IDs:
- Services return raw IDs
- Provider wraps with full format
- Use `provider.BuildEntityID()` or `provider.NewEntityID()`

---

## Common Corrections

### Things to Watch For

1. **Over-engineering**: Keep solutions minimal. Don't add features "just in case."
2. **Verbose configs**: Example configs should be practical and complete.
3. **Tight coupling**: Services should be independently useful.
4. **GPT-style docs**: Keep documentation concise. Examples > explanations.

### Anti-patterns to Avoid

- Don't add provider prefix in service responses
- Don't make services depend on mifind internals
- Don't create overly abstract interfaces prematurely
- Don't trim useful example configs to be "brief"

---

## Documentation

### Keep in Sync

When changing the codebase:
1. **API changes** → Update `docs/API.md`
2. **New services** → Add example config to `config/examples/`
3. **New providers** → Update README with setup instructions
4. **New types** → Update type registry if core type

### Documentation Style

- Concise, not verbose
- Examples over explanations
- No marketing language or excessive enthusiasm
- Assume technical audience

---

## Testing Patterns

### Unit Tests

- Use `internal/filesystem/test/` as a pattern
- Mock services using interfaces
- Test utilities in `testutil.go`

### Integration Tests

- Tag with `integration` build tag
- Require external services (Meilisearch, etc.)
- Use `t.Skip()` in short mode

---

## Git Workflow

### Commit Messages

Follow existing pattern:
```
Brief summary (50 chars or less)

- Detail 1
- Detail 2

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

### Before Committing

1. Run `go build ./...` to verify
2. Run tests: `go test ./...`
3. Check `git status` for unexpected files
4. Use `git add` selectively (no `git add .` for everything)

---

## Development Workflow

### Adding a New Provider

1. Create provider in `pkg/provider/<name>/`
2. Implement `internal/provider.Provider` interface
3. Add config example to `config/examples/mifind.yaml`
4. Register in `cmd/mifind/main.go`
5. Update `docs/API.md` if needed

### Adding a New Service

1. Create `cmd/<service>/main.go`
2. Follow existing patterns (Viper, Zerolog, gorilla/mux)
3. Add config to `config/examples/<service>.yaml`
4. Update `README.md` with setup instructions
5. Create standalone docs if applicable
