package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme
var (
	// Colors - inspired by Vercel's design system
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#00A86B", Dark: "#00D68F"}
	ColorError     = lipgloss.AdaptiveColor{Light: "#E00000", Dark: "#FF4949"}
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#F5A623", Dark: "#FBCA04"}
	ColorInfo      = lipgloss.AdaptiveColor{Light: "#0070F3", Dark: "#3291FF"}
	ColorDim       = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#EAEAEA", Dark: "#333333"}

	// Styles
	baseStyle = lipgloss.NewStyle()

	SuccessStyle = baseStyle.Copy().
			Foreground(ColorSuccess)

	ErrorStyle = baseStyle.Copy().
			Foreground(ColorError)

	WarningStyle = baseStyle.Copy().
			Foreground(ColorWarning)

	InfoStyle = baseStyle.Copy().
			Foreground(ColorInfo)

	DimStyle = baseStyle.Copy().
			Foreground(ColorDim)

	BoldStyle = baseStyle.Copy().
			Foreground(ColorPrimary).
			Bold(true)

	CodeStyle = baseStyle.Copy().
			Foreground(ColorSecondary).
			Background(lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#1A1A1A"}).
			Padding(0, 1)

	// Icons - using Unicode characters for broad compatibility
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "▲"
	IconInfo    = "→"
	IconDot     = "•"
	IconArrow   = "→"
)

// Output functions

func Print(msg string) {
	fmt.Println(msg)
}

func Success(msg string) {
	fmt.Println(SuccessStyle.Render(IconSuccess + " " + msg))
}

func Error(msg string) {
	fmt.Fprintln(os.Stderr, ErrorStyle.Render(IconError+" "+msg))
}

func Warning(msg string) {
	fmt.Println(WarningStyle.Render(IconWarning + " " + msg))
}

func Info(msg string) {
	fmt.Println(InfoStyle.Render(IconInfo + " " + msg))
}

func Dim(msg string) {
	fmt.Println(DimStyle.Render(msg))
}

func Bold(msg string) {
	fmt.Println(BoldStyle.Render(msg))
}

func Spacer() {
	fmt.Println()
}

func Divider() {
	width := 60
	fmt.Println(DimStyle.Render(strings.Repeat("─", width)))
}

func Code(msg string) {
	fmt.Println(CodeStyle.Render(msg))
}

// Section prints a section header
func Section(title string) {
	fmt.Println()
	fmt.Println(BoldStyle.Render(title))
	fmt.Println()
}

// KeyValue prints a key-value pair
func KeyValue(key, value string) {
	keyStyle := DimStyle.Copy().Width(20)
	fmt.Printf("%s %s\n", keyStyle.Render(key+":"), value)
}

// List prints a bulleted list
func List(items []string) {
	for _, item := range items {
		fmt.Println(DimStyle.Render("  " + IconDot + " " + item))
	}
}

// Spinner represents a loading indicator

type spinnerModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
	quitting bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case doneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	if m.done {
		if m.err != nil {
			return ""
		}
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), DimStyle.Render(m.message))
}

type doneMsg struct {
	err error
}

// Spinner wraps a bubbletea spinner
type Spinner struct {
	program *tea.Program
	done    chan error
}

func NewSpinner(message string) *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = InfoStyle

	model := spinnerModel{
		spinner: s,
		message: message,
	}

	return &Spinner{
		program: tea.NewProgram(model),
		done:    make(chan error),
	}
}

func (s *Spinner) Start() {
	go func() {
		if _, err := s.program.Run(); err != nil {
			s.done <- err
		}
	}()
	time.Sleep(50 * time.Millisecond) // Give spinner time to render
}

func (s *Spinner) Stop() {
	s.program.Send(doneMsg{})
	s.program.Quit()
	time.Sleep(10 * time.Millisecond) // Let it clean up
}

func (s *Spinner) StopWithError(err error) {
	s.program.Send(doneMsg{err: err})
	s.program.Quit()
	time.Sleep(10 * time.Millisecond)
}

// WithSpinner runs a function with a spinner
func WithSpinner(message string, fn func() error) error {
	s := NewSpinner(message)
	s.Start()
	err := fn()
	if err != nil {
		s.StopWithError(err)
		Error(fmt.Sprintf("%s failed", message))
		return err
	}
	s.Stop()
	Success(fmt.Sprintf("%s", message))
	return nil
}

// Table creates and displays a formatted table
func Table(headers []string, rows [][]string) {
	if len(rows) == 0 {
		Dim("No data to display")
		return
	}

	columns := make([]table.Column, len(headers))
	for i, h := range headers {
		// Calculate max width for this column
		maxWidth := len(h)
		for _, row := range rows {
			if i < len(row) && len(row[i]) > maxWidth {
				maxWidth = len(row[i])
			}
		}
		columns[i] = table.Column{
			Title: h,
			Width: maxWidth + 2,
		}
	}

	tableRows := make([]table.Row, len(rows))
	for i, row := range rows {
		tableRows[i] = row
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(false),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(ColorPrimary)
	s.Selected = s.Selected.
		Foreground(ColorPrimary).
		Background(lipgloss.Color("")).
		Bold(false)

	t.SetStyles(s)

	fmt.Println(t.View())
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
		Value(&value).
		Placeholder(defaultValue).
		Run()
	if err != nil {
		return "", err
	}
	if value == "" {
		value = defaultValue
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

// MultiSelect allows selecting multiple options
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

// Form represents a multi-step form
func Form(groups ...*huh.Group) error {
	form := huh.NewForm(groups...)
	return form.Run()
}

// ConfirmAction prompts for a potentially destructive action
func ConfirmAction(action, resource string) (bool, error) {
	Warning(fmt.Sprintf("This will %s: %s", action, resource))
	Spacer()
	return Confirm(fmt.Sprintf("Are you sure you want to %s?", action))
}

// LogStream represents a real-time log viewer
type LogStream struct {
	writer io.Writer
}

func NewLogStream() *LogStream {
	return &LogStream{writer: os.Stdout}
}

func (l *LogStream) Write(msg string) {
	// Write log line with subtle styling
	fmt.Fprintln(l.writer, DimStyle.Render(msg))
}

func (l *LogStream) WriteRaw(msg string) {
	fmt.Fprint(l.writer, msg)
}

// Status represents a status line that can be updated
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
	fmt.Println() // New line after status updates
}

// Helper to show next steps
func NextSteps(steps []string) {
	Spacer()
	Dim("Next steps:")
	for _, step := range steps {
		fmt.Println(DimStyle.Render("  " + IconArrow + " " + step))
	}
}

// Helper for error with suggestion
func ErrorWithSuggestion(err error, suggestion string) {
	Error(err.Error())
	if suggestion != "" {
		Spacer()
		Dim("Try: " + suggestion)
	}
}

// StepProgress shows progress through a multi-step process
func StepProgress(current, total int, stepName string) {
	progress := DimStyle.Render(fmt.Sprintf("[%d/%d]", current, total))
	fmt.Printf("%s %s\n", progress, stepName)
}
