package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all user-editable settings for the desktop agent.
// It is persisted as JSON under %AppData%/SmartPrint/config.json.
type Config struct {
	ShopID            string `json:"shopId"`
	AuthToken         string `json:"authToken"`
	PusherAppKey      string `json:"pusherAppKey"`
	PusherCluster     string `json:"pusherCluster"`
	PusherAuthURL     string `json:"pusherAuthUrl"`
	BlackWhitePrinter string `json:"blackWhitePrinter"`
	ColorPrinter      string `json:"colorPrinter"`
	SilentAutoPrint   bool   `json:"silentAutoPrint"`
}

// Store wraps a Config with a mutex and knows how to load/save itself.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// NewStore creates a Store, ensuring the backing directory exists and
// loading any previously saved configuration from disk.
func NewStore() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	s := &Store{path: filepath.Join(dir, "config.json")}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// configDir returns %AppData%/SmartPrint on Windows, falling back to the
// user config dir on other platforms (useful for local dev on macOS/Linux).
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "SmartPrint"), nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.cfg)
}

// Get returns a copy of the current configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Save persists a new configuration to disk and updates the in-memory copy.
func (s *Store) Save(cfg Config) error {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// IsConfigured reports whether the minimum settings needed to run are present.
func (s *Store) IsConfigured() bool {
	c := s.Get()
	return c.ShopID != "" && c.AuthToken != ""
}
