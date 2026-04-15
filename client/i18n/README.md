# i18n Package

Internationalization (i18n) package for the DNSTT Messenger Go CLI client.

## Features

- **Embedded translations**: All translation files are embedded in the binary using Go's `embed` directive for offline operation
- **7 supported languages**: English (en), Chinese (zh), Farsi (fa), Russian (ru), Arabic (ar), Turkish (tr), Vietnamese (vi)
- **Automatic language detection**: Detects system language from `LANG` environment variable
- **Fallback mechanism**: Falls back to English if translation is missing, then to the key itself
- **Parameter substitution**: Supports dynamic text with placeholders like `{user}`, `{count}`, etc.
- **Thread-safe**: Safe for concurrent use after initialization

## Usage

### Basic Usage

```go
import "dnstt-messenger/client/i18n"

// Create a new manager (loads English by default)
m := i18n.NewManager()

// Translate a key
text := m.T("login.title")  // Returns: "Login"

// Translate with parameters
msg := m.T("error.connection_failed", "error", "timeout")
// Returns: "Connection error: timeout"
```

### Language Detection and Switching

```go
// Detect system language
detected := m.DetectLanguage()

// Load detected language
if err := m.LoadLanguage(detected); err != nil {
    // Handle error - falls back to English
}

// Or use SetLanguage (same as LoadLanguage)
m.SetLanguage("zh")  // Switch to Chinese
```

### Parameter Substitution

Parameters are passed as key-value pairs:

```go
// Single parameter
m.T("sidebar.online_count", "count", 42)
// Returns: "Online (42)"

// Multiple parameters
m.T("status.mode_proxy", 
    "proxy", "127.0.0.1:18000",
    "server", "example.com:9999")
// Returns: "Mode: DNSTT Proxy (SOCKS5) | Proxy: 127.0.0.1:18000 -> Server: example.com:9999..."
```

### Supported Languages

```go
langs := m.GetSupportedLanguages()
// Returns: ["en", "zh", "fa", "ru", "ar", "tr", "vi"]

current := m.GetCurrentLanguage()
// Returns: "en" (or whatever is currently loaded)
```

## Translation Files

Translation files are located in `client/i18n/locales/` and follow the naming convention `{language_code}.json`:

- `en.json` - English (default/fallback)
- `zh.json` - Chinese (中文)
- `fa.json` - Farsi (فارسی)
- `ru.json` - Russian (Русский)
- `ar.json` - Arabic (العربية)
- `tr.json` - Turkish (Türkçe)
- `vi.json` - Vietnamese (Tiếng Việt)

### Translation File Format

```json
{
  "app.name": "DNSTT Messenger",
  "login.title": "Login",
  "error.connection_failed": "Connection error: {error}",
  "sidebar.online_count": "Online ({count})"
}
```

Keys use dot notation for hierarchical organization:
- `app.*` - Application-level strings
- `login.*` - Login and registration
- `chat.*` - Chat interface
- `settings.*` - Settings
- `error.*` - Error messages
- `status.*` - Status messages
- `command.*` - CLI commands

## Error Handling

The package handles errors gracefully:

1. **Missing translation file**: Falls back to English
2. **Malformed JSON**: Logs error and uses fallback
3. **Missing translation key**: Returns English translation, or the key itself if not found
4. **Invalid language code**: Returns error, keeps current language

All errors are logged to stderr but don't crash the application.

## Language Detection

The `DetectLanguage()` method detects the user's preferred language with the following priority:

1. **Command-line flag** (`--lang`): Highest priority, overrides all other settings
2. **Config file** (`client_config.json`): Saved language preference
3. **Environment variable** (`LANG`): System language setting
4. **Default**: Falls back to English (`en`)

### Detection Examples

```go
// Priority 1: Command-line flag (highest priority)
detected := m.DetectLanguage("zh", "client_config.json")
// Returns: "zh" (from flag, ignores config and env)

// Priority 2: Config file
detected := m.DetectLanguage("", "client_config.json")
// Returns: language from config file if present

// Priority 3: Environment variable
detected := m.DetectLanguage("", "")
// Returns: language from LANG environment variable

// Priority 4: Default
// If none of the above are set or valid, returns "en"
```

### Environment Variable Parsing

The method extracts the language code from the `LANG` environment variable:

- `LANG=zh_CN.UTF-8` → `zh`
- `LANG=en_US` → `en`
- `LANG=fa_IR.UTF-8` → `fa`
- `LANG=ru_RU` → `ru`

If `LANG` is not set or contains an unsupported language, it falls back to the next priority level.

### Saving Language Preference

You can save the user's language preference to the config file:

```go
// Save language preference to config
if err := m.SaveLanguageToConfig("client_config.json", "zh"); err != nil {
    // Handle error
}

// The config file will be updated with:
// {
//   "proxy_addr": "127.0.0.1:18000",
//   "server_addr": "127.0.0.1:9999",
//   "language": "zh"
// }
```

### Complete Detection Flow

```go
import (
    "flag"
    "dnstt-messenger/client/i18n"
)

func main() {
    // Parse command-line flags
    langFlag := flag.String("lang", "", "Language code (en, zh, fa, ru, ar, tr, vi)")
    flag.Parse()

    // Create i18n manager
    m := i18n.NewManager()

    // Detect language with full priority chain
    detected := m.DetectLanguage(*langFlag, "client_config.json")
    
    // Load the detected language
    if err := m.LoadLanguage(detected); err != nil {
        // Handle error
    }

    // Save preference if flag was used
    if *langFlag != "" {
        m.SaveLanguageToConfig("client_config.json", detected)
    }
}
```

## Requirements Satisfied

This implementation satisfies the following requirements from the multi-language-support spec:

- **Requirement 5.1**: Embeds translation files in binary using Go embed directives
- **Requirement 5.2**: Detects language from LANG environment variable
- **Requirement 5.4**: Provides --lang command-line flag support
- **Requirement 5.5**: Stores language preference in client_config.json
- **Requirement 2.6**: Translations embedded for offline operation
- **Requirement 8.1**: Graceful fallback to English when translations are missing
- **Requirement 1.1-1.3**: Automatic language detection from system settings with priority chain
- **Requirement 1.5**: Persists language preference across application restarts
- **Requirement 2.3-2.5**: Translation key lookup with fallback logic
- **Requirement 7.1**: Parameter substitution for dynamic text

## Testing

Run the test suite:

```bash
go test ./client/i18n/...
```

Run with coverage:

```bash
go test -cover ./client/i18n/...
```

Run examples:

```bash
go test -v -run Example ./client/i18n/
```
