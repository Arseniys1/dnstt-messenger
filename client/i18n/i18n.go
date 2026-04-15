package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

// Manager handles internationalization for the Go client
type Manager struct {
	currentLanguage string
	translations    map[string]string
	fallback        map[string]string
	supportedLangs  []string
}

// NewManager creates a new i18n Manager with English as the default language
func NewManager() *Manager {
	m := &Manager{
		currentLanguage: "en",
		translations:    make(map[string]string),
		fallback:        make(map[string]string),
		supportedLangs:  []string{"en", "zh", "fa", "ru", "ar", "tr", "vi"},
	}
	
	// Load English as fallback
	if err := m.loadLanguageFile("en", &m.fallback); err != nil {
		// If we can't load English, we have a serious problem
		// but we'll continue with empty fallback
		fmt.Fprintf(os.Stderr, "Warning: Failed to load English fallback: %v\n", err)
	}
	
	// Load the default language
	if err := m.LoadLanguage("en"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load default language: %v\n", err)
	}
	
	return m
}

// LoadLanguage loads translations for the specified language code
func (m *Manager) LoadLanguage(code string) error {
	// Validate language code
	if !m.isSupported(code) {
		return fmt.Errorf("unsupported language code: %s", code)
	}
	
	// Load the language file
	translations := make(map[string]string)
	if err := m.loadLanguageFile(code, &translations); err != nil {
		return fmt.Errorf("failed to load language %s: %w", code, err)
	}
	
	m.currentLanguage = code
	m.translations = translations
	
	return nil
}

// loadLanguageFile loads a JSON translation file from embedded FS
func (m *Manager) loadLanguageFile(code string, target *map[string]string) error {
	filename := fmt.Sprintf("locales/%s.json", code)
	
	data, err := localesFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", filename, err)
	}
	
	return nil
}

// T translates a key with optional parameter substitution and pluralization
// Usage: T("error.connection_failed", "error", "timeout")
// For pluralization: T("room.members_count", "count", 5)
// The params should be key-value pairs: key1, value1, key2, value2, ...
func (m *Manager) T(key string, params ...interface{}) string {
	// Parse parameters into a map
	paramMap := make(map[string]interface{})
	for i := 0; i < len(params)-1; i += 2 {
		paramKey, ok := params[i].(string)
		if !ok {
			continue
		}
		paramMap[paramKey] = params[i+1]
	}
	
	// Check if this is a pluralization request (params contains 'count')
	if count, hasCount := paramMap["count"]; hasCount {
		countNum, ok := toInt(count)
		if ok {
			pluralKey := m.getPluralKey(key, countNum)
			text := m.getTranslation(pluralKey)
			
			// If plural form not found, try the base key
			if text == key || text == pluralKey {
				text = m.getTranslation(key)
			}
			
			// Substitute parameters
			return m.substituteParams(text, paramMap)
		}
	}
	
	// Get the translation text
	text := m.getTranslation(key)
	
	// If no parameters, return as-is
	if len(paramMap) == 0 {
		return text
	}
	
	// Substitute parameters
	return m.substituteParams(text, paramMap)
}

// substituteParams replaces placeholders in text with parameter values
func (m *Manager) substituteParams(text string, params map[string]interface{}) string {
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		text = strings.ReplaceAll(text, placeholder, fmt.Sprintf("%v", value))
	}
	return text
}

// toInt converts an interface{} to int if possible
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	default:
		return 0, false
	}
}

// getPluralKey returns the appropriate plural key based on count and language rules
func (m *Manager) getPluralKey(baseKey string, count int) string {
	pluralForm := m.getPluralForm(count)
	
	// Try language-specific plural forms: key.zero, key.one, key.many
	switch pluralForm {
	case "zero":
		return fmt.Sprintf("%s.zero", baseKey)
	case "one":
		return fmt.Sprintf("%s.one", baseKey)
	default:
		return fmt.Sprintf("%s.many", baseKey)
	}
}

// getPluralForm determines plural form based on count and current language rules
func (m *Manager) getPluralForm(count int) string {
	n := count
	if n < 0 {
		n = -n
	}
	
	// Language-specific plural rules
	switch m.currentLanguage {
	case "en", "vi", "tr":
		// English, Vietnamese, Turkish: one (n=1), many (n!=1)
		if n == 1 {
			return "one"
		}
		return "many"
	
	case "zh":
		// Chinese: no plural distinction, always use 'many'
		return "many"
	
	case "ru":
		// Russian: complex rules
		// one: n%10=1 and n%100!=11
		// few: n%10 in 2..4 and n%100 not in 12..14
		// many: otherwise
		if n%10 == 1 && n%100 != 11 {
			return "one"
		}
		return "many"
	
	case "ar":
		// Arabic: complex rules
		// zero: n=0
		// one: n=1
		// two: n=2
		// few: n%100 in 3..10
		// many: n%100 in 11..99
		// other: otherwise
		if n == 0 {
			return "zero"
		}
		if n == 1 {
			return "one"
		}
		return "many"
	
	case "fa":
		// Farsi: one (n=0 or n=1), many (n>1)
		if n == 0 || n == 1 {
			return "one"
		}
		return "many"
	
	default:
		// Default to English rules
		if n == 1 {
			return "one"
		}
		return "many"
	}
}

// getTranslation retrieves a translation with fallback logic
func (m *Manager) getTranslation(key string) string {
	// Try current language
	if text, ok := m.translations[key]; ok {
		return text
	}
	
	// Try fallback (English)
	if text, ok := m.fallback[key]; ok {
		return text
	}
	
	// Return the key itself as last resort
	return key
}

// DetectLanguage detects the system language with priority:
// 1. Command-line flag (--lang)
// 2. Config file (client_config.json)
// 3. Environment variable (LANG)
// 4. Default to "en"
func (m *Manager) DetectLanguage(flagLang, configPath string) string {
	// Priority 1: Command-line flag
	if flagLang != "" {
		code := strings.ToLower(strings.TrimSpace(flagLang))
		if m.isSupported(code) {
			return code
		}
		fmt.Fprintf(os.Stderr, "Warning: Unsupported language flag '%s', falling back\n", flagLang)
	}
	
	// Priority 2: Config file
	if configPath != "" {
		if code := m.detectFromConfig(configPath); code != "" {
			if m.isSupported(code) {
				return code
			}
			fmt.Fprintf(os.Stderr, "Warning: Unsupported language in config '%s', falling back\n", code)
		}
	}
	
	// Priority 3: LANG environment variable
	lang := os.Getenv("LANG")
	if lang != "" {
		// Extract language code from LANG (e.g., "zh_CN.UTF-8" -> "zh")
		code := extractLanguageCode(lang)
		if m.isSupported(code) {
			return code
		}
	}
	
	// Priority 4: Default to English
	return "en"
}

// detectFromConfig reads the language preference from client_config.json
func (m *Manager) detectFromConfig(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}
	
	if lang, ok := config["language"].(string); ok {
		return strings.ToLower(strings.TrimSpace(lang))
	}
	
	return ""
}

// extractLanguageCode extracts the language code from LANG environment variable
// Examples: "zh_CN.UTF-8" -> "zh", "en_US" -> "en", "fa" -> "fa"
func extractLanguageCode(lang string) string {
	// Split by underscore or dot
	parts := strings.FieldsFunc(lang, func(r rune) bool {
		return r == '_' || r == '.' || r == '-'
	})
	
	if len(parts) > 0 {
		return strings.ToLower(parts[0])
	}
	
	return strings.ToLower(lang)
}

// SetLanguage changes the current language
func (m *Manager) SetLanguage(code string) error {
	return m.LoadLanguage(code)
}

// GetCurrentLanguage returns the currently active language code
func (m *Manager) GetCurrentLanguage() string {
	return m.currentLanguage
}

// GetSupportedLanguages returns a list of all supported language codes
func (m *Manager) GetSupportedLanguages() []string {
	return m.supportedLangs
}

// isSupported checks if a language code is supported
func (m *Manager) isSupported(code string) bool {
	for _, lang := range m.supportedLangs {
		if lang == code {
			return true
		}
	}
	return false
}

// SaveLanguageToConfig saves the language preference to client_config.json
func (m *Manager) SaveLanguageToConfig(configPath, languageCode string) error {
	// Read existing config
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, create new config
		config = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}
	
	// Update language field
	config["language"] = languageCode
	
	// Write back to file
	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}
