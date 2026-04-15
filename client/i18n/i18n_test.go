package i18n

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if m.currentLanguage != "en" {
		t.Errorf("Expected default language 'en', got '%s'", m.currentLanguage)
	}
	
	if len(m.translations) == 0 {
		t.Error("Expected translations to be loaded, got empty map")
	}
	
	if len(m.fallback) == 0 {
		t.Error("Expected fallback translations to be loaded, got empty map")
	}
}

func TestLoadLanguage(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"Load English", "en", false},
		{"Load Chinese", "zh", false},
		{"Load Farsi", "fa", false},
		{"Load Russian", "ru", false},
		{"Load Arabic", "ar", false},
		{"Load Turkish", "tr", false},
		{"Load Vietnamese", "vi", false},
		{"Load unsupported", "de", true},
		{"Load invalid", "invalid", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.LoadLanguage(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadLanguage(%s) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
			
			if !tt.wantErr && m.currentLanguage != tt.code {
				t.Errorf("Expected current language '%s', got '%s'", tt.code, m.currentLanguage)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	m := NewManager()
	
	// Test basic translation
	text := m.T("app.name")
	if text == "" {
		t.Error("Expected non-empty translation for 'app.name'")
	}
	
	// Test translation with known key
	loginTitle := m.T("login.title")
	if loginTitle != "Login" {
		t.Errorf("Expected 'Login', got '%s'", loginTitle)
	}
}

func TestTranslateWithParams(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		name     string
		key      string
		params   []interface{}
		expected string
	}{
		{
			name:     "Single parameter",
			key:      "error.connection_failed",
			params:   []interface{}{"error", "timeout"},
			expected: "Connection error: timeout",
		},
		{
			name:     "Multiple parameters",
			key:      "status.mode_proxy",
			params:   []interface{}{"proxy", "127.0.0.1:18000", "server", "example.com:9999"},
			expected: "Mode: DNSTT Proxy (SOCKS5) | Proxy: 127.0.0.1:18000 -> Server: example.com:9999...",
		},
		{
			name:     "Count parameter with pluralization",
			key:      "sidebar.online_count",
			params:   []interface{}{"count", 42},
			expected: "Online (42 users)",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.T(tt.key, tt.params...)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTranslateMissingKey(t *testing.T) {
	m := NewManager()
	
	// Test with a key that doesn't exist
	missingKey := "this.key.does.not.exist"
	result := m.T(missingKey)
	
	// Should return the key itself as fallback
	if result != missingKey {
		t.Errorf("Expected missing key to return itself, got '%s'", result)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	m := NewManager()
	
	// Load a non-English language
	if err := m.LoadLanguage("zh"); err != nil {
		t.Fatalf("Failed to load Chinese: %v", err)
	}
	
	// All keys should still be accessible (either from Chinese or fallback to English)
	text := m.T("app.name")
	if text == "" {
		t.Error("Expected non-empty translation with fallback")
	}
}

func TestDetectLanguage(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		name     string
		langEnv  string
		expected string
	}{
		{"English US", "en_US.UTF-8", "en"},
		{"Chinese CN", "zh_CN.UTF-8", "zh"},
		{"Farsi", "fa_IR.UTF-8", "fa"},
		{"Russian", "ru_RU.UTF-8", "ru"},
		{"Arabic", "ar_SA.UTF-8", "ar"},
		{"Turkish", "tr_TR.UTF-8", "tr"},
		{"Vietnamese", "vi_VN.UTF-8", "vi"},
		{"Simple format", "zh", "zh"},
		{"Unsupported language", "de_DE.UTF-8", "en"},
		{"Empty LANG", "", "en"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set LANG environment variable
			oldLang := os.Getenv("LANG")
			defer os.Setenv("LANG", oldLang)
			
			os.Setenv("LANG", tt.langEnv)
			
			detected := m.DetectLanguage("", "")
			if detected != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, detected)
			}
		})
	}
}

func TestDetectLanguageWithFlag(t *testing.T) {
	m := NewManager()
	
	// Set LANG environment variable
	oldLang := os.Getenv("LANG")
	defer os.Setenv("LANG", oldLang)
	os.Setenv("LANG", "en_US.UTF-8")
	
	tests := []struct {
		name     string
		flagLang string
		expected string
	}{
		{"Valid flag overrides env", "zh", "zh"},
		{"Invalid flag falls back to env", "de", "en"},
		{"Empty flag uses env", "", "en"},
		{"Whitespace flag uses env", "  ", "en"},
		{"Case insensitive flag", "ZH", "zh"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := m.DetectLanguage(tt.flagLang, "")
			if detected != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, detected)
			}
		})
	}
}

func TestDetectLanguageFromConfig(t *testing.T) {
	m := NewManager()
	
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Write config with language preference
	configContent := `{
		"proxy_addr": "127.0.0.1:18000",
		"server_addr": "127.0.0.1:9999",
		"language": "zh"
	}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()
	
	// Set LANG environment variable
	oldLang := os.Getenv("LANG")
	defer os.Setenv("LANG", oldLang)
	os.Setenv("LANG", "en_US.UTF-8")
	
	// Config should override env
	detected := m.DetectLanguage("", tmpFile.Name())
	if detected != "zh" {
		t.Errorf("Expected 'zh' from config, got '%s'", detected)
	}
}

func TestDetectLanguagePriority(t *testing.T) {
	m := NewManager()
	
	// Create a temporary config file with Russian
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	configContent := `{
		"language": "ru"
	}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()
	
	// Set LANG environment variable to Arabic
	oldLang := os.Getenv("LANG")
	defer os.Setenv("LANG", oldLang)
	os.Setenv("LANG", "ar_SA.UTF-8")
	
	tests := []struct {
		name       string
		flagLang   string
		configPath string
		expected   string
	}{
		{"Flag overrides all", "zh", tmpFile.Name(), "zh"},
		{"Config overrides env", "", tmpFile.Name(), "ru"},
		{"Env used when no flag or config", "", "", "ar"},
		{"Invalid flag falls to config", "de", tmpFile.Name(), "ru"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := m.DetectLanguage(tt.flagLang, tt.configPath)
			if detected != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, detected)
			}
		})
	}
}

func TestSaveLanguageToConfig(t *testing.T) {
	m := NewManager()
	
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Write initial config
	initialConfig := `{
		"proxy_addr": "127.0.0.1:18000",
		"server_addr": "127.0.0.1:9999"
	}`
	if _, err := tmpFile.WriteString(initialConfig); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()
	
	// Save language preference
	if err := m.SaveLanguageToConfig(tmpFile.Name(), "zh"); err != nil {
		t.Fatalf("Failed to save language: %v", err)
	}
	
	// Read back and verify
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
	
	if lang, ok := config["language"].(string); !ok || lang != "zh" {
		t.Errorf("Expected language 'zh', got '%v'", config["language"])
	}
	
	// Verify other fields are preserved
	if proxy, ok := config["proxy_addr"].(string); !ok || proxy != "127.0.0.1:18000" {
		t.Errorf("Expected proxy_addr to be preserved, got '%v'", config["proxy_addr"])
	}
}

func TestSaveLanguageToConfigNewFile(t *testing.T) {
	m := NewManager()
	
	// Use a non-existent file path
	tmpFile := os.TempDir() + "/test_new_config.json"
	defer os.Remove(tmpFile)
	
	// Save language preference to new file
	if err := m.SaveLanguageToConfig(tmpFile, "fa"); err != nil {
		t.Fatalf("Failed to save language to new file: %v", err)
	}
	
	// Read back and verify
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
	
	if lang, ok := config["language"].(string); !ok || lang != "fa" {
		t.Errorf("Expected language 'fa', got '%v'", config["language"])
	}
}

func TestExtractLanguageCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en_US.UTF-8", "en"},
		{"zh_CN.UTF-8", "zh"},
		{"fa_IR", "fa"},
		{"ru", "ru"},
		{"ar-SA", "ar"},
		{"tr_TR.ISO-8859-9", "tr"},
		{"EN_US", "en"},
		{"", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractLanguageCode(tt.input)
			if result != tt.expected {
				t.Errorf("extractLanguageCode(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSetLanguage(t *testing.T) {
	m := NewManager()
	
	// Set to Chinese
	if err := m.SetLanguage("zh"); err != nil {
		t.Fatalf("Failed to set language to Chinese: %v", err)
	}
	
	if m.GetCurrentLanguage() != "zh" {
		t.Errorf("Expected current language 'zh', got '%s'", m.GetCurrentLanguage())
	}
	
	// Set to invalid language
	if err := m.SetLanguage("invalid"); err == nil {
		t.Error("Expected error when setting invalid language, got nil")
	}
	
	// Current language should remain unchanged after failed set
	if m.GetCurrentLanguage() != "zh" {
		t.Errorf("Expected current language to remain 'zh', got '%s'", m.GetCurrentLanguage())
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	m := NewManager()
	
	langs := m.GetSupportedLanguages()
	
	if len(langs) != 7 {
		t.Errorf("Expected 7 supported languages, got %d", len(langs))
	}
	
	expectedLangs := []string{"en", "zh", "fa", "ru", "ar", "tr", "vi"}
	for _, expected := range expectedLangs {
		found := false
		for _, lang := range langs {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language '%s' to be in supported languages", expected)
		}
	}
}

func TestIsSupported(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		code     string
		expected bool
	}{
		{"en", true},
		{"zh", true},
		{"fa", true},
		{"ru", true},
		{"ar", true},
		{"tr", true},
		{"vi", true},
		{"de", false},
		{"fr", false},
		{"invalid", false},
		{"", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := m.isSupported(tt.code)
			if result != tt.expected {
				t.Errorf("isSupported(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

func TestAllLanguagesHaveSameKeys(t *testing.T) {
	m := NewManager()
	
	// Load English to get the base keys
	if err := m.LoadLanguage("en"); err != nil {
		t.Fatalf("Failed to load English: %v", err)
	}
	
	englishKeys := make(map[string]bool)
	for key := range m.translations {
		englishKeys[key] = true
	}
	
	// Check each other language
	languages := []string{"zh", "fa", "ru", "ar", "tr", "vi"}
	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			translations := make(map[string]string)
			if err := m.loadLanguageFile(lang, &translations); err != nil {
				t.Fatalf("Failed to load %s: %v", lang, err)
			}
			
			// Check for missing keys
			missingKeys := []string{}
			for key := range englishKeys {
				if _, ok := translations[key]; !ok {
					missingKeys = append(missingKeys, key)
				}
			}
			
			if len(missingKeys) > 0 {
				t.Logf("Warning: %s is missing %d keys (%.1f%% coverage)", 
					lang, len(missingKeys), 
					float64(len(translations))/float64(len(englishKeys))*100)
			}
			
			// We allow some missing keys (95% coverage requirement)
			coverage := float64(len(translations)) / float64(len(englishKeys)) * 100
			if coverage < 95.0 {
				t.Errorf("%s has only %.1f%% coverage (expected >= 95%%)", lang, coverage)
			}
		})
	}
}

func TestEmbeddedFilesExist(t *testing.T) {
	languages := []string{"en", "zh", "fa", "ru", "ar", "tr", "vi"}
	
	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			filename := "locales/" + lang + ".json"
			data, err := localesFS.ReadFile(filename)
			if err != nil {
				t.Errorf("Failed to read embedded file %s: %v", filename, err)
			}
			if len(data) == 0 {
				t.Errorf("Embedded file %s is empty", filename)
			}
		})
	}
}

// Pluralization tests
func TestPluralizationZeroForm(t *testing.T) {
	m := NewManager()
	
	result := m.T("room.list_title", "count", 0)
	expected := "Rooms (0)"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestPluralizationOneForm(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		key      string
		expected string
	}{
		{"sidebar.online_count", "Online (1 user)"},
		{"room.members_count", "1 member"},
	}
	
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := m.T(tt.key, "count", 1)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestPluralizationManyForm(t *testing.T) {
	m := NewManager()
	
	tests := []struct {
		key      string
		count    int
		expected string
	}{
		{"sidebar.online_count", 5, "Online (5 users)"},
		{"room.members_count", 10, "10 members"},
		{"room.list_title", 3, "Rooms (3)"},
	}
	
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := m.T(tt.key, "count", tt.count)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestPluralizationFallbackToBaseKey(t *testing.T) {
	m := NewManager()
	
	// Key without plural forms should fall back to base key
	result := m.T("app.name", "count", 5)
	expected := "DNSTT Messenger"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetPluralFormEnglish(t *testing.T) {
	m := NewManager()
	m.LoadLanguage("en")
	
	tests := []struct {
		count    int
		expected string
	}{
		{0, "many"},
		{1, "one"},
		{2, "many"},
		{100, "many"},
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			result := m.getPluralForm(tt.count)
			if result != tt.expected {
				t.Errorf("English: count=%d, expected '%s', got '%s'", tt.count, tt.expected, result)
			}
		})
	}
}

func TestGetPluralFormRussian(t *testing.T) {
	m := NewManager()
	m.LoadLanguage("ru")
	
	tests := []struct {
		count    int
		expected string
	}{
		{1, "one"},
		{2, "many"},
		{5, "many"},
		{11, "many"},
		{21, "one"},
		{101, "one"},
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			result := m.getPluralForm(tt.count)
			if result != tt.expected {
				t.Errorf("Russian: count=%d, expected '%s', got '%s'", tt.count, tt.expected, result)
			}
		})
	}
}

func TestGetPluralFormArabic(t *testing.T) {
	m := NewManager()
	m.LoadLanguage("ar")
	
	tests := []struct {
		count    int
		expected string
	}{
		{0, "zero"},
		{1, "one"},
		{2, "many"},
		{5, "many"},
		{100, "many"},
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			result := m.getPluralForm(tt.count)
			if result != tt.expected {
				t.Errorf("Arabic: count=%d, expected '%s', got '%s'", tt.count, tt.expected, result)
			}
		})
	}
}

func TestGetPluralFormFarsi(t *testing.T) {
	m := NewManager()
	m.LoadLanguage("fa")
	
	tests := []struct {
		count    int
		expected string
	}{
		{0, "one"},
		{1, "one"},
		{2, "many"},
		{10, "many"},
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			result := m.getPluralForm(tt.count)
			if result != tt.expected {
				t.Errorf("Farsi: count=%d, expected '%s', got '%s'", tt.count, tt.expected, result)
			}
		})
	}
}

func TestGetPluralFormChinese(t *testing.T) {
	m := NewManager()
	m.LoadLanguage("zh")
	
	// Chinese doesn't distinguish plurals, always uses 'many'
	tests := []int{0, 1, 2, 5, 100}
	
	for _, count := range tests {
		t.Run(string(rune(count)), func(t *testing.T) {
			result := m.getPluralForm(count)
			if result != "many" {
				t.Errorf("Chinese: count=%d, expected 'many', got '%s'", count, result)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
		ok       bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(42), 42, true},
		{"int32", int32(42), 42, true},
		{"float64", float64(42.7), 42, true},
		{"float32", float32(42.3), 42, true},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toInt(tt.input)
			if ok != tt.ok {
				t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("toInt(%v) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPluralizationWithMultipleParams(t *testing.T) {
	m := NewManager()
	
	// Test pluralization with additional parameters
	result := m.T("misc.online_users", "count", 5, "users", "Alice, Bob, Charlie, Dave, Eve")
	expected := "Online (5): Alice, Bob, Charlie, Dave, Eve"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestPluralizationNegativeCount(t *testing.T) {
	m := NewManager()
	
	// Negative counts should use absolute value for plural form determination
	result := m.T("sidebar.online_count", "count", -5)
	expected := "Online (-5 users)"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
