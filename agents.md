# Agents Guide

This document provides guidance for AI agents working with the CDP (Coolify Deploy) codebase.

## Project Overview

CDP is a CLI tool written in Go that provides a Vercel-like deployment experience for Coolify. It enables developers to deploy applications with a single command (`cdp`) without requiring manual git repository setup or complex configuration.

## Architecture

### Command Structure

Commands are organized in the `cmd/` directory:
- `root.go` - Main entry point, handles default deploy behavior
- `deploy.go` - Core deployment logic
- `login.go` - Authentication setup
- `logout.go` - Clear credentials
- `whoami.go` - Display current user info
- `ls.go` - List projects/applications
- `logs.go` - View deployment logs
- `link.go` - Link to existing Coolify project
- `env.go` - Environment variable management
- `version.go` - Version information

### Internal Packages

#### `internal/api/`
Coolify API client implementation:
- `client.go` - HTTP client with authentication
- `applications.go` - Application CRUD operations
- `deployments.go` - Deployment management
- `projects.go` - Project management
- `servers.go` - Server listing
- `types.go` - API request/response types

#### `internal/config/`
Configuration management:
- `global.go` - Global config (credentials, defaults) stored in `~/.cdp/config.json`
- `project.go` - Project config stored in `cdp.json` per project
- `types.go` - Configuration structs

#### `internal/detect/`
Framework detection:
- `detector.go` - Detects framework type and build settings
- `types.go` - Framework information structures

#### `internal/docker/`
Docker operations:
- `build.go` - Docker image building with framework-specific Dockerfiles
- `push.go` - Push images to registry
- `dockerfile.go` - Generate Dockerfiles dynamically

#### `internal/git/`
Git operations:
- `repo.go` - Git repository management (init, commit, push)
- `github.go` - GitHub API client for repository creation

#### `internal/ui/`
User interface:
- `ui.go` - Terminal UI helpers (spinners, prompts, colors)

## Key Patterns

### Configuration Flow

1. **Global Config** (`~/.config/cdp/config.json`):
   - Coolify URL and token
   - GitHub token (optional)
   - Docker registry credentials (optional)

2. **Project Config** (`cdp.json`):
   - Project name and UUIDs
   - Deployment method (docker/git)
   - Framework detection results
   - Build commands and port
   - Single application UUID (supports both production and preview deployments)

### Deployment Methods

1. **Docker-based**:
   - Builds Docker image locally (linux/amd64 for server compatibility)
   - Pushes to configured registry
   - Creates/updates Coolify Docker image application
   - Manual deploys always go to production
   - Requires Docker registry configuration locally AND on Coolify server

2. **Git-based** (recommended):
   - Creates/manages GitHub repository automatically
   - Supports public/private repos
   - Commits and pushes code
   - Creates/updates Coolify application with preview deployments enabled
   - Manual deploys go to production
   - Preview deployments are created automatically by Coolify from GitHub PRs
   - Requires GitHub token with `repo` scope

### First-Time Setup Flow

When `cdp.json` doesn't exist:
1. Detect framework
2. Allow editing build settings
3. Choose deployment method
4. Select server
5. Select or create project
6. Create production environment
7. Save configuration

## Common Tasks

### Adding a New Command

1. Create file in `cmd/` directory
2. Define command with `cobra.Command`
3. Add to `rootCmd` in `init()` function
4. Follow existing patterns for error handling and UI feedback

### Adding Framework Detection

1. Update `internal/detect/detector.go`
2. Add detection logic based on file presence/patterns
3. Return `FrameworkInfo` with build commands

### Modifying API Calls

1. Add methods to `internal/api/client.go` if needed
2. Define request/response types in `internal/api/types.go`
3. Use existing `request()` helper for HTTP calls

### UI Feedback

Always use `internal/ui` helpers:
- `ui.Info()` - Informational messages
- `ui.Success()` - Success messages
- `ui.Warn()` - Warnings
- `ui.NewSpinner()` - Loading indicators
- `ui.Select()` / `ui.Input()` - User prompts

## Important Considerations

### Error Handling

- Always return errors, don't panic
- Use `fmt.Errorf()` with `%w` for error wrapping
- Check login status before API calls using `checkLogin()`

### Configuration Files

- Global config: `~/.config/cdp/config.json` (user-specific, permissions 0600)
- Project config: `cdp.json` (project-specific, should be gitignored)
- Never commit credentials or tokens

### Deployment Architecture

**Single Application Model:**
- Creates one Coolify application per project with preview deployments enabled
- Manual `cdp` deploys always target production (PR number = 0)
- Preview deployments are created automatically by Coolify from GitHub Pull Requests via webhooks
- Environment variables can be scoped to production or preview using `--preview` flag in `cdp env` commands

**Legacy Migration:**
- Old configs with separate preview/production apps are automatically migrated
- Uses the production app as the main app and clears legacy fields

### Testing

- Commands should be testable without actual API calls
- Use dependency injection patterns where possible
- Mock external services (Docker, GitHub, Coolify API)

## File Locations

- Commands: `cmd/*.go`
- API client: `internal/api/`
- Config: `internal/config/`
- Framework detection: `internal/detect/`
- Docker ops: `internal/docker/`
- Git ops: `internal/git/`
- UI helpers: `internal/ui/`
- Entry point: `main.go`
- Dependencies: `go.mod`

## Making Changes

1. **Read existing code** - Understand patterns before modifying
2. **Maintain consistency** - Follow existing code style and structure
3. **Update config types** - If adding config fields, update both structs and save/load logic
4. **Handle errors gracefully** - Provide clear error messages
5. **Use UI helpers** - Maintain consistent user experience
6. **Test manually** - Verify commands work end-to-end

## Dependencies

Key external packages:
- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/huh` - Interactive prompts
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/briandowns/spinner` - Loading indicators
