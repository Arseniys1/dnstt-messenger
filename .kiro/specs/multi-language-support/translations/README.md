# Translation Files

This directory contains the base English translation file and will contain translations for all supported languages.

## Supported Languages

The DNSTT Messenger will support the following languages:

1. **English (en)** - Default language ✅ Created
2. **Chinese (zh)** - 中文
3. **Farsi (fa)** - فارسی
4. **Russian (ru)** - Русский
5. **Arabic (ar)** - العربية
6. **Turkish (tr)** - Türkçe
7. **Vietnamese (vi)** - Tiếng Việt

## File Structure

- `en.json` - Base English translations (reference for all other languages)
- `zh.json` - Chinese translations (to be created)
- `fa.json` - Farsi translations (to be created)
- `ru.json` - Russian translations (to be created)
- `ar.json` - Arabic translations (to be created)
- `tr.json` - Turkish translations (to be created)
- `vi.json` - Vietnamese translations (to be created)

## Translation Key Format

All translation files use JSON format with dot-notation keys:

```json
{
  "app.name": "DNSTT Messenger",
  "login.username": "Username",
  "error.connection_failed": "Connection error: {error}"
}
```

### Parameter Substitution

Keys with `{variable}` placeholders support dynamic parameter substitution:

- `{user}` - Username
- `{count}` - Numeric count
- `{error}` - Error message
- `{server}` - Server address
- `{proxy}` - Proxy address
- `{time}` - Timestamp
- `{sender}` - Message sender
- `{text}` - Message text
- `{name}` - Room/channel name
- `{id}` - Room/channel ID
- `{inviter}` - User who sent invitation

## Platform-Specific Usage

### Android (Kotlin)
Android will use the native resource system with `strings.xml` files in `values-{lang}` directories. The JSON files serve as the source of truth for creating Android string resources.

### Electron (JavaScript)
Electron will load these JSON files directly from the `electron-client/i18n/locales/` directory.

### Go CLI
Go will embed these JSON files from the `client/i18n/locales/` directory using Go's embed directive.

## Translation Guidelines

1. **Maintain key consistency**: All language files must have the same keys as `en.json`
2. **Preserve placeholders**: Keep `{variable}` placeholders exactly as they appear
3. **No HTML or code**: Translation values should contain only plain text
4. **Cultural adaptation**: Adapt messages to be culturally appropriate, not just literal translations
5. **Length considerations**: Some UI elements have space constraints; keep translations concise
6. **RTL support**: Arabic and Farsi require right-to-left text direction (handled by platform code)

## Validation

Use the translation validation script (to be created in Task 3.3) to verify:
- All keys from `en.json` exist in other language files
- No extra keys exist in translation files
- Placeholder variables match across all languages
- JSON syntax is valid

## Coverage Requirements

Per Requirement 6.2, all supported languages must have at least 95% translation coverage. The validation script will report coverage percentages.

## Contributing Translations

When adding or updating translations:

1. Update `en.json` first (source of truth)
2. Update all other language files to maintain key parity
3. Run the validation script to verify completeness
4. Test the translations in the actual application UI
5. Verify text fits within UI elements without truncation
