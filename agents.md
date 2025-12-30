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
- `ls.go` - List projects/applications
- `logs.go` - View deployment logs
- `link.go` - Link to existing Coolify project
- `env.go` - Environment variable management
- `version.go` - Version information
- `health.go` - Health check for Coolify server
- `rollback.go` - Rollback to previous deployment
- `reset.go` - Reset project configuration

### Internal Packages

#### `internal/api/`
Coolify API client implementation:
- `client.go` - HTTP client with authentication
- `applications.go` - Application CRUD operations
- `deployments.go` - Deployment management, log parsing, health checks
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

#### `internal/deploy/`
Deployment orchestration:
- `setup.go` - First-time project setup wizard
- `git.go` - Git-based deployment logic with verbose output support
- `docker.go` - Docker-based deployment logic with verbose output support
- `watcher.go` - Deployment status watcher with log streaming

#### `internal/docker/`
Docker operations:
- `build.go` - Docker image building with framework-specific Dockerfiles
- `push.go` - Push images to registry
- `dockerfile.go` - Generate Dockerfiles dynamically

#### `internal/git/`
Git operations:
- `repo.go` - Git repository management (init, commit, push, log)
- `github.go` - GitHub API client for repository creation

#### `internal/ui/`
User interface:
- `ui.go` - Terminal UI helpers (prompts, colors, output formatting) using survey library
- `task_runner.go` - BubbleTea task runner for async operations with spinner feedback
- `messages.go` - Message types for BubbleTea communication

## Key Patterns

### Code Organization

**Recent Refactoring (Dec 2024):**
The codebase underwent a major refactoring to improve separation of concerns:
- Deployment orchestration logic moved from `cmd/deploy.go` to `internal/deploy/` package
- Setup wizard extracted to `internal/deploy/setup.go` for better reusability
- Git deployment logic isolated in `internal/deploy/git.go`
- Docker deployment logic isolated in `internal/deploy/docker.go`
- File watcher functionality separated into `internal/deploy/watcher.go`
- Task runner with BubbleTea integration added to `internal/ui/task_runner.go` for async operations
- Removed `cmd/whoami.go` - functionality was redundant

This improves maintainability by keeping command files focused on CLI argument parsing and delegating business logic to dedicated packages.

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

When `cdp.json` doesn't exist, the deployment orchestration is handled by `internal/deploy/setup.go`:
1. Framework detection - Analyzes project files to determine framework type
2. Build settings customization - Option to edit install/build/start commands
3. Deployment method selection - Choose between Git or Docker deployment
4. Server selection - Select from available Coolify servers
5. Project selection/creation - Use existing project or create new one
6. Advanced configuration - Port, platform, branch, domain settings
7. Configuration save - Generates and saves `cdp.json`

The setup wizard uses a step-by-step progress indicator (1/5, 2/5, etc.) and the task runner for async operations with spinner feedback.

### Async Task Runner Pattern

For operations that may take time (API calls, network requests), use the task runner pattern:

```go
err := ui.RunTasks([]ui.Task{
    {
        Name:         "task-id",
        ActiveName:   "Loading data...",
        CompleteName: "✓ Loaded data",
        Action: func() error {
            // Your async operation here
            return someAsyncOperation()
        },
    },
})
```

The task runner:
- Displays a spinner while tasks execute
- Shows completion messages when tasks finish
- Handles errors gracefully
- Supports sequential task execution
- Can run in verbose mode (no spinner, immediate output)
- Uses BubbleTea for terminal UI management

### Verbose Mode

The CLI supports a global `--verbose` / `-v` flag that enables detailed output:
- Disables spinners in favor of immediate streaming output
- Git operations (`CommitVerbose`, `PushWithTokenVerbose`) stream stdout/stderr in dimmed format
- Deployment operations show real-time progress
- Access verbose state via `cmd.IsVerbose()` from any command

### Debug Mode

Set `CDP_DEBUG=1` environment variable for internal debugging:
- Shows binary hash on startup
- Enables trace output in UI functions
- Shows detailed deployment watcher status
- Logs API call details and error information

### Deployment Watcher Pattern

For monitoring deployments, use `deploy.WatchDeployment()`:

```go
success := deploy.WatchDeployment(client, appUUID)
if success {
    ui.Success("Deployment complete")
} else {
    ui.Error("Deployment failed")
}
```

The watcher:
- Polls Coolify API for deployment status
- Streams build logs in real-time (dimmed output)
- Handles various Coolify API response formats
- Timeouts after ~4 minutes with configurable polling
- Gracefully handles API errors and missing deployments

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
- `ui.Info()` - Informational messages (cyan dot prefix)
- `ui.Success()` - Success messages (green dash prefix)
- `ui.Warning()` - Warnings (yellow exclamation prefix)
- `ui.Error()` - Error messages (red X prefix)
- `ui.Dim()` - Dimmed/muted output
- `ui.Bold()` - Bold text
- `ui.Select()` / `ui.Input()` - User prompts (GitHub CLI style via survey library)
- `ui.Confirm()` - Yes/no prompts
- `ui.Password()` - Secure password input
- `ui.LogChoice()` - Log auto-selected choices without user interaction
- `ui.RunTasks()` - Execute async operations with spinner feedback
- `ui.Spacer()` - Vertical spacing
- `ui.KeyValue()` - Display key-value pairs (dimmed, indented)
- `ui.List()` - Display bulleted lists
- `ui.Table()` - Display tabular data
- `ui.NextSteps()` - Display next steps to user
- `ui.DimStyle` - Lipgloss style for dimmed log output

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
- Environment variables default to preview scope; use `--prod` flag in `cdp env` commands to target production

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
- Deployment orchestration: `internal/deploy/`
- Docker ops: `internal/docker/`
- Git ops: `internal/git/`
- UI helpers: `internal/ui/`
- Entry point: `main.go`
- Dependencies: `go.mod`
- Build automation: `Makefile`

## Making Changes

1. **Read existing code** - Understand patterns before modifying
2. **Maintain consistency** - Follow existing code style and structure
3. **Update config types** - If adding config fields, update both structs and save/load logic
4. **Handle errors gracefully** - Provide clear error messages
5. **Use UI helpers** - Maintain consistent user experience
6. **Test manually** - Verify commands work end-to-end

## Build System

The project includes a Makefile for build automation:
- `make build` - Build binary to `./bin/cdp`
- `make install` - Install to `$GOPATH/bin`
- `make clean` - Remove build artifacts
- `make test` - Run tests
- `make help` - Show available targets

**Build Workflow:** Always run `make build && make install` together when making changes to ensure the CLI is updated both in the bin directory and in the system path.

## Dependencies

Key external packages:
- `github.com/spf13/cobra` - CLI framework
- `github.com/AlecAivazis/survey/v2` - Interactive prompts (GitHub CLI style)
- `github.com/charmbracelet/bubbletea` - Terminal UI framework for task runner
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/bubbles` - BubbleTea components (spinner)
