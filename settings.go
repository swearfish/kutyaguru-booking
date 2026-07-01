package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Settings holds user preferences persisted across sessions.
type Settings struct {
	ColorScheme string            `json:"colorScheme"` // "light" | "dark" | "auto"
	Encoding    string            `json:"encoding"`    // "ISO-8859-2" | "UTF-8"
	CharMapping map[string]string `json:"charMapping"` // unicode char → latin-2 replacement
	FieldValues map[string]string `json:"fieldValues"` // persisted TEXT editable field values

	ServicePrices map[string]string `json:"servicePrices"` // service name → net unit price
	RecentFiles   []string          `json:"recentFiles"`   // most-recent-first, capped
	WindowX       int               `json:"windowX"`
	WindowY       int               `json:"windowY"`
	WindowW       int               `json:"windowW"`
	WindowH       int               `json:"windowH"`
}

func defaultSettings() Settings {
	return Settings{
		ColorScheme:   "auto",
		Encoding:      "ISO-8859-2",
		CharMapping:   map[string]string{},
		FieldValues:   map[string]string{},
		ServicePrices: map[string]string{},
		WindowW:       1280,
		WindowH:       800,
	}
}

// settingsStore owns where settings live on disk and how they are (de)serialised.
// It deliberately holds no in-memory Settings value — the Booking facade keeps the
// live copy and hands it to save(); the store is purely the persistence boundary.
type settingsStore struct {
	path string
}

// newSettingsStore resolves the settings file path under the OS user-config dir,
// falling back to the temp dir when that is unavailable (settings then do not
// survive a reboot, but the app still runs).
func newSettingsStore() *settingsStore {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.TempDir()
		log.Printf("settings: user config dir unavailable (%v); falling back to %s — settings will not survive a reboot", err, cfgDir)
	}
	return &settingsStore{path: filepath.Join(cfgDir, "kutyaguru", "settings.json")}
}

// load reads settings from disk, falling back to defaults on any error and
// backfilling fields that must never be zero/nil (window size, the maps).
func (s *settingsStore) load() Settings {
	st := defaultSettings()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return st
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return defaultSettings()
	}
	// Ensure non-zero window size after loading.
	if st.WindowW == 0 {
		st.WindowW = 1280
	}
	if st.WindowH == 0 {
		st.WindowH = 800
	}
	if st.CharMapping == nil {
		st.CharMapping = map[string]string{}
	}
	if st.FieldValues == nil {
		st.FieldValues = map[string]string{}
	}
	if st.ServicePrices == nil {
		st.ServicePrices = map[string]string{}
	}
	return st
}

// save writes data to disk, creating the parent directory as needed. Errors are
// intentionally swallowed: a failed settings write must never break the app.
func (s *settingsStore) save(data Settings) {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = os.WriteFile(s.path, encoded, 0o644)
}
