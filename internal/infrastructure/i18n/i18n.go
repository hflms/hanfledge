package i18n

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Locale represents a language/region code (e.g., "zh-CN", "en-US").
type Locale string

const (
	LocaleZhCN Locale = "zh-CN"
	LocaleEnUS Locale = "en-US"
)

// DefaultLocale is the fallback locale.
const DefaultLocale = LocaleZhCN

// Translator provides message translation for a given locale.
type Translator struct {
	mu       sync.RWMutex
	messages map[Locale]map[string]string // locale -> key -> message
	fallback Locale
}

// NewTranslator creates a new Translator with the given fallback locale.
func NewTranslator(fallback Locale) *Translator {
	return &Translator{
		messages: make(map[Locale]map[string]string),
		fallback: fallback,
	}
}

// LoadDirectory loads all JSON locale files from a directory.
// Each file should be named "{locale}.json" (e.g., "zh-CN.json").
func (t *Translator) LoadDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read locale directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		locale := Locale(strings.TrimSuffix(entry.Name(), ".json"))
		path := filepath.Join(dir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read locale file %s: %w", path, err)
		}

		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("parse locale file %s: %w", path, err)
		}

		t.mu.Lock()
		t.messages[locale] = messages
		t.mu.Unlock()

		log.Printf("🌐 [i18n] Loaded locale: %s (%d messages)", locale, len(messages))
	}

	return nil
}

// T translates a message key for the given locale.
// Falls back to the default locale if the key is not found.
// Returns the key itself if no translation exists.
func (t *Translator) T(locale Locale, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try requested locale
	if msgs, ok := t.messages[locale]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}

	// Fall back to default locale
	if msgs, ok := t.messages[t.fallback]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}

	// Return key as fallback
	return key
}

// TF translates a message with format arguments.
func (t *Translator) TF(locale Locale, key string, args ...interface{}) string {
	template := t.T(locale, key)
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// HasLocale checks if a locale is loaded.
func (t *Translator) HasLocale(locale Locale) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.messages[locale]
	return ok
}

// Locales returns all loaded locales.
func (t *Translator) Locales() []Locale {
	t.mu.RLock()
	defer t.mu.RUnlock()
	locales := make([]Locale, 0, len(t.messages))
	for l := range t.messages {
		locales = append(locales, l)
	}
	return locales
}

// ParseLocale extracts the locale from Accept-Language header or query param.
// Returns the best matching loaded locale, or the fallback.
func (t *Translator) ParseLocale(acceptLang string) Locale {
	// Simple parsing: check for known locales in the header
	acceptLang = strings.ToLower(acceptLang)

	t.mu.RLock()
	defer t.mu.RUnlock()

	for locale := range t.messages {
		if strings.Contains(acceptLang, strings.ToLower(string(locale))) {
			return locale
		}
	}

	// Check language-only matches (e.g., "en" matches "en-US")
	for locale := range t.messages {
		lang := strings.Split(string(locale), "-")[0]
		if strings.Contains(acceptLang, lang) {
			return locale
		}
	}

	return t.fallback
}
