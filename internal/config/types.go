package config

import "encoding/json"

const (
	EnvPreview    = "preview"
	EnvProduction = "production"
)

const (
	DeployMethodGit    = "git"
	DeployMethodDocker = "docker"
)

const (
	DefaultPort     = "3000"
	DefaultPlatform = "linux/amd64"
	DefaultBranch   = "main"
)

// GlobalConfig stores credentials and settings for cdp
type GlobalConfig struct {
	CoolifyURL     string          `json:"coolify_url"`
	CoolifyToken   string          `json:"coolify_token"`
	DefaultServer  string          `json:"default_server,omitempty"`
	DefaultProject string          `json:"default_project,omitempty"`
	GitHubToken    string          `json:"github_token,omitempty"`
	DockerRegistry *DockerRegistry `json:"docker_registry,omitempty"`
}

// DockerRegistry stores Docker registry credentials
type DockerRegistry struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProjectConfig stores per-project deployment configuration
type ProjectConfig struct {
	Name            string `json:"name"`
	DeployMethod    string `json:"deploy_method"`
	ProjectUUID     string `json:"project_uuid"`
	ServerUUID      string `json:"server_uuid"`
	EnvironmentUUID string `json:"environment_uuid"`
	AppUUID         string `json:"app_uuid"`
	Framework       string `json:"framework"`
	InstallCommand  string `json:"install_command,omitempty"`
	BuildCommand    string `json:"build_command,omitempty"`
	StartCommand    string `json:"start_command,omitempty"`
	PublishDir      string `json:"publish_dir,omitempty"`
	Port            string `json:"port,omitempty"`
	Platform        string `json:"platform,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Domain          string `json:"domain,omitempty"`
	DockerImage     string `json:"docker_image,omitempty"`
	GitHubRepo      string `json:"github_repo,omitempty"`
	GitHubPrivate   bool   `json:"github_private,omitempty"`
	GitHubAppUUID   string `json:"github_app_uuid,omitempty"`

	// Node.js specific (from coolpack)
	PackageManager        string          `json:"package_manager,omitempty"`
	PackageManagerVersion string          `json:"package_manager_version,omitempty"`
	NodeVersion           string          `json:"node_version,omitempty"`
	CoolpackPlan          json.RawMessage `json:"coolpack_plan,omitempty"`

	// Deprecated: legacy fields for migration
	BuildPack       string            `json:"build_pack,omitempty"`
	DetectionEngine string            `json:"detection_engine,omitempty"`
	PreviewEnvUUID  string            `json:"preview_env_uuid,omitempty"`
	ProdEnvUUID     string            `json:"prod_env_uuid,omitempty"`
	AppUUIDs        map[string]string `json:"app_uuids,omitempty"`
}
