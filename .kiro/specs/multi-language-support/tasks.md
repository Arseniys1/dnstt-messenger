# Implementation Plan: Multi-Language Support

## Overview

This implementation adds internationalization (i18n) support to the DNSTT Messenger application across three client platforms: Android (Kotlin), Electron (JavaScript), and Go CLI. The system will support seven languages: English (default), Chinese, Farsi, Russian, Arabic, Turkish, and Vietnamese. Each platform uses its native i18n approach: Android uses the resource system (strings.xml), Electron uses JSON-based translations with dynamic loading, and Go embeds JSON translations in the binary.

## Tasks

- [x] 1. Create translation key inventory and base English translations
  - Audit all three clients to identify hardcoded text strings
  - Create a shared translation key namespace document
  - Create base English translation file with all identified keys
  - _Requirements: 2.1, 2.2, 6.1_

- [x] 2. Implement Android client internationalization
  - [x] 2.1 Create Android resource structure and English strings
    - Create `res/values/strings.xml` with all translation keys
    - Replace hardcoded strings in Kotlin files with string resource references
    - Update MainActivity.kt, MessengerViewModel.kt, and Composable UI components
    - _Requirements: 3.1, 3.4_
  
  - [x] 2.2 Create LocaleManager for Android
    - Implement `LocaleManager` object in Kotlin with getCurrentLocale, setLocale, and getSupportedLanguages methods
    - Add SharedPreferences storage for language preference
    - Implement locale detection from Android system settings
    - _Requirements: 1.1, 1.2, 1.3, 1.5_
  
  - [x] 2.3 Add language selection UI to Android app
    - Create language settings screen in Jetpack Compose
    - Add language picker with native names for all seven languages
    - Wire language selection to LocaleManager
    - _Requirements: 1.4, 1.5_
  
  - [x] 2.4 Write unit tests for Android LocaleManager
    - Test locale detection, switching, and persistence
    - Test fallback to English for unsupported locales
    - _Requirements: 9.5_

- [x] 3. Create translation files for all supported languages
  - [x] 3.1 Create Android strings.xml for six additional languages
    - Create `values-zh/strings.xml` (Chinese)
    - Create `values-fa/strings.xml` (Farsi)
    - Create `values-ru/strings.xml` (Russian)
    - Create `values-ar/strings.xml` (Arabic)
    - Create `values-tr/strings.xml` (Turkish)
    - Create `values-vi/strings.xml` (Vietnamese)
    - _Requirements: 1.6, 3.2, 6.2_
  
  - [x] 3.2 Create JSON translation files for Electron and Go
    - Create `electron-client/i18n/locales/en.json` with all keys
    - Create translation files for zh, fa, ru, ar, tr, vi in same directory
    - Create `client/i18n/locales/en.json` with all keys
    - Create translation files for zh, fa, ru, ar, tr, vi in same directory
    - _Requirements: 2.1, 2.2, 6.2_
  
  - [x] 3.3 Create translation validation script
    - Write Node.js script to validate translation file completeness
    - Check for missing keys, extra keys, and coverage percentage
    - Add script to package.json or Makefile
    - _Requirements: 6.4, 9.1, 9.2, 9.3, 9.4_

- [x] 4. Implement Electron client internationalization
  - [x] 4.1 Create I18n Manager for Electron
    - Create `electron-client/i18n/manager.js` with I18nManager class
    - Implement loadLanguage, translate, detectSystemLanguage, setLanguage methods
    - Add fallback logic for missing keys and files
    - _Requirements: 2.3, 2.4, 2.5, 4.1, 4.2, 8.1, 8.2_
  
  - [x] 4.2 Integrate I18n Manager into Electron renderer
    - Import I18nManager in renderer/app.js
    - Replace all hardcoded strings with translate() calls
    - Update index.html to use translation keys
    - _Requirements: 4.3_
  
  - [x] 4.3 Add RTL support for Arabic and Farsi
    - Implement setTextDirection function to toggle dir attribute
    - Add CSS styles for RTL layout
    - Test UI layout with Arabic and Farsi
    - _Requirements: 4.6_
  
  - [x] 4.4 Add language selector to Electron settings
    - Create language dropdown in settings tab
    - Wire dropdown to I18nManager.setLanguage
    - Implement dynamic UI update without restart
    - _Requirements: 1.4, 4.4, 4.5_
  
  - [x] 4.5 Add language preference persistence for Electron
    - Store selected language in config.json
    - Load language preference on app startup
    - _Requirements: 1.5_
  
  - [x] 4.6 Write unit tests for Electron I18nManager
    - Test translation loading, key lookup, and fallback behavior
    - Test parameter substitution and pluralization
    - Test RTL detection and system language detection
    - _Requirements: 9.5_

- [x] 5. Checkpoint - Test Android and Electron clients
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement Go client internationalization
  - [x] 6.1 Create i18n package for Go client
    - Create `client/i18n/i18n.go` with Manager struct
    - Implement NewManager, LoadLanguage, T (translate), DetectLanguage, SetLanguage methods
    - Add embed directive for translation files
    - _Requirements: 5.1, 2.6, 8.1_
  
  - [x] 6.2 Implement language detection for Go client
    - Parse LANG environment variable to extract language code
    - Check client_config.json for saved language preference
    - Add --lang command-line flag support
    - Implement detection priority: flag > config > env > default
    - _Requirements: 5.2, 5.4, 5.5, 1.1, 1.2, 1.3_
  
  - [x] 6.3 Integrate i18n into Go client main application
    - Import i18n package in client/main.go
    - Replace all hardcoded strings with i18n.T() calls
    - Update console output, prompts, and error messages
    - _Requirements: 5.3_
  
  - [x] 6.4 Write unit tests for Go i18n package
    - Test language loading from embedded files
    - Test translation lookup and fallback behavior
    - Test language detection from env, config, and flag
    - Test parameter substitution
    - _Requirements: 9.5_

- [x] 7. Implement dynamic text features
  - [x] 7.1 Add placeholder substitution support
    - Implement parameter replacement in Electron I18nManager
    - Implement parameter replacement in Go i18n package
    - Android handles this natively with string formatting
    - _Requirements: 7.1_
  
  - [x] 7.2 Add pluralization support
    - Implement plural form selection in Electron I18nManager
    - Implement plural form selection in Go i18n package
    - Add plural forms to translation files where needed
    - _Requirements: 7.2, 7.3, 7.4_
  
  - [x] 7.3 Write tests for dynamic text features
    - Test placeholder substitution with various inputs
    - Test plural forms for different languages
    - Test edge cases (zero, one, many)
    - _Requirements: 7.1, 7.2, 7.3_

- [x] 8. Implement error handling and fallback mechanisms
  - [x] 8.1 Add error handling for translation loading failures
    - Handle missing translation files in all three clients
    - Handle malformed JSON in Electron and Go clients
    - Log errors without crashing the application
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
  
  - [x] 8.2 Implement fallback chain
    - Ensure missing keys fall back to English translation
    - Ensure missing English keys return the key itself
    - Test fallback behavior in all three clients
    - _Requirements: 2.4, 2.5_
  
  - [x] 8.3 Write integration tests for error handling
    - Test with missing translation files
    - Test with corrupted JSON files
    - Test with missing keys
    - Verify app remains functional in all error scenarios
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [x] 9. Checkpoint - Test all three clients end-to-end
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Performance optimization and validation
  - [x] 10.1 Implement lazy loading and caching
    - Ensure only active language is loaded on startup
    - Cache translations in memory for fast lookup
    - Verify no file I/O during translation lookups
    - _Requirements: 10.1, 10.2, 10.5_
  
  - [x] 10.2 Optimize language switching performance
    - Measure and optimize language switch time to < 500ms
    - Implement efficient UI update mechanism
    - _Requirements: 10.3_
  
  - [x] 10.3 Run performance tests
    - Measure app startup time with different languages
    - Measure language switching time
    - Measure memory usage of loaded translations
    - Verify binary size increase is < 500KB per language
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [x] 11. Final integration and validation
  - [x] 11.1 Run translation validation script on all files
    - Verify all translation files have matching keys
    - Verify 95%+ coverage for all languages
    - Fix any missing or duplicate keys
    - _Requirements: 6.2, 6.3, 9.1, 9.2, 9.3, 9.4_
  
  - [x] 11.2 Perform manual testing across all platforms
    - Test each language on Android, Electron, and Go clients
    - Verify RTL layout for Arabic and Farsi
    - Verify text fits within UI elements
    - Verify pluralization and dynamic text
    - _Requirements: 1.6, 3.4, 3.5, 4.3, 4.5, 4.6, 5.3, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.5_
  
  - [x] 11.3 Update documentation
    - Document how to add new languages
    - Document translation key naming conventions
    - Document how to use the validation script
    - _Requirements: 2.1, 2.2, 6.3_

- [x] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- The implementation follows platform-native patterns: Android uses resources, Electron uses JSON with dynamic loading, Go uses embedded JSON
- Translation files should be created with placeholder text initially and can be professionally translated later
- RTL support is critical for Arabic and Farsi users
- Performance requirements are strict: < 500ms for language switching, < 500KB per language
