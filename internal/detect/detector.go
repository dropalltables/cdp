package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Detect attempts to detect the framework in the given directory
func Detect(dir string) (*FrameworkInfo, error) {
	// Check for Dockerfile first (highest priority)
	if fileExists(filepath.Join(dir, "Dockerfile")) {
		return detectDockerfile(dir)
	}

	// Check for Docker Compose
	if fileExists(filepath.Join(dir, "docker-compose.yml")) || fileExists(filepath.Join(dir, "docker-compose.yaml")) {
		return detectDockerCompose(dir)
	}

	// Check for package.json (Node.js projects)
	if fileExists(filepath.Join(dir, "package.json")) {
		return detectNodeProject(dir)
	}

	// Check for Hugo
	if fileExists(filepath.Join(dir, "hugo.toml")) || fileExists(filepath.Join(dir, "config.toml")) {
		if isHugoProject(dir) {
			return detectHugo(dir)
		}
	}

	// Check for Go
	if fileExists(filepath.Join(dir, "go.mod")) {
		return detectGo(dir)
	}

	// Check for Python
	if fileExists(filepath.Join(dir, "requirements.txt")) || fileExists(filepath.Join(dir, "pyproject.toml")) {
		return detectPython(dir)
	}

	// Fallback to static site if index.html exists
	if fileExists(filepath.Join(dir, "index.html")) {
		return detectStatic(dir)
	}

	// Default to nixpacks with no specific framework
	return &FrameworkInfo{
		Name:      "Unknown",
		BuildPack: BuildPackNixpacks,
	}, nil
}

func detectNodeProject(dir string) (*FrameworkInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}

	// Detect Next.js
	if _, ok := allDeps["next"]; ok {
		return &FrameworkInfo{
			Name:           "Next.js",
			BuildPack:      BuildPackNixpacks,
			InstallCommand: "npm install",
			BuildCommand:   "npm run build",
			StartCommand:   "npm start",
			Port:           "3000",
			IsStatic:       false,
		}, nil
	}

	// Detect Astro
	if _, ok := allDeps["astro"]; ok {
		return &FrameworkInfo{
			Name:             "Astro",
			BuildPack:        BuildPackNixpacks,
			InstallCommand:   "npm install",
			BuildCommand:     "npm run build",
			PublishDirectory: "dist",
			Port:             "4321",
			IsStatic:         true,
		}, nil
	}

	// Detect Nuxt
	if _, ok := allDeps["nuxt"]; ok {
		return &FrameworkInfo{
			Name:           "Nuxt",
			BuildPack:      BuildPackNixpacks,
			InstallCommand: "npm install",
			BuildCommand:   "npm run build",
			StartCommand:   "npm start",
			Port:           "3000",
			IsStatic:       false,
		}, nil
	}

	// Detect SvelteKit
	if _, ok := allDeps["@sveltejs/kit"]; ok {
		return &FrameworkInfo{
			Name:           "SvelteKit",
			BuildPack:      BuildPackNixpacks,
			InstallCommand: "npm install",
			BuildCommand:   "npm run build",
			StartCommand:   "npm run preview",
			Port:           "4173",
			IsStatic:       false,
		}, nil
	}

	// Detect Vite (generic)
	if _, ok := allDeps["vite"]; ok {
		return &FrameworkInfo{
			Name:             "Vite",
			BuildPack:        BuildPackNixpacks,
			InstallCommand:   "npm install",
			BuildCommand:     "npm run build",
			PublishDirectory: "dist",
			Port:             "5173",
			IsStatic:         true,
		}, nil
	}

	// Detect React (Create React App)
	if _, ok := allDeps["react-scripts"]; ok {
		return &FrameworkInfo{
			Name:             "Create React App",
			BuildPack:        BuildPackNixpacks,
			InstallCommand:   "npm install",
			BuildCommand:     "npm run build",
			PublishDirectory: "build",
			IsStatic:         true,
		}, nil
	}

	// Generic Node.js
	startCmd := ""
	if _, ok := pkg.Scripts["start"]; ok {
		startCmd = "npm start"
	}
	buildCmd := ""
	if _, ok := pkg.Scripts["build"]; ok {
		buildCmd = "npm run build"
	}

	return &FrameworkInfo{
		Name:           "Node.js",
		BuildPack:      BuildPackNixpacks,
		InstallCommand: "npm install",
		BuildCommand:   buildCmd,
		StartCommand:   startCmd,
		Port:           "3000",
		IsStatic:       false,
	}, nil
}

func detectHugo(dir string) (*FrameworkInfo, error) {
	return &FrameworkInfo{
		Name:             "Hugo",
		BuildPack:        BuildPackNixpacks,
		BuildCommand:     "hugo",
		PublishDirectory: "public",
		IsStatic:         true,
	}, nil
}

func isHugoProject(dir string) bool {
	// Check for typical Hugo directories
	return dirExists(filepath.Join(dir, "content")) ||
		dirExists(filepath.Join(dir, "themes")) ||
		dirExists(filepath.Join(dir, "layouts"))
}

func detectGo(dir string) (*FrameworkInfo, error) {
	return &FrameworkInfo{
		Name:         "Go",
		BuildPack:    BuildPackNixpacks,
		BuildCommand: "go build -o app",
		StartCommand: "./app",
		Port:         "8080",
		IsStatic:     false,
	}, nil
}

func detectPython(dir string) (*FrameworkInfo, error) {
	installCmd := "pip install -r requirements.txt"
	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		installCmd = "pip install ."
	}

	return &FrameworkInfo{
		Name:           "Python",
		BuildPack:      BuildPackNixpacks,
		InstallCommand: installCmd,
		Port:           "8000",
		IsStatic:       false,
	}, nil
}

func detectStatic(dir string) (*FrameworkInfo, error) {
	return &FrameworkInfo{
		Name:             "Static Site",
		BuildPack:        BuildPackStatic,
		PublishDirectory: ".",
		Port:             "80",
		IsStatic:         true,
	}, nil
}

func detectDockerfile(dir string) (*FrameworkInfo, error) {
	return &FrameworkInfo{
		Name:      "Dockerfile",
		BuildPack: BuildPackDockerfile,
		Port:      "3000",
		IsStatic:  false,
	}, nil
}

func detectDockerCompose(dir string) (*FrameworkInfo, error) {
	return &FrameworkInfo{
		Name:      "Docker Compose",
		BuildPack: BuildPackDockerCompose,
		IsStatic:  false,
	}, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
