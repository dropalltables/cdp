package detect

// FrameworkInfo contains detected framework information
type FrameworkInfo struct {
	Name             string
	BuildPack        string // nixpacks, static, dockerfile, dockercompose
	InstallCommand   string
	BuildCommand     string
	StartCommand     string
	PublishDirectory string
	Port             string
	IsStatic         bool
}

// Common build packs
const (
	BuildPackNixpacks      = "nixpacks"
	BuildPackStatic        = "static"
	BuildPackDockerfile    = "dockerfile"
	BuildPackDockerCompose = "dockercompose"
)
