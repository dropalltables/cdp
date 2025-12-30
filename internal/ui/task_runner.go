package ui

import (
	"fmt"
	"time"
)

// Task represents an async operation to run
type Task struct {
	Name         string       // Unique identifier for the task
	ActiveName   string       // Message shown while task is running
	CompleteName string       // Message shown when task completes
	Action       func() error // Function to execute
}

// RunTasks executes a sequence of tasks with spinner feedback
func RunTasks(tasks []Task) error {
	return RunTasksVerbose(tasks, false)
}

// RunTasksVerbose executes a sequence of tasks with optional verbose mode
func RunTasksVerbose(tasks []Task, verbose bool) error {
	if len(tasks) == 0 {
		return nil
	}

	for _, task := range tasks {
		if verbose {
			// In verbose mode, skip spinner and run action directly
			err := task.Action()
			if err != nil {
				Error(task.ActiveName)
				return err
			}
			Success(task.CompleteName)
		} else {
			// In normal mode, use spinner
			spinner := NewSpinner(task.ActiveName)
			spinner.Start()

			err := task.Action()

			if err != nil {
				spinner.StopWithError(task.ActiveName)
				return err
			}

			spinner.StopWithSuccess(task.CompleteName)
		}
	}

	return nil
}

// Spinner provides a simple streaming spinner
type Spinner struct {
	message string
	frames  []string
	done    chan struct{}
	stopped chan struct{}
	stopped_bool bool
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  []string{"|", "/", "-", "\\"},
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		stopped_bool: false,
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		frame := 0
		for {
			select {
			case <-s.done:
				close(s.stopped)
				return
			default:
				fmt.Printf("\r%s %s", CyanStyle.Render(s.frames[frame%len(s.frames)]), s.message)
				frame++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	if s.stopped_bool {
		return
	}
	s.stopped_bool = true
	close(s.done)
	<-s.stopped // Wait for goroutine to finish
	fmt.Print("\r\033[K")
}

// StopWithSuccess stops and shows success message
func (s *Spinner) StopWithSuccess(message string) {
	s.Stop()
	Success(message)
}

// StopWithError stops and shows error message
func (s *Spinner) StopWithError(message string) {
	s.Stop()
	Error(message)
}
