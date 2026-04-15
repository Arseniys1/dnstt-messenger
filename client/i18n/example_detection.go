// +build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"dnstt-messenger/client/i18n"
)

func main() {
	// Define command-line flags
	langFlag := flag.String("lang", "", "Language code (en, zh, fa, ru, ar, tr, vi)")
	configPath := flag.String("config", "client_config.json", "Path to config file")
	flag.Parse()

	// Create i18n manager
	m := i18n.NewManager()

	// Detect language with priority: flag > config > env > default
	detected := m.DetectLanguage(*langFlag, *configPath)
	fmt.Printf("Detected language: %s\n", detected)

	// Load the detected language
	if err := m.LoadLanguage(detected); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load language: %v\n", err)
		os.Exit(1)
	}

	// Display some translated text
	fmt.Println("\n=== Translations ===")
	fmt.Printf("App Name: %s\n", m.T("app.name"))
	fmt.Printf("Login Title: %s\n", m.T("login.title"))
	fmt.Printf("Username: %s\n", m.T("login.username"))
	fmt.Printf("Password: %s\n", m.T("login.password"))
	fmt.Printf("Submit: %s\n", m.T("login.button.submit"))

	// Display online count with parameter
	fmt.Printf("\nOnline Count: %s\n", m.T("sidebar.online_count", "count", 42))

	// Save language preference to config if flag was used
	if *langFlag != "" {
		if err := m.SaveLanguageToConfig(*configPath, detected); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save language preference: %v\n", err)
		} else {
			fmt.Printf("\nLanguage preference '%s' saved to %s\n", detected, *configPath)
		}
	}
}
