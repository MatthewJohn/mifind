package filesystem

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the filesystem-api service configuration.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Meilisearch MeilisearchConfig `mapstructure:"meilisearch"`
	Scan        ScanConfig        `mapstructure:"scan"`
	API         APIConfig         `mapstructure:"api"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// MeilisearchConfig holds Meilisearch configuration.
type MeilisearchConfig struct {
	URL       string `mapstructure:"url"`
	APIKey    string `mapstructure:"api_key"`
	IndexName string `mapstructure:"index_name"`
}

// ScanConfig holds filesystem scanning configuration.
type ScanConfig struct {
	Paths           []string `mapstructure:"paths"`
	ExcludedDirs    []string `mapstructure:"excluded_dirs"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
	FollowSymlinks  bool     `mapstructure:"follow_symlinks"`
	MaxDepth        int      `mapstructure:"max_depth"`
	AutoScan        bool     `mapstructure:"auto_scan"`
}

// APIConfig holds API configuration.
type APIConfig struct {
	APIKey      string `mapstructure:"api_key"`
	CORSEnabled bool   `mapstructure:"cors_enabled"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

// LoadConfig loads configuration from file and environment.
func LoadConfig() (*Config, error) {
	// Set defaults
	setDefaults()

	// Read config file - check config/ dir first, then fallback locations
	viper.SetConfigName("filesystem-api")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/mifind")
	viper.AddConfigPath("$HOME/.mifind")

	// Optional config file
	viper.ReadInConfig()

	// Environment variables
	viper.SetEnvPrefix("MIFIND_FILESYSTEM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Unmarshal config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values.
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "127.0.0.1")
	viper.SetDefault("server.port", 8082)
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)

	// Meilisearch defaults
	viper.SetDefault("meilisearch.url", "http://localhost:7700")
	viper.SetDefault("meilisearch.api_key", "")
	viper.SetDefault("meilisearch.index_name", "filesystem")

	// Scan defaults
	viper.SetDefault("scan.paths", []string{})
	viper.SetDefault("scan.excluded_dirs", []string{
		".git", "node_modules", ".venv", "venv",
		".env", "target", "build", "dist",
		"__pycache__", ".pytest_cache",
	})
	viper.SetDefault("scan.exclude_patterns", []string{
		"*.tmp", ".*", "*.swp", "*.bak", "*.log",
	})
	viper.SetDefault("scan.follow_symlinks", false)
	viper.SetDefault("scan.max_depth", 20)
	viper.SetDefault("scan.auto_scan", false)

	// API defaults
	viper.SetDefault("api.api_key", "")
	viper.SetDefault("api.cors_enabled", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate server port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate timeouts
	if c.Server.ReadTimeout < 1 {
		return fmt.Errorf("invalid read timeout: %d", c.Server.ReadTimeout)
	}
	if c.Server.WriteTimeout < 1 {
		return fmt.Errorf("invalid write timeout: %d", c.Server.WriteTimeout)
	}

	// Validate Meilisearch URL
	if c.Meilisearch.URL == "" {
		return fmt.Errorf("meilisearch URL is required")
	}

	// Validate scan paths
	if len(c.Scan.Paths) == 0 {
		return fmt.Errorf("at least one scan path is required")
	}

	// Validate max depth
	if c.Scan.MaxDepth < 1 {
		return fmt.Errorf("invalid max depth: %d", c.Scan.MaxDepth)
	}

	return nil
}

// GetReadTimeout returns the read timeout as a duration.
func (c *Config) GetReadTimeout() time.Duration {
	return time.Duration(c.Server.ReadTimeout) * time.Second
}

// GetWriteTimeout returns the write timeout as a duration.
func (c *Config) GetWriteTimeout() time.Duration {
	return time.Duration(c.Server.WriteTimeout) * time.Second
}

// IsExcludedDir checks if a directory name is in the excluded list.
func (c *Config) IsExcludedDir(name string) bool {
	for _, excluded := range c.Scan.ExcludedDirs {
		if name == excluded {
			return true
		}
	}
	return false
}

// MatchesExcludePattern checks if a file name matches any exclude pattern.
func (c *Config) MatchesExcludePattern(name string) bool {
	for _, pattern := range c.Scan.ExcludePatterns {
		if matched, _ := matchPattern(name, pattern); matched {
			return true
		}
	}
	return false
}

// matchPattern checks if a name matches a glob pattern.
func matchPattern(name, pattern string) (bool, error) {
	// Simple glob matching
	// This is a basic implementation; for production, use path/filepath.Match
	// or a more sophisticated glob library
	if pattern == "*" {
		return true, nil
	}
	if pattern == ".*" && strings.HasPrefix(name, ".") {
		return true, nil
	}
	if strings.HasPrefix(pattern, "*.") && strings.HasSuffix(name, pattern[1:]) {
		return true, nil
	}
	if strings.Contains(pattern, "*") {
		// More complex patterns
		prefix := strings.Split(pattern, "*")[0]
		suffix := strings.Split(pattern, "*")[1]
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			return true, nil
		}
	}
	return name == pattern, nil
}
