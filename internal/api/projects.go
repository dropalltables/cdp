package api

import "fmt"

// ListProjects returns all projects
func (c *Client) ListProjects() ([]Project, error) {
	var projects []Project
	err := c.Get("/projects", &projects)
	return projects, err
}

// GetProject returns a project by UUID
func (c *Client) GetProject(uuid string) (*Project, error) {
	var project Project
	err := c.Get("/projects/"+uuid, &project)
	return &project, err
}

// CreateProject creates a new project
func (c *Client) CreateProject(name, description string) (*Project, error) {
	body := map[string]string{
		"name":        name,
		"description": description,
	}
	var project Project
	err := c.Post("/projects", body, &project)
	return &project, err
}

// CreateEnvironment creates a new environment in a project
func (c *Client) CreateEnvironment(projectUUID, name string) (*Environment, error) {
	body := map[string]string{
		"name": name,
	}
	var env Environment
	err := c.Post(fmt.Sprintf("/projects/%s/environments", projectUUID), body, &env)
	return &env, err
}

// DeleteProject deletes a project by UUID
func (c *Client) DeleteProject(uuid string) error {
	return c.Delete("/projects/" + uuid)
}
