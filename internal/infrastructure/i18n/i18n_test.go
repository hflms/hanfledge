package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ============================
// i18n Translator Unit Tests
// ============================

// -- Constructor Tests ----------------------------------------

func TestNewTranslator(t *testing.T) {
	tr := NewTranslator(LocaleZhCN)
	if tr == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if tr.fallback != LocaleZhCN {
		t.Errorf("fallback = %q, want %q", tr.fallback, LocaleZhCN)
	}
}

// -- LoadDirectory Tests --------------------------------------

func TestTranslator_LoadDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create locale files
	zhMsgs := map[string]string{
		"greeting":   "你好",
		"error.auth": "认证失败",
	}
	enMsgs := map[string]string{
		"greeting":   "Hello",
		"error.auth": "Authentication failed",
	}

	writeLocaleFile(t, tmpDir, "zh-CN.json", zhMsgs)
	writeLocaleFile(t, tmpDir, "en-US.json", enMsgs)

	tr := NewTranslator(LocaleZhCN)
	err := tr.LoadDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadDirectory() error: %v", err)
	}

	if !tr.HasLocale(LocaleZhCN) {
		t.Error("zh-CN should be loaded")
	}
	if !tr.HasLocale(LocaleEnUS) {
		t.Error("en-US should be loaded")
	}
}

func TestTranslator_LoadDirectory_SkipsNonJSON(t *testing.T) {
	tmpDir := t.TempDir()

	writeLocaleFile(t, tmpDir, "zh-CN.json", map[string]string{"key": "val"})
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# readme"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)

	tr := NewTranslator(LocaleZhCN)
	err := tr.LoadDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadDirectory() error: %v", err)
	}

	locales := tr.Locales()
	if len(locales) != 1 {
		t.Errorf("Locales() = %d, want 1 (only zh-CN.json)", len(locales))
	}
}

func TestTranslator_LoadDirectory_NonexistentDir(t *testing.T) {
	tr := NewTranslator(LocaleZhCN)
	err := tr.LoadDirectory("/tmp/nonexistent_locale_dir_" + t.Name())
	if err == nil {
		t.Error("LoadDirectory() should return error for nonexistent dir")
	}
}

func TestTranslator_LoadDirectory_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte("not json"), 0644)

	tr := NewTranslator(LocaleZhCN)
	err := tr.LoadDirectory(tmpDir)
	if err == nil {
		t.Error("LoadDirectory() should return error for invalid JSON")
	}
}

// -- T (Translate) Tests --------------------------------------

func TestTranslator_T_ExactLocale(t *testing.T) {
	tr := setupTranslator(t)

	msg := tr.T(LocaleZhCN, "greeting")
	if msg != "你好" {
		t.Errorf("T(zh-CN, greeting) = %q, want %q", msg, "你好")
	}

	msg = tr.T(LocaleEnUS, "greeting")
	if msg != "Hello" {
		t.Errorf("T(en-US, greeting) = %q, want %q", msg, "Hello")
	}
}

func TestTranslator_T_FallbackToDefault(t *testing.T) {
	tr := setupTranslator(t)

	// "only_zh" only exists in zh-CN
	msg := tr.T(LocaleEnUS, "only_zh")
	if msg != "仅中文" {
		t.Errorf("T(en-US, only_zh) should fall back to zh-CN, got %q", msg)
	}
}

func TestTranslator_T_KeyNotFound(t *testing.T) {
	tr := setupTranslator(t)

	// Key not in any locale
	msg := tr.T(LocaleZhCN, "nonexistent.key")
	if msg != "nonexistent.key" {
		t.Errorf("T() should return key itself for missing translations, got %q", msg)
	}
}

func TestTranslator_T_UnknownLocale(t *testing.T) {
	tr := setupTranslator(t)

	// Unknown locale should fall back to zh-CN
	msg := tr.T(Locale("fr-FR"), "greeting")
	if msg != "你好" {
		t.Errorf("T(fr-FR, greeting) should fall back to zh-CN, got %q", msg)
	}
}

// -- TF (Translate with Format) Tests -------------------------

func TestTranslator_TF(t *testing.T) {
	tr := NewTranslator(LocaleZhCN)
	tr.mu.Lock()
	tr.messages[LocaleZhCN] = map[string]string{
		"welcome": "欢迎 %s, 你有 %d 条消息",
	}
	tr.mu.Unlock()

	msg := tr.TF(LocaleZhCN, "welcome", "张三", 5)
	expected := "欢迎 张三, 你有 5 条消息"
	if msg != expected {
		t.Errorf("TF() = %q, want %q", msg, expected)
	}
}

func TestTranslator_TF_NoArgs(t *testing.T) {
	tr := NewTranslator(LocaleZhCN)
	tr.mu.Lock()
	tr.messages[LocaleZhCN] = map[string]string{
		"simple": "简单消息",
	}
	tr.mu.Unlock()

	msg := tr.TF(LocaleZhCN, "simple")
	if msg != "简单消息" {
		t.Errorf("TF() with no args = %q, want %q", msg, "简单消息")
	}
}

// -- HasLocale Tests ------------------------------------------

func TestTranslator_HasLocale(t *testing.T) {
	tr := setupTranslator(t)

	if !tr.HasLocale(LocaleZhCN) {
		t.Error("HasLocale(zh-CN) should return true")
	}
	if tr.HasLocale(Locale("ja-JP")) {
		t.Error("HasLocale(ja-JP) should return false")
	}
}

// -- Locales Tests --------------------------------------------

func TestTranslator_Locales(t *testing.T) {
	tr := setupTranslator(t)

	locales := tr.Locales()
	if len(locales) != 2 {
		t.Errorf("Locales() length = %d, want 2", len(locales))
	}

	hasZh, hasEn := false, false
	for _, l := range locales {
		if l == LocaleZhCN {
			hasZh = true
		}
		if l == LocaleEnUS {
			hasEn = true
		}
	}
	if !hasZh || !hasEn {
		t.Errorf("Locales() = %v, want both zh-CN and en-US", locales)
	}
}

// -- ParseLocale Tests ----------------------------------------

func TestTranslator_ParseLocale_ExactMatch(t *testing.T) {
	tr := setupTranslator(t)

	locale := tr.ParseLocale("en-US,en;q=0.9")
	if locale != LocaleEnUS {
		t.Errorf("ParseLocale('en-US,...') = %q, want %q", locale, LocaleEnUS)
	}
}

func TestTranslator_ParseLocale_LanguageOnly(t *testing.T) {
	tr := setupTranslator(t)

	locale := tr.ParseLocale("en;q=0.9")
	if locale != LocaleEnUS {
		t.Errorf("ParseLocale('en') = %q, want %q", locale, LocaleEnUS)
	}
}

func TestTranslator_ParseLocale_Fallback(t *testing.T) {
	tr := setupTranslator(t)

	locale := tr.ParseLocale("ja-JP")
	if locale != LocaleZhCN {
		t.Errorf("ParseLocale('ja-JP') should fall back to %q, got %q", LocaleZhCN, locale)
	}
}

func TestTranslator_ParseLocale_Empty(t *testing.T) {
	tr := setupTranslator(t)

	locale := tr.ParseLocale("")
	if locale != LocaleZhCN {
		t.Errorf("ParseLocale('') should fall back to %q, got %q", LocaleZhCN, locale)
	}
}

// -- Helpers --------------------------------------------------

func setupTranslator(t *testing.T) *Translator {
	t.Helper()
	tmpDir := t.TempDir()

	writeLocaleFile(t, tmpDir, "zh-CN.json", map[string]string{
		"greeting": "你好",
		"only_zh":  "仅中文",
	})
	writeLocaleFile(t, tmpDir, "en-US.json", map[string]string{
		"greeting": "Hello",
	})

	tr := NewTranslator(LocaleZhCN)
	if err := tr.LoadDirectory(tmpDir); err != nil {
		t.Fatalf("setupTranslator LoadDirectory failed: %v", err)
	}
	return tr
}

func writeLocaleFile(t *testing.T, dir, filename string, msgs map[string]string) {
	t.Helper()
	data, err := json.Marshal(msgs)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
