package api

// Server represents a Coolify server
type Server struct {
	ID          int             `json:"id"`
	UUID        string          `json:"uuid"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	IP          string          `json:"ip"`
	User        string          `json:"user"`
	Port        int             `json:"port"`
	Settings    *ServerSettings `json:"settings"`
}

// ServerSettings contains server settings
type ServerSettings struct {
	IsReachable    bool   `json:"is_reachable"`
	IsUsable       bool   `json:"is_usable"`
	WildcardDomain string `json:"wildcard_domain"`
}

// Project represents a Coolify project
type Project struct {
	ID           int           `json:"id"`
	UUID         string        `json:"uuid"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Environments []Environment `json:"environments"`
}

// Environment represents a Coolify environment within a project
type Environment struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid,omitempty"`
	Name        string `json:"name"`
	ProjectID   int    `json:"project_id"`
	Description string `json:"description"`
}

// Application represents a Coolify application
type Application struct {
	ID                          int    `json:"id"`
	UUID                        string `json:"uuid"`
	Name                        string `json:"name"`
	Description                 string `json:"description"`
	FQDN                        string `json:"fqdn"`
	GitRepository               string `json:"git_repository"`
	GitBranch                   string `json:"git_branch"`
	BuildPack                   string `json:"build_pack"`
	InstallCommand              string `json:"install_command"`
	BuildCommand                string `json:"build_command"`
	StartCommand                string `json:"start_command"`
	PortsExposes                string `json:"ports_exposes"`
	Status                      string `json:"status"`
	EnvironmentID               int    `json:"environment_id"`
	DestinationID               int    `json:"destination_id"`
	DockerRegistryName          string `json:"docker_registry_image_name"`
	DockerRegistryTag           string `json:"docker_registry_image_tag"`
	PreviewURLTemplate          string `json:"preview_url_template"`
	IsPreviewDeploymentsEnabled bool   `json:"is_preview_deployments_enabled"`
}

// CreatePublicAppRequest is the request body for creating a public app
type CreatePublicAppRequest struct {
	ProjectUUID      string `json:"project_uuid"`
	ServerUUID       string `json:"server_uuid"`
	EnvironmentName  string `json:"environment_name,omitempty"`
	EnvironmentUUID  string `json:"environment_uuid,omitempty"`
	GitRepository    string `json:"git_repository"`
	GitBranch        string `json:"git_branch"`
	BuildPack        string `json:"build_pack,omitempty"`
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Domains          string `json:"domains,omitempty"`
	InstantDeploy    bool   `json:"instant_deploy,omitempty"`
	InstallCommand   string `json:"install_command,omitempty"`
	BuildCommand     string `json:"build_command,omitempty"`
	StartCommand     string `json:"start_command,omitempty"`
	PortsExposes     string `json:"ports_exposes,omitempty"`
	PublishDirectory string `json:"publish_directory,omitempty"`
	BaseDirectory    string `json:"base_directory,omitempty"`
}

// CreateDockerImageAppRequest is the request body for creating a docker image app
type CreateDockerImageAppRequest struct {
	ProjectUUID             string `json:"project_uuid"`
	ServerUUID              string `json:"server_uuid"`
	EnvironmentName         string `json:"environment_name,omitempty"`
	EnvironmentUUID         string `json:"environment_uuid,omitempty"`
	Name                    string `json:"name,omitempty"`
	Description             string `json:"description,omitempty"`
	Domains                 string `json:"domains,omitempty"`
	InstantDeploy           bool   `json:"instant_deploy,omitempty"`
	DockerRegistryImageName string `json:"docker_registry_image_name"`
	DockerRegistryImageTag  string `json:"docker_registry_image_tag,omitempty"`
	PortsExposes            string `json:"ports_exposes,omitempty"`
}

// CreateAppResponse is the response from creating an app
type CreateAppResponse struct {
	UUID string `json:"uuid"`
}

// DeployResponse is the response from triggering a deployment
type DeployResponse struct {
	Deployments []DeploymentInfo `json:"deployments"`
}

// DeploymentInfo contains info about a triggered deployment
type DeploymentInfo struct {
	Message        string `json:"message"`
	ResourceUUID   string `json:"resource_uuid"`
	DeploymentUUID string `json:"deployment_uuid"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	IsBuildTime bool   `json:"is_build_time"`
	IsPreview   bool   `json:"is_preview"`
}

// HealthCheckResponse is the response from the health check endpoint
type HealthCheckResponse struct {
	Status string `json:"status"`
}

// Team represents a Coolify team
type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GitHubApp represents a GitHub App configured in Coolify
type GitHubApp struct {
	ID             int    `json:"id"`
	UUID           string `json:"uuid"`
	Name           string `json:"name"`
	Organization   string `json:"organization"`
	AppID          int    `json:"app_id"`
	InstallationID int    `json:"installation_id"`
	IsSystemWide   bool   `json:"is_system_wide"`
}

// CreatePrivateGitHubAppRequest is the request body for creating a private GitHub app
type CreatePrivateGitHubAppRequest struct {
	ProjectUUID        string `json:"project_uuid"`
	ServerUUID         string `json:"server_uuid"`
	EnvironmentName    string `json:"environment_name,omitempty"`
	EnvironmentUUID    string `json:"environment_uuid,omitempty"`
	GitHubAppUUID      string `json:"github_app_uuid"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	BuildPack          string `json:"build_pack,omitempty"`
	IsStatic           bool   `json:"is_static,omitempty"`
	Name               string `json:"name,omitempty"`
	Description        string `json:"description,omitempty"`
	Domains            string `json:"domains,omitempty"`
	InstantDeploy      bool   `json:"instant_deploy,omitempty"`
	InstallCommand     string `json:"install_command,omitempty"`
	BuildCommand       string `json:"build_command,omitempty"`
	StartCommand       string `json:"start_command,omitempty"`
	PortsExposes       string `json:"ports_exposes,omitempty"`
	PublishDirectory   string `json:"publish_directory,omitempty"`
	BaseDirectory      string `json:"base_directory,omitempty"`
	HealthCheckEnabled bool   `json:"health_check_enabled,omitempty"`
	HealthCheckPath    string `json:"health_check_path,omitempty"`
}
