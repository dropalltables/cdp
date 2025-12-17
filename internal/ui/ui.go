package ui

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	titleStyle   = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			Padding(0, 1)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)

// Success prints a success message
func Success(msg string) {
	fmt.Println(successStyle.Render("✓ " + msg))
}

// Error prints an error message
func Error(msg string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("✗ "+msg))
}

// Warn prints a warning message
func Warn(msg string) {
	fmt.Println(warnStyle.Render("! " + msg))
}

// Info prints an info message
func Info(msg string) {
	fmt.Println(infoStyle.Render("→ " + msg))
}

// Dim prints a dimmed message
func Dim(msg string) {
	fmt.Println(dimStyle.Render(msg))
}

// NewSpinner creates a new spinner
func NewSpinner(msg string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + msg
	return s
}

// Prompt functions using charmbracelet/huh

// Input prompts for text input
func Input(label, placeholder string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(label).
		Placeholder(placeholder).
		Value(&value).
		Run()
	return value, err
}

// InputWithDefault prompts for text input with a default value
func InputWithDefault(label, defaultValue string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(label).
		Value(&value).
		Placeholder(defaultValue).
		Run()
	if value == "" {
		value = defaultValue
	}
	return value, err
}

// Password prompts for password input (masked)
func Password(label string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(label).
		EchoMode(huh.EchoModePassword).
		Value(&value).
		Run()
	return value, err
}

// Confirm prompts for yes/no confirmation
func Confirm(label string) (bool, error) {
	var value bool
	err := huh.NewConfirm().
		Title(label).
		Value(&value).
		Run()
	return value, err
}

// Select prompts for selection from options
func Select(label string, options []string) (string, error) {
	var value string
	opts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, opt)
	}
	err := huh.NewSelect[string]().
		Title(label).
		Options(opts...).
		Value(&value).
		Run()
	return value, err
}

// SelectWithKeys prompts for selection with display labels and return keys
func SelectWithKeys(label string, options map[string]string) (string, error) {
	var value string
	opts := make([]huh.Option[string], 0, len(options))
	for key, display := range options {
		opts = append(opts, huh.NewOption(display, key))
	}
	err := huh.NewSelect[string]().
		Title(label).
		Options(opts...).
		Value(&value).
		Run()
	return value, err
}

// waitForEnter waits for user to press Enter or Ctrl+C
func waitForEnter() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	done := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadBytes('\n')
		if err != nil {
			errChan <- err
			return
		}
		done <- true
	}()

	select {
	case <-done:
		return nil
	case <-sigChan:
		return fmt.Errorf("cancelled")
	case err := <-errChan:
		return fmt.Errorf("failed to read input: %w", err)
	}
}

// ProceedOrCancel waits for Enter to proceed or Ctrl+C to cancel
func ProceedOrCancel(message string) error {
	if message == "" {
		message = "Press Enter to proceed, Ctrl+C to cancel"
	}
	fmt.Println()
	fmt.Print(dimStyle.Render(message + ": "))
	if err := waitForEnter(); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// WelcomeScreen displays a styled welcome screen and waits for user to proceed
func WelcomeScreen() error {
	logo := `           .___       
  ____   __| _/_____  
_/ ___\ / __ |\____ \ 
\  \___/ /_/ ||  |_> >
 \___  >____ ||   __/ 
     \/     \/|__|    `

	fmt.Println()
	fmt.Println(titleStyle.Render(logo))
	fmt.Println()
	fmt.Println(helpStyle.Render("Press [ENTER] to continue, [CTRL+C] to exit"))
	fmt.Println()

	return waitForEnter()
}
