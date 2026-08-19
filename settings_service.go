package main

import (
	"encoding/json"
	"fmt"
	"os"

	"s3scalpel/internal/model"
	"s3scalpel/internal/store"
)

// SettingsService exposes application settings to the frontend.
type SettingsService struct{ core *Core }

// Get returns the current settings.
func (s *SettingsService) Get() model.AppSettings { return s.core.Settings() }

// Update replaces the settings, normalising bounds, persisting them and pushing
// the queue-related values onto every open window.
func (s *SettingsService) Update(next model.AppSettings) (model.AppSettings, error) {
	return s.core.setSettings(next)
}

// settingsBundle is the shape of an export/import file.
type settingsBundle struct {
	Version     string             `json:"version"`
	Settings    model.AppSettings  `json:"settings"`
	Connections []model.Connection `json:"connections"`
	Sensitive   bool               `json:"sensitive"`
}

// Export writes all settings (and connections) to a user-chosen file.
//
// Credentials never leave the app as plaintext: including them requires a
// passphrase, and the file is then sealed with AES-256-GCM under an Argon2id key.
// Without credentials the export stays plain JSON, which is convenient to diff
// and carries nothing sensitive.
func (s *SettingsService) Export(includeSensitive bool, passphrase string) (string, error) {
	if s.core.app == nil {
		return "", fmt.Errorf("app not ready")
	}
	if includeSensitive && passphrase == "" {
		return "", fmt.Errorf("a passphrase is required to export credentials")
	}

	filename := "s3scalpel-settings.json"
	if includeSensitive {
		filename = "s3scalpel-settings.encrypted.json"
	}
	path, err := s.core.app.Dialog.SaveFile().
		SetFilename(filename).
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
			conns[i].SessionToken = ""
		}
	}
	bundle := settingsBundle{
		Version:     s.core.version,
		Settings:    s.core.Settings(),
		Connections: conns,
		Sensitive:   includeSensitive,
	}

	var data []byte
	if includeSensitive {
		data, err = store.SealExport(bundle, passphrase)
	} else {
		data, err = json.MarshalIndent(bundle, "", "  ")
	}
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ImportNeedsPassphrase asks the user for a file and reports whether it is
// sealed, so the frontend knows to prompt before calling Import. It returns the
// chosen path, which Import then reads.
func (s *SettingsService) ImportNeedsPassphrase() (model.ImportProbe, error) {
	if s.core.app == nil {
		return model.ImportProbe{}, fmt.Errorf("app not ready")
	}
	path, err := s.core.app.Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
	if err != nil {
		return model.ImportProbe{}, err
	}
	if path == "" {
		return model.ImportProbe{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return model.ImportProbe{}, err
	}
	return model.ImportProbe{Path: path, Encrypted: store.IsSealedExport(data)}, nil
}

// Import loads settings (and connections) from a file previously chosen through
// ImportNeedsPassphrase. Imported connections are merged by id; settings are
// replaced. A sealed file needs the passphrase it was exported with.
func (s *SettingsService) Import(path, passphrase string) (bool, error) {
	if path == "" {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var bundle settingsBundle
	if store.IsSealedExport(data) {
		if passphrase == "" {
			return false, fmt.Errorf("this export is encrypted; a passphrase is required")
		}
		if err := store.OpenExport(data, passphrase, &bundle); err != nil {
			return false, err
		}
	} else if err := json.Unmarshal(data, &bundle); err != nil {
		return false, fmt.Errorf("invalid settings file: %w", err)
	}

	if _, err := s.core.setSettings(bundle.Settings); err != nil {
		return false, err
	}

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
				if c.SessionToken == "" {
					c.SessionToken = s.core.conns[i].SessionToken
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

	return true, nil
}
