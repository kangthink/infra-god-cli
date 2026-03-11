package inventory

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level servers.yaml structure.
type Config struct {
	Defaults Defaults          `yaml:"defaults"`
	Servers  map[string]Server `yaml:"servers"`
	Groups   map[string][]string `yaml:"groups"`
}

// Defaults holds default SSH connection settings.
type Defaults struct {
	SSHUser string `yaml:"ssh_user"`
	SSHPort int    `yaml:"ssh_port"`
	Prefer  string `yaml:"prefer"`
	Auth    Auth   `yaml:"auth"`
}

// Auth holds authentication configuration.
type Auth struct {
	Type        string `yaml:"type"`        // "password" or "key"
	PasswordEnv string `yaml:"password_env"` // environment variable name
	KeyPath     string `yaml:"key_path"`
}

// Server represents a single server entry.
type Server struct {
	Host        interface{} `yaml:"host"` // string or map[string]string
	Role        string      `yaml:"role"`
	Status      string      `yaml:"status"`
	Description string      `yaml:"description"`
	SSHUser     string      `yaml:"ssh_user"`
	SSHPort     int         `yaml:"ssh_port"`
	Auth        *Auth       `yaml:"auth"`
}

// ResolvedServer is a server with all defaults applied and IPs resolved.
type ResolvedServer struct {
	Name     string
	WiredIP  string
	WirelessIP string
	Role     string
	Status   string
	SSHUser  string
	SSHPort  int
	AuthType string // "password" or "key"
	Password string
	KeyPath  string
}

// PrimaryIP returns the preferred IP (wired first, then wireless).
func (s *ResolvedServer) PrimaryIP() string {
	if s.WiredIP != "" {
		return s.WiredIP
	}
	return s.WirelessIP
}

// FallbackIP returns the fallback IP if primary fails.
func (s *ResolvedServer) FallbackIP() string {
	if s.WiredIP != "" && s.WirelessIP != "" {
		return s.WirelessIP
	}
	return ""
}

// IsActive returns true if the server is not stopped.
func (s *ResolvedServer) IsActive() bool {
	return s.Status != "stopped"
}

// DefaultConfigPath returns ~/.infra-god/servers.yaml or ./servers.yaml
func DefaultConfigPath() string {
	// Check current directory first
	if _, err := os.Stat("servers.yaml"); err == nil {
		return "servers.yaml"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "servers.yaml"
	}
	return filepath.Join(home, ".infra-god", "servers.yaml")
}

// Load reads and parses servers.yaml, returning resolved servers.
func Load(path string) (*Config, map[string]*ResolvedServer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	if cfg.Defaults.SSHPort == 0 {
		cfg.Defaults.SSHPort = 22
	}
	if cfg.Defaults.SSHUser == "" {
		cfg.Defaults.SSHUser = "root"
	}
	if cfg.Defaults.Auth.Type == "" {
		cfg.Defaults.Auth.Type = "password"
	}

	servers := make(map[string]*ResolvedServer)
	for name, srv := range cfg.Servers {
		resolved := &ResolvedServer{
			Name:   name,
			Role:   srv.Role,
			Status: srv.Status,
		}

		// Resolve host IPs
		switch h := srv.Host.(type) {
		case string:
			resolved.WiredIP = h
		case map[string]interface{}:
			if w, ok := h["wired"]; ok {
				resolved.WiredIP = fmt.Sprintf("%v", w)
			}
			if w, ok := h["wireless"]; ok {
				resolved.WirelessIP = fmt.Sprintf("%v", w)
			}
		case nil:
			// host is null (stopped server)
		}

		// Resolve SSH user
		resolved.SSHUser = cfg.Defaults.SSHUser
		if srv.SSHUser != "" {
			resolved.SSHUser = srv.SSHUser
		}

		// Resolve SSH port
		resolved.SSHPort = cfg.Defaults.SSHPort
		if srv.SSHPort != 0 {
			resolved.SSHPort = srv.SSHPort
		}

		// Resolve auth
		if srv.Auth != nil {
			resolved.AuthType = srv.Auth.Type
			resolved.KeyPath = srv.Auth.KeyPath
			if srv.Auth.PasswordEnv != "" {
				resolved.Password = os.Getenv(srv.Auth.PasswordEnv)
			}
		} else {
			resolved.AuthType = cfg.Defaults.Auth.Type
			resolved.KeyPath = cfg.Defaults.Auth.KeyPath
			if cfg.Defaults.Auth.PasswordEnv != "" {
				resolved.Password = os.Getenv(cfg.Defaults.Auth.PasswordEnv)
			}
		}

		servers[name] = resolved
	}

	return &cfg, servers, nil
}

// Save writes the config back to the given path.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ConfigPath returns the resolved config path (used for save operations).
func ConfigPath(cfgPath string) string {
	if cfgPath != "" {
		return cfgPath
	}
	return DefaultConfigPath()
}

// ResolveTargets returns servers matching the target specification.
// target can be: server name, --all, --group <name>
func ResolveTargets(cfg *Config, servers map[string]*ResolvedServer, names []string, group string, all bool) ([]*ResolvedServer, error) {
	var result []*ResolvedServer

	if all {
		// All active servers
		for _, srv := range servers {
			if srv.IsActive() {
				result = append(result, srv)
			}
		}
		return result, nil
	}

	if group != "" {
		members, ok := cfg.Groups[group]
		if !ok {
			return nil, fmt.Errorf("unknown group: %s", group)
		}
		for _, name := range members {
			if srv, ok := servers[name]; ok && srv.IsActive() {
				result = append(result, srv)
			}
		}
		return result, nil
	}

	// Specific server names
	for _, name := range names {
		srv, ok := servers[name]
		if !ok {
			return nil, fmt.Errorf("unknown server: %s", name)
		}
		result = append(result, srv)
	}
	return result, nil
}
