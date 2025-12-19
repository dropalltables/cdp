package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var debugMode = os.Getenv("CDP_DEBUG") != ""

func trace(fn string) {
	if debugMode {
		// Get caller info
		_, file, line, _ := runtime.Caller(2)
		fmt.Fprintf(os.Stderr, "[UI_DEBUG] %s (called from %s:%d)\n", fn, file, line)
	}
}

// Colors
var (
	ColorSuccess = lipgloss.Color("#00D68F")
	ColorError   = lipgloss.Color("#FF4949")
	ColorWarning = lipgloss.Color("#FBCA04")
	ColorInfo    = lipgloss.Color("#3291FF")
	ColorDim     = lipgloss.Color("#666666")
	ColorCode    = lipgloss.Color("#888888")
	ColorBorder  = lipgloss.Color("#333333")
)

// Styles for inline rendering
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ColorError)
	WarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	InfoStyle    = lipgloss.NewStyle().Foreground(ColorInfo)
	DimStyle     = lipgloss.NewStyle().Foreground(ColorDim)
	BoldStyle    = lipgloss.NewStyle().Bold(true)
	CodeStyle    = lipgloss.NewStyle().Foreground(ColorCode)
)

// Icons
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconInfo    = "•"
	IconDot     = "•"
	IconArrow   = "→"
)

// Logger instance
var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportTimestamp: false,
})

func init() {
	// Configure logger styles
	styles := log.DefaultStyles()
	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString("•").
		Foreground(ColorInfo)
	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
		SetString("!").
		Foreground(ColorWarning)
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("✗").
		Foreground(ColorError)
	logger.SetStyles(styles)
}

// Output functions using charmbracelet/log

func Print(msg string) {
	trace("Print")
	fmt.Println(msg)
}

func Success(msg string) {
	trace("Success")
	fmt.Println(SuccessStyle.Render(IconSuccess + " " + msg))
}

func Error(msg string) {
	trace("Error")
	logger.Error(msg)
}

func Warning(msg string) {
	trace("Warning")
	logger.Warn(msg)
}

func Info(msg string) {
	trace("Info")
	logger.Info(msg)
}

func Dim(msg string) {
	trace("Dim")
	fmt.Println(DimStyle.Render(msg))
}

func Bold(msg string) {
	trace("Bold")
	fmt.Println(BoldStyle.Render(msg))
}

func Spacer() {
	trace("Spacer")
	fmt.Println()
}

func Divider() {
	width := getTerminalWidth()
	if width > 80 {
		width = 80 // Cap at 80 for readability
	} else if width < 40 {
		width = 40 // Minimum width
	}
	fmt.Println(DimStyle.Render(strings.Repeat("─", width)))
}

// getTerminalWidth returns the terminal width, defaulting to 60 if unavailable
func getTerminalWidth() int {
	// Try to get terminal size from standard library
	if width, _, err := getTerminalSize(); err == nil && width > 0 {
		return width
	}
	return 60 // Default fallback
}

// getTerminalSize returns the terminal width and height
func getTerminalSize() (int, int, error) {
	// This is a simple implementation that works on Unix-like systems
	// For more robust cross-platform support, consider using golang.org/x/term
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	var height, width int
	_, err = fmt.Sscanf(string(out), "%d %d", &height, &width)
	return width, height, err
}

func Code(msg string) {
	fmt.Println(CodeStyle.Render(msg))
}

func Section(title string) {
	fmt.Println()
	fmt.Println(BoldStyle.Render(title))
	fmt.Println()
}

func KeyValue(key, value string) {
	fmt.Printf("%s %s\n", DimStyle.Render(key+":"), value)
}

func List(items []string) {
	for _, item := range items {
		fmt.Println(DimStyle.Render("  " + IconDot + " " + item))
	}
}

// Table renders a simple table
func Table(headers []string, rows [][]string) {
	if len(rows) == 0 {
		Dim("No data to display")
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Build header row
	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
		}
		headerLine += BoldStyle.Render(fmt.Sprintf("%-*s", widths[i], h))
	}
	fmt.Println(headerLine)

	// Build separator
	sepLine := ""
	totalWidth := 0
	for i, w := range widths {
		totalWidth += w
		if i > 0 {
			totalWidth += 2
		}
	}
	sepLine = strings.Repeat("─", totalWidth)
	fmt.Println(DimStyle.Render(sepLine))

	// Print rows
	for _, row := range rows {
		rowLine := ""
		for i, cell := range row {
			if i > 0 {
				rowLine += "  "
			}
			if i < len(widths) {
				rowLine += fmt.Sprintf("%-*s", widths[i], cell)
			}
		}
		fmt.Println(rowLine)
	}
}

// Prompt functions using huh

func Input(prompt, placeholder string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(prompt).
		Placeholder(placeholder).
		Value(&value).
		Run()
	return value, err
}

func InputWithDefault(prompt, defaultValue string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(prompt).
		Placeholder(defaultValue).
		Value(&value).
		Run()
	if err != nil {
		return "", err
	}
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func Password(prompt string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(prompt).
		EchoMode(huh.EchoModePassword).
		Value(&value).
		Run()
	return value, err
}

func Confirm(prompt string) (bool, error) {
	var value bool
	err := huh.NewConfirm().
		Title(prompt).
		Affirmative("Yes").
		Negative("No").
		Value(&value).
		Run()
	return value, err
}

func Select(prompt string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	var value string
	opts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, opt)
	}

	err := huh.NewSelect[string]().
		Title(prompt).
		Options(opts...).
		Value(&value).
		Run()
	return value, err
}

func SelectWithKeys(prompt string, options map[string]string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	var value string
	opts := make([]huh.Option[string], 0, len(options))
	for key, display := range options {
		opts = append(opts, huh.NewOption(display, key))
	}

	err := huh.NewSelect[string]().
		Title(prompt).
		Options(opts...).
		Value(&value).
		Run()
	return value, err
}

func MultiSelect(prompt string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided")
	}

	var values []string
	opts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, opt)
	}

	err := huh.NewMultiSelect[string]().
		Title(prompt).
		Options(opts...).
		Value(&values).
		Run()
	return values, err
}

func Form(groups ...*huh.Group) error {
	return huh.NewForm(groups...).Run()
}

func ConfirmAction(action, resource string) (bool, error) {
	Warning(fmt.Sprintf("This will %s: %s", action, resource))
	Spacer()
	return Confirm(fmt.Sprintf("Are you sure you want to %s?", action))
}

// LogStream for real-time log viewing
type LogStream struct {
	writer io.Writer
}

func NewLogStream() *LogStream {
	return &LogStream{writer: os.Stdout}
}

func (l *LogStream) Write(msg string) {
	fmt.Fprintln(l.writer, DimStyle.Render(msg))
}

func (l *LogStream) WriteRaw(msg string) {
	fmt.Fprint(l.writer, msg)
}

// CmdOutput is a writer that styles command output as dimmed streamed logs
type CmdOutput struct{}

func NewCmdOutput() *CmdOutput {
	return &CmdOutput{}
}

func (c *CmdOutput) Write(p []byte) (n int, err error) {
	if debugMode {
		lines := strings.Split(string(p), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Println(DimStyle.Render("  " + line))
			}
		}
	}
	return len(p), nil
}

// Status line
type Status struct {
	message string
}

func NewStatus(message string) *Status {
	return &Status{message: message}
}

func (s *Status) Update(message string) {
	s.message = message
	fmt.Printf("\r%s", DimStyle.Render(s.message))
}

func (s *Status) Done() {
	fmt.Println()
}

// Helper functions

func NextSteps(steps []string) {
	trace("NextSteps")
	Dim("Next steps:")
	for _, step := range steps {
		fmt.Println(DimStyle.Render("  " + IconArrow + " " + step))
	}
}

func ErrorWithSuggestion(err error, suggestion string) {
	Error(err.Error())
	if suggestion != "" {
		Spacer()
		Dim("Try: " + suggestion)
	}
}

func StepProgress(current, total int, stepName string) {
	progress := DimStyle.Render(fmt.Sprintf("[%d/%d]", current, total))
	fmt.Printf("%s %s\n", progress, stepName)
}
