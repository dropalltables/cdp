package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GitHubClient is a simple GitHub API client
type GitHubClient struct {
	token      string
	httpClient *http.Client
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateRepoRequest is the request body for creating a repo
type CreateRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Private     bool   `json:"private"`
	AutoInit    bool   `json:"auto_init"`
}

// Repository represents a GitHub repository
type Repository struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
}

// User represents a GitHub user
type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

// GetUser returns the authenticated user
func (c *GitHubClient) GetUser() (*User, error) {
	var user User
	err := c.request("GET", "https://api.github.com/user", nil, &user)
	return &user, err
}

// CreateRepo creates a new repository
func (c *GitHubClient) CreateRepo(name, description string, private bool) (*Repository, error) {
	req := &CreateRepoRequest{
		Name:        name,
		Description: description,
		Private:     private,
		AutoInit:    false,
	}
	var repo Repository
	err := c.request("POST", "https://api.github.com/user/repos", req, &repo)
	return &repo, err
}

// GetRepo gets a repository by owner and name
func (c *GitHubClient) GetRepo(owner, name string) (*Repository, error) {
	var repo Repository
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	err := c.request("GET", url, nil, &repo)
	return &repo, err
}

// RepoExists checks if a repository exists
func (c *GitHubClient) RepoExists(owner, name string) bool {
	_, err := c.GetRepo(owner, name)
	return err == nil
}

// DeleteRepo deletes a repository
func (c *GitHubClient) DeleteRepo(owner, name string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	return c.request("DELETE", url, nil, nil)
}

func (c *GitHubClient) request(method, url string, body interface{}, result interface{}) error {
	debug := os.Getenv("CDP_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] GitHub API: %s %s\n", method, url)
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Request body: %s\n", string(jsonBody))
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Sending request...\n")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Request failed: %v\n", err)
		}
		return err
	}
	defer resp.Body.Close()

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Response status: %d\n", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if debug && len(respBody) > 0 {
		fmt.Fprintf(os.Stderr, "[DEBUG] Response body: %s\n", string(respBody))
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, result)
	}

	return nil
}

// GenerateRepoName generates a repository name for deployment
func GenerateRepoName(projectName string) string {
	// Clean up the name
	name := strings.ToLower(projectName)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name + "-deploy"
}
