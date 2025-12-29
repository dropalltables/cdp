package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/charmbracelet/lipgloss"
)

var debugMode = os.Getenv("CDP_DEBUG") != ""

func trace(fn string) {
	if debugMode {
		_, file, line, _ := runtime.Caller(2)
		fmt.Fprintf(os.Stderr, "[UI_DEBUG] %s (called from %s:%d)\n", fn, file, line)
	}
}

// GitHub CLI-style colors (ANSI base 16)
var (
	ColorCyan    = lipgloss.Color("6")
	ColorGreen   = lipgloss.Color("2")
	ColorRed     = lipgloss.Color("1")
	ColorYellow  = lipgloss.Color("3")
	ColorBlue    = lipgloss.Color("4")
	ColorMagenta = lipgloss.Color("5")
	ColorGray    = lipgloss.Color("8")
	ColorWhite   = lipgloss.Color("7")
)

// Styles
var (
	CyanStyle    = lipgloss.NewStyle().Foreground(ColorCyan)
	GreenStyle   = lipgloss.NewStyle().Foreground(ColorGreen)
	RedStyle     = lipgloss.NewStyle().Foreground(ColorRed)
	YellowStyle  = lipgloss.NewStyle().Foreground(ColorYellow)
	BlueStyle    = lipgloss.NewStyle().Foreground(ColorBlue)
	MagentaStyle = lipgloss.NewStyle().Foreground(ColorMagenta)
	GrayStyle    = lipgloss.NewStyle().Foreground(ColorGray)
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	// Semantic aliases
	SuccessStyle = GreenStyle
	ErrorStyle   = RedStyle
	WarningStyle = YellowStyle
	InfoStyle    = CyanStyle
	DimStyle     = GrayStyle
	CodeStyle    = GrayStyle
)

// Icons (ASCII only)
const (
	IconSuccess  = "-"
	IconError    = "X"
	IconWarning  = "!"
	IconQuestion = "?"
	IconDot      = "*"
	IconArrow    = "->"
)

// Survey icons config for GitHub CLI style
var surveyIcons = survey.WithIcons(func(icons *survey.IconSet) {
	icons.Question.Text = "?"
	icons.Question.Format = "cyan+b"
	icons.SelectFocus.Text = ">"
	icons.SelectFocus.Format = "cyan+b"
})

// LogChoice logs a prompt choice without user interaction (for auto-selections)
func LogChoice(question, answer string) {
	prefix := CyanStyle.Bold(true).Render(IconQuestion)
	q := BoldStyle.Render(question)
	a := CyanStyle.Render(answer)
	fmt.Printf("%s %s %s\n", prefix, q, a)
}

// --- Output Functions ---

func Print(msg string) {
	trace("Print")
	fmt.Println(msg)
}

func Success(msg string) {
	trace("Success")
	fmt.Println(GreenStyle.Render(IconSuccess) + " " + msg)
}

func Error(msg string) {
	trace("Error")
	fmt.Println(RedStyle.Render(IconError) + " " + msg)
}

func Warning(msg string) {
	trace("Warning")
	fmt.Println(YellowStyle.Render(IconWarning) + " " + msg)
}

func Info(msg string) {
	trace("Info")
	fmt.Println(CyanStyle.Render(IconDot) + " " + msg)
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
	// Deprecated - use Spacer() instead
	Spacer()
}

func getTerminalWidth() int {
	if width, _, err := getTerminalSize(); err == nil && width > 0 {
		return width
	}
	return 60
}

func getTerminalSize() (int, int, error) {
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
	// Deprecated - sections removed for cleaner output
	// Just add a spacer if needed
	Spacer()
}

func KeyValue(key, value string) {
	// Simple inline display without indentation
	fmt.Printf("%s: %s\n", key, value)
}

func List(items []string) {
	for _, item := range items {
		fmt.Println("  " + IconDot + " " + item)
	}
}

func Table(headers []string, rows [][]string) {
	if len(rows) == 0 {
		Dim("No data to display")
		return
	}

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

	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
		}
		headerLine += fmt.Sprintf("%-*s", widths[i], h)
	}
	fmt.Println(headerLine)

	totalWidth := 0
	for i, w := range widths {
		totalWidth += w
		if i > 0 {
			totalWidth += 2
		}
	}
	fmt.Println(strings.Repeat("-", totalWidth))

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

// --- Prompt Functions (GitHub CLI style using survey) ---

func Confirm(prompt string) (bool, error) {
	var value bool
	err := survey.AskOne(&survey.Confirm{
		Message: prompt,
		Default: false,
	}, &value, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return false, fmt.Errorf("interrupted")
		}
		return false, err
	}

	return value, nil
}

func Input(prompt, placeholder string) (string, error) {
	var value string
	err := survey.AskOne(&survey.Input{
		Message: prompt,
		Default: placeholder,
	}, &value, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return "", fmt.Errorf("interrupted")
		}
		return "", err
	}

	return value, nil
}

func InputWithDefault(prompt, defaultValue string) (string, error) {
	var value string
	err := survey.AskOne(&survey.Input{
		Message: prompt,
		Default: defaultValue,
	}, &value, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return "", fmt.Errorf("interrupted")
		}
		return "", err
	}

	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func Password(prompt string) (string, error) {
	var value string
	err := survey.AskOne(&survey.Password{
		Message: prompt,
	}, &value, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return "", fmt.Errorf("interrupted")
		}
		return "", err
	}

	return value, nil
}

func Select(prompt string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	var value string
	err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: options,
	}, &value, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return "", fmt.Errorf("interrupted")
		}
		return "", err
	}

	return value, nil
}

func SelectWithKeys(prompt string, options map[string]string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	// Build display list and key mapping
	displayOptions := make([]string, 0, len(options))
	keyMap := make(map[string]string)
	for key, display := range options {
		displayOptions = append(displayOptions, display)
		keyMap[display] = key
	}

	var selected string
	err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: displayOptions,
	}, &selected, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return "", fmt.Errorf("interrupted")
		}
		return "", err
	}

	return keyMap[selected], nil
}

func MultiSelect(prompt string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided")
	}

	var values []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: prompt,
		Options: options,
	}, &values, surveyIcons)

	if err != nil {
		if err == terminal.InterruptErr {
			return nil, fmt.Errorf("interrupted")
		}
		return nil, err
	}

	return values, nil
}

func ConfirmAction(action, resource string) (bool, error) {
	Warning(fmt.Sprintf("This will %s: %s", action, resource))
	Spacer()
	return Confirm(fmt.Sprintf("Are you sure you want to %s?", action))
}

// --- Log Stream ---

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

// --- Command Output ---

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

// --- Status Line ---

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

// --- Helper Functions ---

func NextSteps(steps []string) {
	trace("NextSteps")
	fmt.Println("Next steps:")
	for _, step := range steps {
		fmt.Println("  " + IconArrow + " " + step)
	}
}

func ErrorWithSuggestion(err error, suggestion string) {
	Error(err.Error())
	if suggestion != "" {
		Spacer()
		Dim("Try: " + suggestion)
	}
}

// StepProgress is deprecated - use LogChoice instead
func StepProgress(current, total int, stepName string) {
	LogChoice(fmt.Sprintf("Step %d/%d", current, total), stepName)
}
