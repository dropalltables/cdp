// Package detect provides framework detection using coolpack for Node.js
// and simple file-based detection for other languages.
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/coollabsio/coolpack/pkg/app"
	"github.com/coollabsio/coolpack/pkg/detector"
	"github.com/coollabsio/coolpack/pkg/generator"
)

// Result represents the output of framework detection
type Result struct {
	Framework string // display name (e.g., "Next.js", "Go", "Python")
	Kind      string // node, go, python, hugo, static, dockerfile, dockercompose, unknown

	// Build configuration
	InstallCommand   string
	BuildCommand     string
	StartCommand     string
	PublishDirectory string
	Port             string
	IsStatic         bool

	// Coolpack fields (populated for Node.js projects)
	Language              string // nodejs, bun
	LanguageVersion       string // e.g., "20"
	PackageManager        string // npm, yarn, pnpm, bun
	PackageManagerVersion string

	// Internal: coolpack plan for Dockerfile generation
	plan *app.Plan
}

// Detect analyzes the directory and returns detection results
func Detect(dir string) (*Result, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// User's Dockerfile takes precedence
	if fileExists(filepath.Join(absDir, "Dockerfile")) {
		return &Result{
			Kind:      "dockerfile",
			Framework: "Dockerfile",
			Port:      "3000",
		}, nil
	}

	// Docker Compose
	if fileExists(filepath.Join(absDir, "docker-compose.yml")) || fileExists(filepath.Join(absDir, "docker-compose.yaml")) {
		return &Result{
			Kind:      "dockercompose",
			Framework: "Docker Compose",
		}, nil
	}

	// Node.js - use coolpack
	if fileExists(filepath.Join(absDir, "package.json")) {
		return detectNode(absDir)
	}

	// Hugo
	if (fileExists(filepath.Join(absDir, "hugo.toml")) || fileExists(filepath.Join(absDir, "config.toml"))) && isHugoProject(absDir) {
		return &Result{
			Kind:             "hugo",
			Framework:        "Hugo",
			BuildCommand:     "hugo --minify",
			PublishDirectory: "public",
			Port:             "80",
			IsStatic:         true,
		}, nil
	}

	// Go
	if fileExists(filepath.Join(absDir, "go.mod")) {
		return &Result{
			Kind:         "go",
			Framework:    "Go",
			BuildCommand: "go build -o app",
			StartCommand: "./app",
			Port:         "8080",
		}, nil
	}

	// Python
	if fileExists(filepath.Join(absDir, "requirements.txt")) || fileExists(filepath.Join(absDir, "pyproject.toml")) {
		installCmd := "pip install -r requirements.txt"
		if fileExists(filepath.Join(absDir, "pyproject.toml")) {
			installCmd = "pip install ."
		}
		return &Result{
			Kind:           "python",
			Framework:      "Python",
			InstallCommand: installCmd,
			Port:           "8000",
		}, nil
	}

	// Static site
	if fileExists(filepath.Join(absDir, "index.html")) {
		return &Result{
			Kind:             "static",
			Framework:        "Static Site",
			PublishDirectory: ".",
			Port:             "80",
			IsStatic:         true,
		}, nil
	}

	return &Result{
		Kind:      "unknown",
		Framework: "Unknown",
		Port:      "3000",
	}, nil
}

// detectNode uses coolpack for Node.js detection
func detectNode(dir string) (*Result, error) {
	d := detector.New(dir)
	plan, err := d.Detect()
	if err != nil {
		return nil, err
	}

	// coolpack returns nil if no Node.js project detected
	if plan == nil {
		return &Result{
			Kind:           "node",
			Framework:      "Node.js",
			InstallCommand: "npm install",
			StartCommand:   "npm start",
			Port:           "3000",
		}, nil
	}

	isStatic := false
	if ot, ok := plan.Metadata["output_type"].(string); ok {
		isStatic = ot == "static"
	}

	port := "3000"
	if isStatic {
		port = "80"
	}

	publishDir := ""
	if d, ok := plan.Metadata["output_dir"].(string); ok {
		publishDir = d
	}

	// Validate start command - coolpack may reference scripts that don't exist
	startCmd := validateStartCommand(dir, plan)

	return &Result{
		Kind:                  "node",
		Framework:             formatFrameworkName(plan.Framework),
		InstallCommand:        plan.InstallCommand,
		BuildCommand:          plan.BuildCommand,
		StartCommand:          startCmd,
		PublishDirectory:      publishDir,
		Port:                  port,
		IsStatic:              isStatic,
		Language:              plan.Language,
		LanguageVersion:       plan.LanguageVersion,
		PackageManager:        plan.PackageManager,
		PackageManagerVersion: plan.PackageManagerVersion,
		plan:                  plan,
	}, nil
}

// validateStartCommand ensures the start command will work
func validateStartCommand(dir string, plan *app.Plan) string {
	cmd := plan.StartCommand
	if cmd == "" {
		return getDefaultStartCommand(dir, plan.Framework)
	}

	// Check if it references an npm script that exists
	scriptName := ""
	if strings.HasPrefix(cmd, "npm run ") {
		scriptName = strings.TrimPrefix(cmd, "npm run ")
	} else if cmd == "npm start" {
		scriptName = "start"
	}

	if scriptName != "" && !hasScript(dir, scriptName) {
		return getDefaultStartCommand(dir, plan.Framework)
	}

	return cmd
}

// hasScript checks if package.json has the given script
func hasScript(dir, name string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	_, ok := pkg.Scripts[name]
	return ok
}

// getDefaultStartCommand returns framework-appropriate start command
func getDefaultStartCommand(dir, framework string) string {
	switch framework {
	case "nextjs":
		return "next start"
	case "nuxt":
		return "node .output/server/index.mjs"
	case "sveltekit":
		return "node build"
	case "remix", "react-router":
		return "remix-serve build"
	case "express", "fastify", "nestjs", "adonisjs":
		// Try to find main file
		if main := getMainFile(dir); main != "" {
			return "node " + main
		}
		return "node index.js"
	default:
		if main := getMainFile(dir); main != "" {
			return "node " + main
		}
		return "node index.js"
	}
}

// getMainFile reads the main field from package.json
func getMainFile(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Main string `json:"main"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Main
}

// GenerateDockerfile generates a Dockerfile for the detected project
func (r *Result) GenerateDockerfile() string {
	// Node.js projects use coolpack's generator
	if r.Kind == "node" && r.plan != nil {
		gen := generator.New(r.plan)
		dockerfile, err := gen.GenerateDockerfile()
		if err == nil {
			return dockerfile
		}
	}

	// Other languages use simple templates
	switch r.Kind {
	case "go":
		return dockerfileGo
	case "python":
		return dockerfilePython
	case "hugo":
		return dockerfileHugo
	case "static":
		return dockerfileStatic
	default:
		return dockerfileGeneric
	}
}

// MarshalPlan returns the coolpack plan as JSON for storage
func (r *Result) MarshalPlan() json.RawMessage {
	if r.plan == nil {
		return nil
	}
	data, _ := json.Marshal(r.plan)
	return data
}

func formatFrameworkName(name string) string {
	names := map[string]string{
		"nextjs":         "Next.js",
		"nuxt":           "Nuxt",
		"astro":          "Astro",
		"sveltekit":      "SvelteKit",
		"remix":          "Remix",
		"vite":           "Vite",
		"gatsby":         "Gatsby",
		"cra":            "Create React App",
		"angular":        "Angular",
		"eleventy":       "Eleventy",
		"express":        "Express",
		"fastify":        "Fastify",
		"nestjs":         "NestJS",
		"adonisjs":       "AdonisJS",
		"solid-start":    "Solid Start",
		"tanstack-start": "TanStack Start",
		"react-router":   "React Router",
	}
	if formatted, ok := names[name]; ok {
		return formatted
	}
	if name == "" {
		return "Node.js"
	}
	return name
}

func isHugoProject(dir string) bool {
	return dirExists(filepath.Join(dir, "content")) ||
		dirExists(filepath.Join(dir, "themes")) ||
		dirExists(filepath.Join(dir, "layouts"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Dockerfile templates for non-Node.js languages
const dockerfileGo = `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
CMD ["./app"]
`

const dockerfilePython = `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt* pyproject.toml* ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || pip install --no-cache-dir .
COPY . .
EXPOSE 8000
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
`

const dockerfileHugo = `FROM klakegg/hugo:ext-alpine AS builder
WORKDIR /app
COPY . .
RUN hugo --minify

FROM nginx:alpine
COPY --from=builder /app/public /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

const dockerfileStatic = `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

const dockerfileGeneric = `FROM node:20-alpine
WORKDIR /app
COPY . .
RUN npm install 2>/dev/null || true
EXPOSE 3000
CMD ["npm", "start"]
`
