package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Task represents an async operation to run
type Task struct {
	Name         string      // Unique identifier for the task
	ActiveName   string      // Message shown while task is running (e.g., "Loading servers...")
	CompleteName string      // Message shown when task completes (e.g., "✓ Loaded servers")
	Action       func() error // Function to execute
}

// TaskRunnerModel runs sequential tasks with spinner feedback
type TaskRunnerModel struct {
	tasks       []Task
	currentIdx  int
	spinner     spinner.Model
	completed   []string
	err         error
	done        bool
	quitting    bool
}

// NewTaskRunner creates a new task runner model
func NewTaskRunner(tasks []Task) TaskRunnerModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = DimStyle

	return TaskRunnerModel{
		tasks:     tasks,
		spinner:   s,
		completed: []string{},
	}
}

// Init initializes the model
func (m TaskRunnerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.runNextTask(),
	)
}

// runNextTask executes the next task in the queue
func (m TaskRunnerModel) runNextTask() tea.Cmd {
	if m.currentIdx >= len(m.tasks) {
		return func() tea.Msg {
			return allTasksCompleteMsg{}
		}
	}

	task := m.tasks[m.currentIdx]
	return func() tea.Msg {
		err := task.Action()
		return taskCompleteMsg{err: err}
	}
}

// Update handles messages
func (m TaskRunnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			m.done = true
			return m, tea.Quit
		}

	case taskCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}

		// Add completed task to list
		task := m.tasks[m.currentIdx]
		m.completed = append(m.completed, task.CompleteName)
		m.currentIdx++

		// Check if all tasks are done
		if m.currentIdx >= len(m.tasks) {
			m.done = true
			return m, tea.Quit
		}

		// Run next task
		return m, m.runNextTask()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case allTasksCompleteMsg:
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

// View renders the UI
func (m TaskRunnerModel) View() string {
	if m.done {
		return ""
	}

	var buf strings.Builder

	// Show all completed tasks
	for _, completed := range m.completed {
		buf.WriteString(completed + "\n")
	}

	// Show current task with spinner
	if m.currentIdx < len(m.tasks) {
		task := m.tasks[m.currentIdx]
		buf.WriteString(m.spinner.View() + " " + task.ActiveName)
	}

	return buf.String()
}

// RunTasks executes a sequence of tasks with spinner feedback
// Returns an error if any task fails
func RunTasks(tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	p := tea.NewProgram(NewTaskRunner(tasks))
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("task runner failed: %w", err)
	}

	if m, ok := finalModel.(TaskRunnerModel); ok {
		if m.err != nil {
			return m.err
		}
	} else {
		// Type assertion failed - this should never happen but handle it gracefully
		return fmt.Errorf("unexpected model type returned from task runner")
	}

	return nil
}
