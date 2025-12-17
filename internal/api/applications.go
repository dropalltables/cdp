package api

import "fmt"

// ListApplications returns all applications
func (c *Client) ListApplications() ([]Application, error) {
	var apps []Application
	err := c.Get("/applications", &apps)
	return apps, err
}

// GetApplication returns an application by UUID
func (c *Client) GetApplication(uuid string) (*Application, error) {
	var app Application
	err := c.Get("/applications/"+uuid, &app)
	return &app, err
}

// CreatePublicApp creates an application from a public git repository
func (c *Client) CreatePublicApp(req *CreatePublicAppRequest) (*CreateAppResponse, error) {
	var resp CreateAppResponse
	err := c.Post("/applications/public", req, &resp)
	return &resp, err
}

// CreateDockerImageApp creates an application from a Docker registry image
func (c *Client) CreateDockerImageApp(req *CreateDockerImageAppRequest) (*CreateAppResponse, error) {
	var resp CreateAppResponse
	err := c.Post("/applications/dockerimage", req, &resp)
	return &resp, err
}

// UpdateApplication updates an application
func (c *Client) UpdateApplication(uuid string, updates map[string]interface{}) error {
	return c.Patch("/applications/"+uuid, updates, nil)
}

// DeleteApplication deletes an application
func (c *Client) DeleteApplication(uuid string) error {
	return c.Delete("/applications/" + uuid)
}

// GetApplicationEnvVars returns environment variables for an application
func (c *Client) GetApplicationEnvVars(uuid string) ([]EnvVar, error) {
	var envVars []EnvVar
	err := c.Get(fmt.Sprintf("/applications/%s/envs", uuid), &envVars)
	return envVars, err
}

// CreateApplicationEnvVar creates an environment variable for an application
func (c *Client) CreateApplicationEnvVar(uuid, key, value string, isBuildTime, isPreview bool) (*EnvVar, error) {
	body := map[string]interface{}{
		"key":           key,
		"value":         value,
		"is_build_time": isBuildTime,
		"is_preview":    isPreview,
	}
	var envVar EnvVar
	err := c.Post(fmt.Sprintf("/applications/%s/envs", uuid), body, &envVar)
	return &envVar, err
}

// DeleteApplicationEnvVar deletes an environment variable
func (c *Client) DeleteApplicationEnvVar(appUUID, envUUID string) error {
	return c.Delete(fmt.Sprintf("/applications/%s/envs/%s", appUUID, envUUID))
}

// ListGitHubApps returns all GitHub Apps configured in Coolify
func (c *Client) ListGitHubApps() ([]GitHubApp, error) {
	var apps []GitHubApp
	err := c.Get("/github-apps", &apps)
	return apps, err
}

// CreatePrivateGitHubApp creates an application from a private GitHub repository using a GitHub App
func (c *Client) CreatePrivateGitHubApp(req *CreatePrivateGitHubAppRequest) (*CreateAppResponse, error) {
	var resp CreateAppResponse
	err := c.Post("/applications/private-github-app", req, &resp)
	return &resp, err
}
