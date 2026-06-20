package main

import (
	"encoding/json"
	"fmt"
	"os"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// SettingsService exposes application settings to the frontend.
type SettingsService struct{ core *Core }

// Get returns the current settings.
func (s *SettingsService) Get() model.AppSettings { return s.core.Settings() }

// Update replaces the settings, normalising bounds, and persists them.
func (s *SettingsService) Update(next model.AppSettings) (model.AppSettings, error) {
	if next.Concurrency <= 0 {
		next.Concurrency = 5
	}
	if next.PartSize < s3x.MinPartSize {
		next.PartSize = s3x.MinPartSize
	}
	if next.PreviewMaxSize <= 0 {
		next.PreviewMaxSize = 10 * 1024 * 1024
	}
	s.core.settingsMu.Lock()
	s.core.settings = next
	s.core.settingsMu.Unlock()
	if err := s.core.saveSettings(); err != nil {
		return next, err
	}
	s.core.emit("settings:changed", next)
	return next, nil
}

// settingsBundle is the shape of an export/import file.
type settingsBundle struct {
	Version     string             `json:"version"`
	Settings    model.AppSettings  `json:"settings"`
	Connections []model.Connection `json:"connections"`
	Sensitive   bool               `json:"sensitive"`
}

// Export writes all settings (and connections) to a user-chosen JSON file.
// When includeSensitive is false, access/secret keys are blanked.
func (s *SettingsService) Export(includeSensitive bool) (string, error) {
	if s.core.app == nil {
		return "", fmt.Errorf("app not ready")
	}
	path, err := s.core.app.Dialog.SaveFile().
		SetFilename("s3scalpel-settings.json").
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}

	s.core.connMu.RLock()
	conns := make([]model.Connection, len(s.core.conns))
	copy(conns, s.core.conns)
	s.core.connMu.RUnlock()
	if !includeSensitive {
		for i := range conns {
			conns[i].AccessKey = ""
			conns[i].SecretKey = ""
		}
	}
	bundle := settingsBundle{
		Version:     s.core.version,
		Settings:    s.core.Settings(),
		Connections: conns,
		Sensitive:   includeSensitive,
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Import loads settings (and connections) from a user-chosen JSON file. Imported
// connections are merged by id; settings are replaced.
func (s *SettingsService) Import() (bool, error) {
	if s.core.app == nil {
		return false, fmt.Errorf("app not ready")
	}
	path, err := s.core.app.Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var bundle settingsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return false, fmt.Errorf("invalid settings file: %w", err)
	}

	next := bundle.Settings
	if next.Concurrency <= 0 {
		next.Concurrency = 5
	}
	if next.PartSize < s3x.MinPartSize {
		next.PartSize = s3x.MinPartSize
	}
	if next.PreviewMaxSize <= 0 {
		next.PreviewMaxSize = 10 * 1024 * 1024
	}
	s.core.settingsMu.Lock()
	s.core.settings = next
	s.core.settingsMu.Unlock()
	_ = s.core.saveSettings()

	if len(bundle.Connections) > 0 {
		s.core.connMu.Lock()
		index := map[string]int{}
		for i, c := range s.core.conns {
			index[c.ID] = i
		}
		for _, c := range bundle.Connections {
			if c.ID == "" {
				c.ID = randID()
			}
			if i, ok := index[c.ID]; ok {
				// keep existing secrets if the import omitted them
				if c.AccessKey == "" {
					c.AccessKey = s.core.conns[i].AccessKey
				}
				if c.SecretKey == "" {
					c.SecretKey = s.core.conns[i].SecretKey
				}
				s.core.conns[i] = c
			} else {
				s.core.conns = append(s.core.conns, c)
				index[c.ID] = len(s.core.conns) - 1
			}
		}
		s.core.connMu.Unlock()
		_ = s.core.saveConnections()
		s.core.clients.Invalidate("")
		s.core.emit("configs:changed", nil)
	}

	s.core.emit("settings:changed", next)
	return true, nil
}
