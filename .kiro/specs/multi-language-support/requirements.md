# Requirements Document

## Introduction

This document defines requirements for adding multi-language support (internationalization/i18n) to the DNSTT Messenger application. The messenger currently has hardcoded Russian text across three client platforms (Android, Electron, Go CLI). This feature will enable the application to support multiple languages with a focus on languages from countries with heavy internet censorship, while maintaining English as the default language.

## Glossary

- **I18n_System**: The internationalization system responsible for managing translations and language switching
- **Translation_File**: A file containing key-value pairs mapping translation keys to localized text
- **Language_Code**: ISO 639-1 two-letter language code (e.g., "en", "ru", "zh", "fa")
- **Default_Language**: English ("en") - the fallback language when translations are missing
- **Target_Languages**: Languages from countries with heavy internet censorship (Chinese, Farsi, Russian, Arabic, Turkish, Vietnamese)
- **Android_Client**: The Android mobile application
- **Electron_Client**: The desktop application built with Electron
- **Go_Client**: The command-line client written in Go
- **Translation_Key**: A unique identifier for a translatable string (e.g., "login.button.submit")
- **Locale_Detector**: Component that determines the user's preferred language from system settings

## Requirements

### Requirement 1: Language Detection and Selection

**User Story:** As a user, I want the application to automatically detect my system language, so that I see the interface in my preferred language without manual configuration.

#### Acceptance Criteria

1. WHEN the application starts for the first time, THE Locale_Detector SHALL determine the system language
2. IF the system language matches a supported Target_Language, THEN THE I18n_System SHALL load that language
3. IF the system language is not supported, THEN THE I18n_System SHALL load the Default_Language
4. THE I18n_System SHALL provide a manual language selection option in the settings
5. WHEN a user manually selects a language, THE I18n_System SHALL persist this preference across application restarts
6. THE I18n_System SHALL support at least the following Target_Languages: Chinese (zh), Farsi (fa), Russian (ru), Arabic (ar), Turkish (tr), Vietnamese (vi), and English (en)

### Requirement 2: Translation File Management

**User Story:** As a developer, I want translation files organized in a standard format, so that I can easily add or update translations.

#### Acceptance Criteria

1. THE I18n_System SHALL store translations in JSON format with Translation_Key to text mappings
2. THE I18n_System SHALL organize Translation_Files by Language_Code (e.g., "en.json", "ru.json", "zh.json")
3. WHEN a Translation_Key is requested, THE I18n_System SHALL return the text for the current language
4. IF a Translation_Key is missing in the current language, THEN THE I18n_System SHALL return the Default_Language text
5. IF a Translation_Key is missing in both current and Default_Language, THEN THE I18n_System SHALL return the Translation_Key itself
6. THE Translation_Files SHALL be embedded in the application binary for offline operation

### Requirement 3: Android Client Internationalization

**User Story:** As an Android user, I want the app interface in my language, so that I can understand all buttons, labels, and messages.

#### Acceptance Criteria

1. THE Android_Client SHALL use Android's native resource system for translations (strings.xml files)
2. THE Android_Client SHALL provide strings.xml files for each supported Target_Language in the appropriate values-{language} directories
3. WHEN the Android_Client starts, THE Locale_Detector SHALL use Android's system locale
4. THE Android_Client SHALL translate all UI elements including: button labels, screen titles, input placeholders, error messages, notification text, and dialog content
5. WHEN the system language changes, THE Android_Client SHALL update the UI text without requiring an app restart

### Requirement 4: Electron Client Internationalization

**User Story:** As a desktop user, I want the Electron app interface in my language, so that I can navigate the application comfortably.

#### Acceptance Criteria

1. THE Electron_Client SHALL load Translation_Files from the application resources directory
2. WHEN the Electron_Client starts, THE Locale_Detector SHALL detect the operating system language
3. THE Electron_Client SHALL translate all UI elements in index.html including: tab labels, button text, input placeholders, sidebar section labels, modal dialog content, and status messages
4. THE Electron_Client SHALL provide a language selector in the settings tab
5. WHEN a user changes the language, THE Electron_Client SHALL update all visible text immediately without requiring an application restart
6. THE Electron_Client SHALL support right-to-left (RTL) text direction for Arabic and Farsi

### Requirement 5: Go Client Internationalization

**User Story:** As a command-line user, I want console messages in my language, so that I can understand prompts and responses.

#### Acceptance Criteria

1. THE Go_Client SHALL embed Translation_Files in the binary using Go embed directives
2. WHEN the Go_Client starts, THE Locale_Detector SHALL check the LANG environment variable
3. THE Go_Client SHALL translate all console output including: login prompts, error messages, status messages, command help text, and user notifications
4. THE Go_Client SHALL provide a --lang command-line flag to override the detected language
5. THE Go_Client SHALL store the language preference in client_config.json

### Requirement 6: Translation Completeness and Quality

**User Story:** As a user, I want complete and accurate translations, so that I don't see mixed languages or untranslated text.

#### Acceptance Criteria

1. THE I18n_System SHALL provide translations for all user-facing text in the Default_Language
2. FOR ALL supported Target_Languages, THE I18n_System SHALL provide translations for at least 95% of Translation_Keys
3. WHEN a new Translation_Key is added to the codebase, THE I18n_System SHALL include it in all Translation_Files
4. THE I18n_System SHALL maintain a translation coverage report showing completion percentage per language
5. THE Translation_Files SHALL not contain HTML or code - only plain text or text with simple formatting placeholders

### Requirement 7: Dynamic Text and Pluralization

**User Story:** As a user, I want messages with numbers to be grammatically correct in my language, so that the interface feels natural.

#### Acceptance Criteria

1. WHEN displaying text with variable content, THE I18n_System SHALL support placeholder substitution (e.g., "Hello {username}")
2. THE I18n_System SHALL support plural forms appropriate to each language's grammar rules
3. WHEN displaying counts, THE I18n_System SHALL select the correct plural form based on the language's pluralization rules
4. THE I18n_System SHALL support at least zero, one, and many plural forms
5. WHEN formatting dates and times, THE I18n_System SHALL use locale-appropriate formats

### Requirement 8: Fallback and Error Handling

**User Story:** As a user, I want the application to remain functional even if translations are incomplete, so that I can still use the app.

#### Acceptance Criteria

1. IF a Translation_File fails to load, THEN THE I18n_System SHALL fall back to the Default_Language
2. IF the Default_Language Translation_File is missing or corrupted, THEN THE I18n_System SHALL use hardcoded English strings
3. WHEN a translation error occurs, THE I18n_System SHALL log the error but continue operation
4. THE I18n_System SHALL not crash or freeze due to missing or malformed translations
5. IF a Translation_File contains invalid JSON, THEN THE I18n_System SHALL log the error and use the Default_Language

### Requirement 9: Testing and Validation

**User Story:** As a developer, I want automated tests for translations, so that I can catch missing or broken translations early.

#### Acceptance Criteria

1. THE I18n_System SHALL provide a validation tool that checks all Translation_Files for structural correctness
2. THE validation tool SHALL verify that all Translation_Keys in the Default_Language exist in other Translation_Files
3. THE validation tool SHALL detect duplicate Translation_Keys within a single Translation_File
4. THE validation tool SHALL verify that placeholder variables in translations match the Default_Language
5. THE I18n_System SHALL include unit tests that verify language switching works correctly on all three client platforms

### Requirement 10: Performance and Resource Usage

**User Story:** As a user, I want language support to not slow down the application, so that my experience remains smooth.

#### Acceptance Criteria

1. WHEN the application starts, THE I18n_System SHALL load only the active language Translation_File
2. THE I18n_System SHALL cache loaded translations in memory for fast access
3. WHEN switching languages, THE I18n_System SHALL complete the switch within 500 milliseconds
4. THE Translation_Files SHALL not increase the application binary size by more than 500KB per language
5. THE I18n_System SHALL not perform file I/O operations during normal text translation lookups
