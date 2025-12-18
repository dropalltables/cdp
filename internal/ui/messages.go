package ui

// Message types for BubbleTea task runner communication

// taskStartMsg indicates a task is starting
type taskStartMsg struct{}

// taskCompleteMsg indicates a task has completed
type taskCompleteMsg struct {
	err error
}

// allTasksCompleteMsg indicates all tasks have finished
type allTasksCompleteMsg struct{}
