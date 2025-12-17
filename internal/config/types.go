package config

// Environment names
const (
	EnvPreview    = "preview"
	EnvProduction = "production"
)

// Deployment methods
const (
	DeployMethodGit    = "git"
	DeployMethodDocker = "docker"
)

// Default values
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
	Name           string            `json:"name"`
	DeployMethod   string            `json:"deploy_method"` // "docker" or "git"
	ProjectUUID    string            `json:"project_uuid"`
	ServerUUID     string            `json:"server_uuid"`
	PreviewEnvUUID string            `json:"preview_env_uuid"`
	ProdEnvUUID    string            `json:"prod_env_uuid"`
	AppUUIDs       map[string]string `json:"app_uuids"` // "preview" -> uuid, "production" -> uuid
	Framework      string            `json:"framework"`
	BuildPack      string            `json:"build_pack,omitempty"` // nixpacks, static, dockerfile
	InstallCommand string            `json:"install_command,omitempty"`
	BuildCommand   string            `json:"build_command,omitempty"`
	StartCommand   string            `json:"start_command,omitempty"`
	PublishDir     string            `json:"publish_dir,omitempty"`
	Port           string            `json:"port,omitempty"`
	Platform       string            `json:"platform,omitempty"`       // linux/amd64, linux/arm64
	Branch         string            `json:"branch,omitempty"`         // git branch to deploy
	Domain         string            `json:"domain,omitempty"`         // custom domain or empty for auto
	DockerImage    string            `json:"docker_image,omitempty"`
	GitHubRepo     string            `json:"github_repo,omitempty"`
	GitHubPrivate  bool              `json:"github_private,omitempty"`
}
