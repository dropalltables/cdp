package docker

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/detect"
	"github.com/dropalltables/cdp/internal/ui"
)

// BuildOptions contains options for building a Docker image
type BuildOptions struct {
	Dir       string
	ImageName string
	Tag       string
	Framework *detect.FrameworkInfo
	Platform  string // e.g., "linux/amd64" or "linux/arm64"
}

// Build builds a Docker image for the project
func Build(opts *BuildOptions) error {
	// Generate Dockerfile if one doesn't exist
	dockerfilePath := filepath.Join(opts.Dir, "Dockerfile")
	tempDockerfile := false

	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		content := GenerateDockerfile(opts.Framework)
		tempDockerfilePath := filepath.Join(opts.Dir, "Dockerfile.cdp")
		if err := os.WriteFile(tempDockerfilePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write Dockerfile: %w", err)
		}
		dockerfilePath = tempDockerfilePath
		tempDockerfile = true
	}

	if tempDockerfile {
		defer os.Remove(dockerfilePath)
	}

	platform := opts.Platform
	if platform == "" {
		platform = config.DefaultPlatform
	}

	imageTag := fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag)
	args := []string{"build", "--progress=plain", "--platform", platform, "-t", imageTag, "-f", dockerfilePath, opts.Dir}

	cmd := exec.Command("docker", args...)
	cmd.Dir = opts.Dir
	cmdOut := ui.NewCmdOutput()
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	return nil
}

// GenerateTag generates a unique tag for the image
func GenerateTag(env string) string {
	// Create a short hash based on timestamp
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	shortHash := fmt.Sprintf("%x", hash[:4])
	return fmt.Sprintf("%s-%s", env, shortHash)
}

// IsDockerAvailable checks if Docker is available
func IsDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

// GetImageFullName returns the full image name with registry
func GetImageFullName(registry, username, projectName string) string {
	registry = strings.TrimSuffix(registry, "/")
	return fmt.Sprintf("%s/%s/%s", registry, username, projectName)
}
