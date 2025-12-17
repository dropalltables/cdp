package docker

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dropalltables/cdp/internal/ui"
)

// PushOptions contains options for pushing a Docker image
type PushOptions struct {
	ImageName string
	Tag       string
	Registry  string
	Username  string
	Password  string
}

// Push pushes a Docker image to a registry
func Push(opts *PushOptions) error {
	if opts.Username != "" && opts.Password != "" {
		if err := login(opts.Registry, opts.Username, opts.Password); err != nil {
			return fmt.Errorf("failed to login to registry: %w", err)
		}
	}

	imageTag := fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag)
	cmd := exec.Command("docker", "push", imageTag)
	cmdOut := ui.NewCmdOutput()
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}
	return nil
}

func login(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry, "-u", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	cmdOut := ui.NewCmdOutput()
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	return cmd.Run()
}

// VerifyLogin verifies Docker registry credentials without printing output
func VerifyLogin(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry, "-u", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(output))
	}
	return nil
}
