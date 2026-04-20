// Package config loads and persists the StatusPulse CLI's local state —
// the API base URL and the session token. Precedence (highest wins):
//
//  1. --api-url flag (set on the root command)
//  2. STATUSPULSE_API_URL / STATUSPULSE_API_KEY env vars
//  3. ~/.config/cloudbox/statuspulse.json on disk
//  4. Built-in defaults (prod URL, no token)
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultAPIURL = "https://statuspulse.cloudbox.sh"
	envAPIURL     = "STATUSPULSE_API_URL"
	envAPIKey     = "STATUSPULSE_API_KEY"
)

// Config is the on-disk shape. The file also acts as a "logged in" marker —
// its presence means we have a token we can try.
type Config struct {
	APIURL    string `json:"api_url,omitempty"`
	Token     string `json:"token,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
}

// Path returns the absolute path to the config file. It respects
// XDG_CONFIG_HOME and falls back to ~/.config.
func Path() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "cloudbox", "statuspulse.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cloudbox", "statuspulse.json"), nil
}

// Load reads the config file. A missing file is not an error — it returns
// an empty Config.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config at %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the config file atomically, creating the parent directory and
// restricting permissions (0600) because it holds the session token.
func Save(c *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("swap config file: %w", err)
	}
	return nil
}

// Clear deletes the config file. Missing file is not an error.
func Clear() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove config: %w", err)
	}
	return nil
}

// Resolved is the effective config after layering env vars and flag overrides
// on top of the on-disk config. It is what command handlers consume.
type Resolved struct {
	APIURL string
	Token  string
	// Source describes where Token came from so error messages can point the
	// user at the right fix (run `statuspulse login` vs. unset env var).
	Source string
}

// Resolve loads the disk config and layers env vars and the flag override on
// top. The apiURLFlag argument is the value of the root command's --api-url
// flag (empty string if unset).
func Resolve(apiURLFlag string) (*Resolved, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}

	r := &Resolved{
		APIURL: DefaultAPIURL,
		Source: "none",
	}
	if c.APIURL != "" {
		r.APIURL = c.APIURL
	}
	if v := os.Getenv(envAPIURL); v != "" {
		r.APIURL = v
	}
	if apiURLFlag != "" {
		r.APIURL = apiURLFlag
	}

	if c.Token != "" {
		r.Token = c.Token
		r.Source = "config"
	}
	if v := os.Getenv(envAPIKey); v != "" {
		r.Token = v
		r.Source = "env:" + envAPIKey
	}
	return r, nil
}

// ErrNotAuthenticated is returned by commands that require a token when none
// is available.
var ErrNotAuthenticated = errors.New(`not logged in — run 'statuspulse login' or set ` + envAPIKey)
