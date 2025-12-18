package docker

import (
	"bufio"
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
	Verbose   bool // Show full output instead of hiding it
}

// Push pushes a Docker image to a registry
func Push(opts *PushOptions) error {
	if opts.Username != "" && opts.Password != "" {
		if err := login(opts.Registry, opts.Username, opts.Password, opts.Verbose); err != nil {
			return fmt.Errorf("failed to login to registry: %w", err)
		}
	}

	imageTag := fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag)
	cmd := exec.Command("docker", "push", imageTag)

	// In verbose mode, stream output with dim styling like deployment logs
	if opts.Verbose {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("docker push failed to start: %w", err)
		}

		done := make(chan bool, 2)

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		<-done
		<-done

		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("docker push failed: %w", err)
		}
	} else {
		// In normal mode, capture output (only shown on error via CDP_DEBUG)
		cmdOut := ui.NewCmdOutput()
		cmd.Stdout = cmdOut
		cmd.Stderr = cmdOut

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker push failed: %w", err)
		}
	}

	return nil
}

func login(registry, username, password string, verbose bool) error {
	cmd := exec.Command("docker", "login", registry, "-u", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)

	// In verbose mode, stream output with dim styling like deployment logs
	if verbose {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			return err
		}

		done := make(chan bool, 2)

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		<-done
		<-done

		return cmd.Wait()
	}

	// In normal mode, capture output (only shown on error via CDP_DEBUG)
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
