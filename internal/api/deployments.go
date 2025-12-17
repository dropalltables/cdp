package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Deploy triggers a deployment for an application
// pr parameter: 0 for production, >0 for preview deployment
func (c *Client) Deploy(uuid string, force bool, pr int) (*DeployResponse, error) {
	params := map[string]string{
		"uuid": uuid,
	}
	if force {
		params["force"] = "true"
	}
	if pr > 0 {
		params["pr"] = fmt.Sprintf("%d", pr)
	}
	var resp DeployResponse
	err := c.GetWithParams("/deploy", params, &resp)
	return &resp, err
}

// DeployByTag triggers a deployment by tag
func (c *Client) DeployByTag(tag string, force bool) (*DeployResponse, error) {
	params := map[string]string{
		"tag": tag,
	}
	if force {
		params["force"] = "true"
	}
	var resp DeployResponse
	err := c.GetWithParams("/deploy", params, &resp)
	return &resp, err
}

// GetDeploymentLogs returns logs for a deployment
func (c *Client) GetDeploymentLogs(appUUID string) (string, error) {
	var resp DeploymentLogsResponse
	err := c.Get(fmt.Sprintf("/applications/%s/logs", appUUID), &resp)
	return resp.Logs, err
}

// Deployment represents a deployment in Coolify
// Note: Coolify API returns some IDs as strings
type Deployment struct {
	ID               interface{} `json:"id"`
	UUID             string      `json:"uuid"`
	Status           string      `json:"status"`
	ApplicationID    interface{} `json:"application_id"`
	ApplicationUUID  string      `json:"application_uuid"`
	DeploymentUUID   string      `json:"deployment_uuid"`
	PullRequestID    interface{} `json:"pull_request_id"`
	ForceRebuild     bool        `json:"force_rebuild"`
	CommitMessage    string      `json:"commit_message"`
	Commit           string      `json:"commit"`
	GitCommitSha     string      `json:"git_commit_sha"`
	GitType          string      `json:"git_type"`
	OnlyThisServer   bool        `json:"only_this_server"`
	RollbackToUUID   string      `json:"rollback_to"`
	CurrentProcessID string      `json:"current_process_id"`
	DestinationID    interface{} `json:"destination_id"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
}

// DeploymentLogsResponse represents the response from the deployment logs endpoint
type DeploymentLogsResponse struct {
	Logs string `json:"logs"`
}

// ListDeployments returns deployments for an application
func (c *Client) ListDeployments(appUUID string) ([]Deployment, error) {
	var deployments []Deployment
	err := c.Get(fmt.Sprintf("/deployments?application_uuid=%s", appUUID), &deployments)
	return deployments, err
}

// DeploymentDetail contains full deployment info including logs
// Note: Coolify API returns some IDs as strings, so we use json.Number/interface{} for flexibility
type DeploymentDetail struct {
	ID              interface{} `json:"id"`
	ApplicationID   interface{} `json:"application_id"`
	DeploymentUUID  string      `json:"deployment_uuid"`
	Status          string      `json:"status"`
	Logs            string      `json:"logs"`
	Commit          string      `json:"commit"`
	CommitMessage   string      `json:"commit_message"`
	ServerID        interface{} `json:"server_id"`
	ApplicationName string      `json:"application_name"`
	ServerName      string      `json:"server_name"`
	DeploymentURL   string      `json:"deployment_url"`
	DestinationID   interface{} `json:"destination_id"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
}

// GetDeployment returns a specific deployment by UUID with full details
func (c *Client) GetDeployment(deploymentUUID string) (*DeploymentDetail, error) {
	var deployment DeploymentDetail
	err := c.Get(fmt.Sprintf("/deployments/%s", deploymentUUID), &deployment)
	return &deployment, err
}

// LogEntry represents a single log entry from Coolify
type LogEntry struct {
	Output    string `json:"output"`
	Type      string `json:"type"`
	Hidden    bool   `json:"hidden"`
	Timestamp string `json:"timestamp"`
	Order     int    `json:"order"`
}

// ParseLogs extracts readable log output from Coolify's JSON log format
func ParseLogs(rawLogs string) string {
	if rawLogs == "" {
		return ""
	}

	// Try to parse as JSON array
	var entries []LogEntry

	// The logs might be multiple JSON arrays concatenated, so we need to handle that
	// First, try parsing as a single array
	if err := json.Unmarshal([]byte(rawLogs), &entries); err == nil {
		return formatLogEntries(entries)
	}

	// If that fails, try to find and parse JSON arrays within the string
	var allEntries []LogEntry
	remaining := rawLogs

	for len(remaining) > 0 {
		// Find the start of a JSON array
		start := strings.Index(remaining, "[")
		if start == -1 {
			break
		}

		// Find the matching end bracket
		depth := 0
		end := -1
		for i := start; i < len(remaining); i++ {
			if remaining[i] == '[' {
				depth++
			} else if remaining[i] == ']' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}

		if end == -1 {
			break
		}

		// Try to parse this array
		var entries []LogEntry
		if err := json.Unmarshal([]byte(remaining[start:end]), &entries); err == nil {
			allEntries = append(allEntries, entries...)
		}

		remaining = remaining[end:]
	}

	if len(allEntries) > 0 {
		return formatLogEntries(allEntries)
	}

	// If nothing worked, return raw logs
	return rawLogs
}

func formatLogEntries(entries []LogEntry) string {
	var lines []string
	for _, e := range entries {
		// Skip hidden entries and empty output
		if e.Hidden || e.Output == "" {
			continue
		}
		lines = append(lines, e.Output)
	}
	return strings.Join(lines, "\n")
}

// GetBuildLogs returns build logs for a specific deployment
func (c *Client) GetBuildLogs(deploymentUUID string) (string, error) {
	deployment, err := c.GetDeployment(deploymentUUID)
	if err != nil {
		return "", err
	}
	return deployment.Logs, nil
}

// HealthCheck validates the API connection
func (c *Client) HealthCheck() error {
	var resp HealthCheckResponse
	// Try to get the version endpoint instead since healthcheck might not be available
	err := c.Get("/version", &resp)
	if err != nil {
		// If version fails, try listing teams as a validation
		var teams []Team
		err = c.Get("/teams", &teams)
	}
	return err
}
