package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

const (
	panelDir       = ".okdock"
	legacyPanelDir = ".gamedock"
	dnsFile        = "dns.json"
	panelFile      = "config.json"
	templatesDir   = "templates"
)

// where the templates written in the panel live, the config folder when nothing is chosen
func (s *Store) TemplatesDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.templates != "" {
		return s.templates
	}
	return filepath.Join(s.ConfigRoot, panelDir, templatesDir)
}

func (s *Store) readPanel(file string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(s.ConfigRoot, panelDir, file))
	if errors.Is(err, os.ErrNotExist) {
		return os.ReadFile(filepath.Join(s.ConfigRoot, legacyPanelDir, file))
	}
	return raw, err
}

type PanelConfig struct {
	Root      string `json:"root,omitempty"`
	Templates string `json:"templates,omitempty"`
}

func (s *Store) PanelPath() string {
	return filepath.Join(s.ConfigRoot, panelDir, panelFile)
}

func (s *Store) LoadPanel() (PanelConfig, error) {
	var cfg PanelConfig
	raw, err := s.readPanel(panelFile)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return PanelConfig{}, err
	}
	return cfg, nil
}

func (s *Store) SavePanel(cfg PanelConfig) error {
	if err := os.MkdirAll(filepath.Join(s.ConfigRoot, panelDir), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.PanelPath(), append(raw, '\n'), 0o600)
}

type DNSConfig struct {
	Token   string                  `json:"token,omitempty"`
	Links   map[string]instance.DNS `json:"links,omitempty"`
	Domains map[string]instance.DNS `json:"domains,omitempty"`
}

func (s *Store) DNSPath() string {
	return filepath.Join(s.ConfigRoot, panelDir, dnsFile)
}

func (s *Store) LoadDNS() (DNSConfig, error) {
	cfg := DNSConfig{Links: map[string]instance.DNS{}, Domains: map[string]instance.DNS{}}
	raw, err := s.readPanel(dnsFile)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DNSConfig{Links: map[string]instance.DNS{}, Domains: map[string]instance.DNS{}}, err
	}
	if cfg.Links == nil {
		cfg.Links = map[string]instance.DNS{}
	}
	if cfg.Domains == nil {
		cfg.Domains = map[string]instance.DNS{}
	}
	return cfg, nil
}

func (s *Store) SaveDNS(cfg DNSConfig) error {
	dir := filepath.Join(s.ConfigRoot, panelDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.DNSPath(), append(raw, '\n'), 0o600)
}
