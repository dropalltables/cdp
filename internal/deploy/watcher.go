package deploy

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/ui"
	"golang.org/x/term"
)

const (
	// Polling configuration
	maxPollAttempts      = 120 // 4 minutes max (2s intervals)
	pollInterval         = 2 * time.Second
	noDeploymentTimeout  = 15 // attempts before giving up if no deployment found
	maxConsecutiveErrors = 5  // max API errors before giving up
)

// WatchResult represents the outcome of watching a deployment
type WatchResult int

const (
	WatchSuccess WatchResult = iota
	WatchFailed
	WatchCancelled
)

// WatchDeployment polls the deployment status and displays build logs.
// Returns true if deployment succeeded, false if it failed.
func WatchDeployment(client *api.Client, appUUID string) bool {
	result := WatchDeploymentWithCancel(client, appUUID)
	return result == WatchSuccess
}

// WatchDeploymentWithCancel polls the deployment status and allows cancellation with 'q'.
// Returns WatchSuccess, WatchFailed, or WatchCancelled.
func WatchDeploymentWithCancel(client *api.Client, appUUID string) WatchResult {
	debug := os.Getenv("CDP_DEBUG") != ""
	if debug {
		fmt.Printf("[DEBUG] Watching app UUID: %s\n", appUUID)
	}

	watcher := &deploymentWatcher{
		client:            client,
		appUUID:           appUUID,
		debug:             debug,
		consecutiveErrors: 0,
		lastLogLen:        0,
		cancelChan:        make(chan struct{}),
	}

	// Set up raw mode in main goroutine so restore is guaranteed
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
			// Start keyboard listener for 'q' to cancel (no terminal management inside)
			go watcher.listenForCancel(fd)
			ui.Dim("Press 'q' to cancel deployment\r")
			fmt.Print("\r\n")
		}
	}

	return watcher.watch()
}

type deploymentWatcher struct {
	client             *api.Client
	appUUID            string
	debug              bool
	consecutiveErrors  int
	lastLogLen         int
	lastDeploymentUUID string
	seenDeployment     bool
	cancelChan         chan struct{}
	cancelled          bool
}

func (w *deploymentWatcher) listenForCancel(fd int) {
	buf := make([]byte, 1)
	for {
		select {
		case <-w.cancelChan:
			return
		default:
			// Use short timeout for responsive cancellation
			os.Stdin.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _ := os.Stdin.Read(buf)
			if n > 0 && (buf[0] == 'q' || buf[0] == 'Q') {
				w.cancelled = true
				select {
				case <-w.cancelChan:
				default:
					close(w.cancelChan)
				}
				return
			}
		}
	}
}

func (w *deploymentWatcher) watch() WatchResult {
	defer func() {
		// Signal cancel listener to stop
		select {
		case <-w.cancelChan:
		default:
			close(w.cancelChan)
		}
	}()

	for attempt := 0; attempt < maxPollAttempts; attempt++ {
		// Check if user pressed 'q'
		if w.cancelled {
			return w.handleCancellation()
		}

		status, done := w.checkDeploymentStatus(attempt)
		if done {
			if status == deploymentSuccess {
				return WatchSuccess
			}
			return WatchFailed
		}
		
		// Print progress every 30 attempts (1 minute)
		if attempt > 0 && attempt%30 == 0 && w.debug {
			fmt.Printf("[DEBUG] Still waiting... (attempt %d)\n", attempt)
		}
		
		time.Sleep(pollInterval)
	}

	// Timeout reached - make final check
	if w.debug {
		fmt.Printf("[DEBUG] Reached max poll attempts (%d), making final check\n", maxPollAttempts)
	}
	if w.checkFinalStatus() {
		return WatchSuccess
	}
	return WatchFailed
}

func (w *deploymentWatcher) handleCancellation() WatchResult {
	fmt.Print("\r\n")
	ui.Warning("Cancelling deployment...")

	if w.lastDeploymentUUID != "" {
		err := w.client.CancelDeployment(w.lastDeploymentUUID)
		if err != nil {
			if w.debug {
				fmt.Printf("[DEBUG] Cancel error: %v\n", err)
			}
			ui.Error("Failed to cancel deployment")
		} else {
			ui.Success("Deployment cancelled")
		}
	}

	return WatchCancelled
}

type deploymentStatus int

const (
	deploymentInProgress deploymentStatus = iota
	deploymentSuccess
	deploymentFailed
)

func (w *deploymentWatcher) checkDeploymentStatus(attempt int) (deploymentStatus, bool) {
	// Get deployments for the app
	deployments, err := w.client.ListDeployments(w.appUUID)
	if err != nil {
		return w.handleAPIError(err)
	}

	// Reset error counter on successful API call
	w.consecutiveErrors = 0

	// No deployments found
	if len(deployments) == 0 {
		return w.handleNoDeployments(attempt)
	}

	// Found deployments
	w.seenDeployment = true
	return w.processDeployment(deployments[0], attempt)
}

func (w *deploymentWatcher) handleAPIError(err error) (deploymentStatus, bool) {
	if w.debug {
		fmt.Printf("[DEBUG] ListDeployments error: %v\n", err)
	}

	w.consecutiveErrors++
	if w.consecutiveErrors >= maxConsecutiveErrors {
		if w.debug {
			fmt.Printf("[DEBUG] Too many consecutive errors, giving up\n")
		}
		return deploymentFailed, true
	}

	return deploymentInProgress, false
}

func (w *deploymentWatcher) handleNoDeployments(attempt int) (deploymentStatus, bool) {
	// If we never saw a deployment after reasonable wait, give up
	if !w.seenDeployment && attempt >= noDeploymentTimeout {
		if w.debug {
			fmt.Printf("[DEBUG] No deployment found after %d attempts\n", attempt)
		}
		return deploymentFailed, true
	}

	// If we SAW a deployment but it's now gone, deployment finished - check app status
	if w.seenDeployment {
		if w.debug {
			fmt.Printf("[DEBUG] Deployment list empty after seeing deployment, checking app status\n")
		}
		return w.checkAppAndFinish()
	}

	if w.debug && attempt%10 == 0 {
		fmt.Printf("[DEBUG] No deployments (attempt %d)\n", attempt)
	}

	return deploymentInProgress, false
}

func (w *deploymentWatcher) checkAppAndFinish() (deploymentStatus, bool) {
	app, err := w.client.GetApplication(w.appUUID)
	if err == nil {
		status, done := w.checkStatus(app.Status)
		if done {
			return status, true
		}
		// Check if running
		if strings.Contains(strings.ToLower(app.Status), "running") {
			return deploymentSuccess, true
		}
	}
	// Default to success (deployment likely completed)
	return deploymentSuccess, true
}

func (w *deploymentWatcher) processDeployment(deployment api.Deployment, attempt int) (deploymentStatus, bool) {
	// Determine deployment UUID
	deployUUID := deployment.DeploymentUUID
	if deployUUID == "" {
		deployUUID = deployment.UUID
	}

	// Track new deployment
	if deployUUID != w.lastDeploymentUUID {
		if w.debug {
			fmt.Printf("[DEBUG] New deployment UUID: %s\n", deployUUID)
		}
		w.lastDeploymentUUID = deployUUID
		w.lastLogLen = 0
	}

	// Try to get detailed deployment info with logs
	detail, err := w.client.GetDeployment(deployUUID)
	if err != nil {
		if w.debug {
			fmt.Printf("[DEBUG] GetDeployment error: %v\n", err)
		}
	} else {
		// Print new logs
		w.printNewLogs(detail.Logs)

		// Check status from detailed info
		if status, done := w.checkStatus(detail.Status); done {
			return status, true
		}
	}

	// Fallback: check status from deployment list
	if status, done := w.checkStatus(deployment.Status); done {
		return status, true
	}

	return deploymentInProgress, false
}

func (w *deploymentWatcher) printNewLogs(rawLogs string) {
	parsedLogs := api.ParseLogs(rawLogs)
	if len(parsedLogs) > w.lastLogLen {
		newContent := parsedLogs[w.lastLogLen:]
		lines := strings.Split(newContent, "\n")
		for _, line := range lines {
			if line != "" {
				// Use \r\n for raw terminal mode compatibility
				fmt.Print(ui.DimStyle.Render("  "+line) + "\r\n")
			}
		}
		w.lastLogLen = len(parsedLogs)
	}
}

func (w *deploymentWatcher) checkStatus(status string) (deploymentStatus, bool) {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))

	if w.debug {
		fmt.Printf("[DEBUG] Deployment status: %q\n", normalizedStatus)
	}

	switch {
	case normalizedStatus == "finished":
		return deploymentSuccess, true
	case normalizedStatus == "failed" || normalizedStatus == "error" || normalizedStatus == "cancelled":
		return deploymentFailed, true
	case normalizedStatus == "running" || normalizedStatus == "in_progress" || normalizedStatus == "queued":
		return deploymentInProgress, false
	case strings.Contains(normalizedStatus, "running") && strings.Contains(normalizedStatus, "healthy"):
		// Status like "running:healthy" indicates successful deployment
		return deploymentSuccess, true
	default:
		// Unknown status, keep watching
		if w.debug {
			fmt.Printf("[DEBUG] Unknown status, continuing to wait\n")
		}
		return deploymentInProgress, false
	}
}

func (w *deploymentWatcher) checkFinalStatus() bool {
	if w.debug {
		fmt.Printf("[DEBUG] Timeout reached, checking final app status\n")
	}

	app, err := w.client.GetApplication(w.appUUID)
	if err != nil {
		if w.debug {
			fmt.Printf("[DEBUG] GetApplication error: %v\n", err)
		}
		return false
	}

	appStatus := strings.ToLower(strings.TrimSpace(app.Status))
	if w.debug {
		fmt.Printf("[DEBUG] Final application status: %s\n", appStatus)
	}

	return appStatus == "running"
}
