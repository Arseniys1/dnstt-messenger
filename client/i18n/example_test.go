package i18n_test

import (
	"fmt"
	"dnstt-messenger/client/i18n"
)

// Example demonstrates basic usage of the i18n Manager
func Example() {
	// Create a new i18n manager
	m := i18n.NewManager()
	
	// Translate a simple key
	fmt.Println(m.T("app.name"))
	
	// Translate with parameters
	fmt.Println(m.T("error.connection_failed", "error", "timeout"))
	
	// Switch to Chinese
	m.SetLanguage("zh")
	fmt.Println(m.T("login.title"))
	
	// Output:
	// DNSTT Messenger
	// Connection error: timeout
	// 登录
}

// ExampleManager_DetectLanguage demonstrates language detection
func ExampleManager_DetectLanguage() {
	m := i18n.NewManager()
	
	// Detect system language from LANG environment variable
	// Pass empty strings for flag and config path to use environment detection
	detected := m.DetectLanguage("", "")
	fmt.Printf("Detected language: %s\n", detected)
	
	// Load the detected language
	if err := m.LoadLanguage(detected); err != nil {
		fmt.Printf("Failed to load language: %v\n", err)
	}
}

// ExampleManager_T demonstrates translation with multiple parameters
func ExampleManager_T() {
	m := i18n.NewManager()
	
	// Simple translation
	fmt.Println(m.T("login.username"))
	
	// Translation with pluralization (count parameter triggers plural form)
	fmt.Println(m.T("sidebar.online_count", "count", 5))
	
	// Translation with multiple parameters
	fmt.Println(m.T("status.mode_proxy", 
		"proxy", "127.0.0.1:18000",
		"server", "example.com:9999"))
	
	// Output:
	// Username
	// Online (5 users)
	// Mode: DNSTT Proxy (SOCKS5) | Proxy: 127.0.0.1:18000 -> Server: example.com:9999...
}

// ExampleManager_GetSupportedLanguages demonstrates listing supported languages
func ExampleManager_GetSupportedLanguages() {
	m := i18n.NewManager()
	
	langs := m.GetSupportedLanguages()
	fmt.Printf("Supported languages: %v\n", langs)
	
	// Output:
	// Supported languages: [en zh fa ru ar tr vi]
}
