package config

import "fmt"

type Config struct {
	Name      string            `yaml:"name"`
	Framework string            `yaml:"framework"`
	Domain    string            `yaml:"domain"`
	Server    ServerConfig      `yaml:"server"`
	Build     BuildConfig       `yaml:"build"`
	Env       map[string]string `yaml:"env"`
	Deploy    DeployConfig      `yaml:"deploy"`
	Nginx     NginxConfig       `yaml:"nginx"`
	Docker    DockerConfig      `yaml:"docker"`
}

type ServerConfig struct {
	Host       string `yaml:"host"`
	User       string `yaml:"user"`
	Key        string `yaml:"key"`
	Method     string `yaml:"method"`
	InstanceID string `yaml:"instance_id"`
	Port       int    `yaml:"port"`
}

type BuildConfig struct {
	Command string `yaml:"command"`
	Output  string `yaml:"output"`
}

type DeployConfig struct {
	KeepReleases int    `yaml:"keep_releases"`
	Strategy     string `yaml:"strategy"`
}

type NginxConfig struct {
	Template string `yaml:"template"`
	Port     int    `yaml:"port"`
}

type DockerConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Registry string `yaml:"registry,omitempty"`
	Image    string `yaml:"image,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Template string `yaml:"template,omitempty"` // spa | nextjs | node-ssr
}

func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if c.Server.Host == "" && c.Server.Method != "ssm" {
		return fmt.Errorf("server.host is required for SSH connections")
	}
	if c.Server.Method == "ssm" && c.Server.InstanceID == "" {
		return fmt.Errorf("server.instance_id is required for SSM connections")
	}
	if c.Server.Method != "" && c.Server.Method != "ssh" && c.Server.Method != "ssm" {
		return fmt.Errorf("server.method must be 'ssh' or 'ssm'")
	}
	if c.Framework != "" && !isValidFramework(c.Framework) {
		return fmt.Errorf("invalid framework: %s (must be vite, nextjs, react, react-cra, astro, sveltekit, nuxt, remix, static, or unknown)", c.Framework)
	}
	return nil
}

func (c *Config) WebRoot() string {
	return fmt.Sprintf("/var/www/%s", c.Name)
}

func (c *Config) SetDefaults() {
	if c.Server.User == "" {
		c.Server.User = "ubuntu"
	}
	if c.Server.Method == "" {
		c.Server.Method = "ssh"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 22
	}
	if c.Build.Command == "" {
		c.Build.Command = "npm run build"
	}
	if c.Build.Output == "" {
		switch c.Framework {
		case "nextjs":
			c.Build.Output = ".next"
		case "react", "react-cra":
			c.Build.Output = "build"
		case "nuxt":
			c.Build.Output = ".output"
		case "sveltekit":
			c.Build.Output = "build"
		default:
			c.Build.Output = "dist"
		}
	}
	if c.Deploy.KeepReleases == 0 {
		c.Deploy.KeepReleases = 5
	}
	if c.Deploy.Strategy == "" {
		c.Deploy.Strategy = "rolling"
	}
	if c.Nginx.Template == "" {
		switch c.Framework {
		case "nextjs", "nuxt", "remix":
			c.Nginx.Template = "ssr"
		case "sveltekit":
			// SvelteKit depends on adapter; default to SSR
			c.Nginx.Template = "ssr"
		case "astro":
			// Astro depends on adapter; default to static
			c.Nginx.Template = "spa"
		default:
			c.Nginx.Template = "spa"
		}
	}
	if c.Nginx.Port == 0 {
		c.Nginx.Port = 3000
	}
	if c.Docker.Port == 0 {
		c.Docker.Port = 80
	}
	if c.Env == nil {
		c.Env = make(map[string]string)
	}
}

func isValidFramework(f string) bool {
	switch f {
	case "vite", "nextjs", "react", "react-cra", "static",
		"astro", "sveltekit", "nuxt", "remix", "unknown":
		return true
	}
	return false
}
