# cdp
> [!CAUTION]
> This project is in alpha, if things go wrong, please do not blame me!

A CLI tool for deploying applications to [Coolify](https://coolify.io) with a single command.

## Installation

### From source

```bash
git clone https://github.com/dropalltables/cdp.git
cd cdp
go build -o cdp .
sudo mv cdp /usr/local/bin/
```

## Quick Start

```bash
# Authenticate with Coolify
cdp login

# Deploy (run in your project directory)
cdp
```

## Usage

### Commands

| Command | Description |
|---------|-------------|
| `cdp` | Deploy to preview environment |
| `cdp --prod` | Deploy to production environment |
| `cdp login` | Configure Coolify, GitHub, and Docker credentials |
| `cdp logout` | Clear stored credentials |
| `cdp whoami` | Show current configuration |
| `cdp health` | Check connectivity to all services |
| `cdp ls` | List deployments for current project |
| `cdp logs` | View deployment logs |
| `cdp link` | Link to existing Coolify application |
| `cdp env ls` | List environment variables |
| `cdp env add KEY=value` | Add environment variable |
| `cdp env rm KEY` | Remove environment variable |
| `cdp env pull` | Download env vars to .env file |
| `cdp env push` | Upload .env file to Coolify |

### Deployment Methods

**Git-based** (recommended for most projects):
- Automatically creates and manages a GitHub repository
- Pushes code and triggers Coolify deployment
- Requires GitHub token with `repo` scope

**Docker-based**:
- Builds Docker image locally
- Pushes to container registry (ghcr.io, Docker Hub, etc.)
- Requires Docker installed and registry credentials
- Registry must be configured on Coolify server

### Framework Detection

Automatically detects and configures:

- Next.js
- Nuxt
- Astro
- SvelteKit
- Vite / React
- Hugo
- Go
- Python
- Node.js
- Static sites

## Configuration

### Global config

Stored at `~/.config/cdp/config.json`:

```json
{
  "coolify_url": "https://coolify.example.com",
  "coolify_token": "...",
  "github_token": "...",
  "docker_registry": {
    "url": "ghcr.io",
    "username": "...",
    "password": "..."
  }
}
```

### Project config

Created automatically as `cdp.json` in your project directory. Add to `.gitignore`.

## Requirements

- Go 1.21+ (for building from source)
- Git
- Docker (optional, for Docker-based deployments)
- Coolify instance with API access

## License

AGPL 3.0
