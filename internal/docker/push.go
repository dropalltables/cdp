package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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
	// Login to registry if credentials provided
	if opts.Username != "" && opts.Password != "" {
		if err := login(opts.Registry, opts.Username, opts.Password); err != nil {
			return fmt.Errorf("failed to login to registry: %w", err)
		}
	}

	// Push the image
	imageTag := fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag)
	cmd := exec.Command("docker", "push", imageTag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	return nil
}

func login(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry, "-u", username, "--password-stdin")
	cmd.Stdin = stringReader(password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// VerifyLogin verifies Docker registry credentials without printing output
func VerifyLogin(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry, "-u", username, "--password-stdin")
	cmd.Stdin = stringReader(password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(output))
	}
	return nil
}

type stringReaderType struct {
	s string
	i int
}

func stringReader(s string) *stringReaderType {
	return &stringReaderType{s: s}
}

func (r *stringReaderType) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}
