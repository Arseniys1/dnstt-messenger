# DNSTT Messenger - Internationalization Guide

This guide provides comprehensive instructions for maintaining and extending the multi-language support system in DNSTT Messenger.

## Table of Contents

1. [Adding New Languages](#adding-new-languages)
2. [Translation Key Naming Conventions](#translation-key-naming-conventions)
3. [Using the Validation Script](#using-the-validation-script)
4. [Platform-Specific Guidelines](#platform-specific-guidelines)

---

## Adding New Languages

The DNSTT Messenger supports internationalization across three client platforms: Android, Electron, and Go CLI. To add a new language, you must create translation files for all three platforms.

### Prerequisites

- ISO 639-1 language code (e.g., `es` for Spanish, `de` for German)
- Native language name for UI display
- Knowledge of whether the language uses RTL (right-to-left) text direction

### Step 1: Update Language Configuration

#### 1.1 Update the Design Document

Add your language to the `SUPPORTED_LANGUAGES` array in `.kiro/specs/multi-language-support/design.md`:

```javascript
const SUPPORTED_LANGUAGES = [
    { code: 'en', name: 'English', nativeName: 'English', rtl: false },
    { code: 'zh', name: 'Chinese', nativeName: '中文', rtl: false },
    // ... existing languages ...
    { code: 'es', name: 'Spanish', nativeName: 'Español', rtl: false }  // New language
];
```

#### 1.2 Update the Validation Script

Edit `scripts/validate-translations.js` and add your language code to the `SUPPORTED_LANGUAGES` array:

```javascript
const SUPPORTED_LANGUAGES = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi', 'es'];
```

### Step 2: Create Android Translation Files

Android uses XML resource files for translations.

#### 2.1 Create Language Directory

Create a new values directory for your language:

```bash
mkdir -p android-client/app/src/main/res/values-{LANG_CODE}
```

Example for Spanish:
```bash
mkdir -p android-client/app/src/main/res/values-es
```

#### 2.2 Create strings.xml

Copy the base English strings file and translate:

```bash
cp android-client/app/src/main/res/values/strings.xml \
   android-client/app/src/main/res/values-es/strings.xml
```

Then translate all string values in the new file. Keep the `name` attributes unchanged:

```xml
<resources>
    <string name="app_name">DNSTT Messenger</string>
    <string name="login_title">Iniciar sesión</string>
    <string name="login_username">Nombre de usuario</string>
    <!-- ... translate all strings ... -->
</resources>
```

**Important:** 
- Keep all `name` attributes exactly as they are in the English file
- Only translate the text content between the tags
- Preserve placeholder syntax like `%s`, `%d`, `%1$s`

### Step 3: Create Electron Translation Files

Electron uses JSON files for translations.

#### 3.1 Create JSON File

Create a new JSON file in the Electron locales directory:

```bash
touch electron-client/i18n/locales/{LANG_CODE}.json
```

Example for Spanish:
```bash
touch electron-client/i18n/locales/es.json
```

#### 3.2 Copy and Translate

Copy the structure from `electron-client/i18n/locales/en.json` and translate all values:

```json
{
    "app.name": "DNSTT Messenger",
    "login.title": "Iniciar sesión",
    "login.username": "Nombre de usuario",
    "login.password": "Contraseña",
    "chat.input.placeholder": "Mensaje...",
    "chat.online_count": "{count} en línea"
}
```

**Important:**
- Keep all keys exactly as they are in the English file
- Only translate the values (right side of the colon)
- Preserve placeholder syntax like `{variable}`, `{count}`, `{user}`

### Step 4: Create Go Translation Files

Go also uses JSON files, embedded in the binary.

#### 4.1 Create JSON File

Create a new JSON file in the Go locales directory:

```bash
touch client/i18n/locales/{LANG_CODE}.json
```

Example for Spanish:
```bash
touch client/i18n/locales/es.json
```

#### 4.2 Copy and Translate

Copy the structure from `client/i18n/locales/en.json` and translate all values. The format is identical to Electron's JSON files.

### Step 5: Update Language Managers

#### 5.1 Android LocaleManager

Edit `android-client/app/src/main/java/com/example/myapplication/LocaleManager.kt` and add your language to the `getSupportedLanguages()` method:

```kotlin
fun getSupportedLanguages(): List<Language> {
    return listOf(
        Language("en", "English", "English"),
        Language("zh", "Chinese", "中文"),
        // ... existing languages ...
        Language("es", "Spanish", "Español")  // Add new language
    )
}
```

#### 5.2 Electron I18nManager

Edit `electron-client/i18n/manager.js` and add your language code to the `supportedLanguages` array:

```javascript
constructor() {
    this.supportedLanguages = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi', 'es'];
    // ...
}
```

If your language uses RTL text direction, also update the `setTextDirection` function:

```javascript
function setTextDirection(languageCode) {
    const rtlLanguages = ['ar', 'fa'];  // Add your RTL language code here if needed
    const direction = rtlLanguages.includes(languageCode) ? 'rtl' : 'ltr';
    document.documentElement.setAttribute('dir', direction);
}
```

#### 5.3 Go I18n Manager

Edit `client/i18n/i18n.go` and add your language code to the `GetSupportedLanguages()` function:

```go
func (m *Manager) GetSupportedLanguages() []string {
    return []string{"en", "zh", "fa", "ru", "ar", "tr", "vi", "es"}
}
```

### Step 6: Validate Your Translations

Run the validation script to ensure all keys are present:

```bash
node scripts/validate-translations.js
```

The script will report:
- Missing keys (keys in English but not in your translation)
- Extra keys (keys in your translation but not in English)
- Coverage percentage (should be 95% or higher)

Fix any issues reported by the validation script.

### Step 7: Test Your Language

#### Android Testing
1. Build and run the Android app
2. Go to Settings → Language
3. Select your new language
4. Verify all UI elements are translated correctly
5. Test RTL layout if applicable

#### Electron Testing
1. Run the Electron app: `npm start` (from electron-client directory)
2. Go to Settings tab
3. Select your new language from the dropdown
4. Verify all UI elements update immediately
5. Test RTL layout if applicable

#### Go CLI Testing
1. Build the Go client: `go build` (from client directory)
2. Run with language flag: `./client --lang=es`
3. Verify all console output is translated
4. Test all commands and error messages

---

## Translation Key Naming Conventions

Translation keys follow a hierarchical dot-notation structure for consistency across all platforms.

### Key Structure

```
<category>.<subcategory>.<element>
```

### Categories

| Category | Purpose | Example Keys |
|----------|---------|--------------|
| `app.*` | Application-level strings | `app.name`, `app.title` |
| `login.*` | Login and registration | `login.title`, `login.username` |
| `chat.*` | Chat interface | `chat.title_global`, `chat.input_placeholder` |
| `settings.*` | Settings and configuration | `settings.server_address`, `settings.language` |
| `sidebar.*` | Sidebar navigation | `sidebar.section_chats`, `sidebar.button_logout` |
| `dm.*` | Direct messages | `dm.title`, `dm.new_conversation` |
| `room.*` | Room/channel functionality | `room.create`, `room.members_count` |
| `status.*` | Status messages | `status.connecting`, `status.connected` |
| `error.*` | Error messages | `error.connection_failed`, `error.invalid_credentials` |
| `success.*` | Success messages | `success.account_created`, `success.settings_saved` |
| `notification.*` | Notification text | `notification.new_message`, `notification.incoming_messages` |
| `command.*` | CLI command help (Go only) | `command.help_exit`, `command.usage_dm` |
| `server.*` | Server and network | `server.list_title`, `server.network_servers` |
| `misc.*` | Miscellaneous | `misc.online_users`, `misc.prompt` |

### Naming Rules

1. **Use lowercase**: All keys must be lowercase
2. **Use dots for hierarchy**: Separate levels with dots (`.`)
3. **Use underscores for multi-word elements**: `login.button_login`, not `login.button-login`
4. **Be descriptive**: Keys should clearly indicate their purpose
5. **Group related keys**: Keep related translations under the same category
6. **Avoid abbreviations**: Use full words for clarity (`button` not `btn`)

### Examples

#### Good Key Names ✓
```
login.username
chat.input_placeholder
error.connection_failed
settings.server_address
room.members_count
```

#### Bad Key Names ✗
```
LoginUsername          // Not lowercase, no hierarchy
chat_input             // Missing category hierarchy
err.conn               // Unclear abbreviations
settings-server        // Using hyphens instead of underscores
room.count             // Not descriptive enough
```

### Platform-Specific Key Formats

#### Android (strings.xml)
Android uses underscores in XML attribute names, which are converted to dots internally:

```xml
<string name="login_username">Username</string>
<!-- Accessed in code as: login.username -->
```

#### Electron and Go (JSON)
JSON files use the dot notation directly:

```json
{
    "login.username": "Username"
}
```

### Placeholder Variables

When a translation includes dynamic content, use curly braces for placeholders:

```json
{
    "chat.online_count": "{count} online",
    "dm.sent_to": "[DM → {user}]: {text}",
    "error.connection_failed": "Connection error: {error}"
}
```

**Rules for placeholders:**
- Use descriptive names: `{username}` not `{x}`
- Keep placeholder names consistent across languages
- Document expected data types in comments if needed

### Pluralization Keys

For languages with plural forms, use suffixes:

```json
{
    "chat.online_count": "{count} online",
    "chat.online_count_plural": "{count} users online",
    "room.members_count": "{count} member",
    "room.members_count_plural": "{count} members"
}
```

### Adding New Keys

When adding new translation keys:

1. **Choose the appropriate category** based on the feature
2. **Follow the naming conventions** outlined above
3. **Add the key to ALL language files** (start with English)
4. **Update the translation-keys.md** documentation
5. **Run the validation script** to ensure consistency

Example workflow:
```bash
# 1. Add key to English files
# electron-client/i18n/locales/en.json
{
    "profile.edit_button": "Edit Profile"
}

# 2. Add to all other language files
# electron-client/i18n/locales/es.json
{
    "profile.edit_button": "Editar perfil"
}

# 3. Validate
node scripts/validate-translations.js
```

---

## Using the Validation Script

The validation script (`scripts/validate-translations.js`) ensures translation completeness and consistency across all three platforms.

### Prerequisites

Install required dependencies:

```bash
npm install xml2js
```

### Running the Script

From the project root directory:

```bash
node scripts/validate-translations.js
```

Or if you've added it to package.json:

```bash
npm run validate-translations
```

### What the Script Checks

The validation script performs the following checks for each language:

1. **File Existence**: Verifies all translation files exist
2. **JSON Validity**: Ensures JSON files are properly formatted
3. **XML Validity**: Ensures Android XML files are properly formatted
4. **Key Completeness**: Checks that all keys from English exist in other languages
5. **Extra Keys**: Identifies keys that exist in translations but not in English
6. **Coverage Percentage**: Calculates what percentage of keys are translated

### Understanding the Output

#### Individual Language Results

For each language and platform, you'll see:

```
Electron - zh
  ✓ Coverage: 100.0% (87/87 keys)
  ✓ All keys match!
```

Or if there are issues:

```
Electron - es
  ✗ Coverage: 92.0% (80/87 keys)
  ⚠ Missing keys (7):
    - chat.history_divider
    - room.invite_title
    - error.service_not_ready
    ... and 4 more
  ℹ Extra keys (2):
    - chat.historial_divider
    - room.titulo_invitar
```

#### Summary Table

The script provides a summary table showing all validations:

```
═══════════════════════════════════════════════════════════
                    VALIDATION SUMMARY
═══════════════════════════════════════════════════════════

Electron:
  Lang    Coverage    Status
  ────────────────────────────
  zh      100.0%      ✓
  fa      98.9%       ✓
  ru      100.0%      ✓
  ar      97.7%       ✓ (2 issues)
  tr      100.0%      ✓
  vi      100.0%      ✓

Go:
  Lang    Coverage    Status
  ────────────────────────────
  zh      100.0%      ✓
  fa      100.0%      ✓
  ru      100.0%      ✓
  ar      100.0%      ✓
  tr      100.0%      ✓
  vi      100.0%      ✓

Android:
  Lang    Coverage    Status
  ────────────────────────────
  zh      100.0%      ✓
  fa      100.0%      ✓
  ru      100.0%      ✓
  ar      100.0%      ✓
  tr      100.0%      ✓
  vi      100.0%      ✓

Overall:
  Total validations: 18
  Passed: 18
```

### Exit Codes

- **0**: All validations passed (coverage ≥ 95% for all languages)
- **1**: One or more validations failed

This makes the script suitable for CI/CD pipelines:

```bash
# In CI/CD pipeline
node scripts/validate-translations.js || exit 1
```

### Common Issues and Solutions

#### Issue: Missing Keys

**Output:**
```
⚠ Missing keys (3):
  - chat.new_feature
  - settings.advanced_option
  - error.new_error_type
```

**Solution:**
Add the missing keys to the translation file. Copy from the English file and translate:

```json
{
    "chat.new_feature": "Nueva función",
    "settings.advanced_option": "Opción avanzada",
    "error.new_error_type": "Error de tipo nuevo"
}
```

#### Issue: Extra Keys

**Output:**
```
ℹ Extra keys (2):
  - chat.old_feature
  - settings.deprecated_option
```

**Solution:**
These keys exist in the translation but not in English. Either:
1. Remove them if they're obsolete
2. Add them to the English file if they should exist

#### Issue: Low Coverage

**Output:**
```
✗ Coverage: 87.4% (76/87 keys)
```

**Solution:**
Coverage below 95% indicates many missing translations. Review the missing keys list and add translations for all of them.

#### Issue: Malformed JSON

**Output:**
```
Error loading electron-client/i18n/locales/es.json: Unexpected token } in JSON
```

**Solution:**
Fix the JSON syntax error. Common issues:
- Missing or extra commas
- Unescaped quotes in strings
- Missing closing braces

Use a JSON validator or linter to identify the exact issue.

#### Issue: Malformed XML (Android)

**Output:**
```
Error loading android-client/app/src/main/res/values-es/strings.xml: Unclosed tag
```

**Solution:**
Fix the XML syntax error. Common issues:
- Unclosed tags: `<string name="key">Value` (missing `</string>`)
- Unescaped special characters: Use `&lt;`, `&gt;`, `&amp;`, `&quot;`, `&apos;`
- Missing quotes around attribute values

### Integrating with CI/CD

Add the validation script to your CI/CD pipeline to catch translation issues early:

#### GitHub Actions Example

```yaml
name: Validate Translations

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-node@v2
        with:
          node-version: '16'
      - run: npm install xml2js
      - run: node scripts/validate-translations.js
```

#### GitLab CI Example

```yaml
validate-translations:
  stage: test
  script:
    - npm install xml2js
    - node scripts/validate-translations.js
```

### Manual Validation Workflow

For developers adding or updating translations:

1. **Make your changes** to translation files
2. **Run the validation script**:
   ```bash
   node scripts/validate-translations.js
   ```
3. **Review the output** for any issues
4. **Fix any missing or extra keys**
5. **Re-run the script** until all validations pass
6. **Commit your changes** once validation succeeds

---

## Platform-Specific Guidelines

### Android

#### File Location
```
android-client/app/src/main/res/
├── values/strings.xml           (English - default)
├── values-zh/strings.xml        (Chinese)
├── values-fa/strings.xml        (Farsi)
└── values-{LANG}/strings.xml    (Other languages)
```

#### Key Format
Android uses underscores in XML attributes, converted to dots in code:
```xml
<string name="login_username">Username</string>
```

#### Special Characters
Escape special characters in XML:
- `<` → `&lt;`
- `>` → `&gt;`
- `&` → `&amp;`
- `"` → `&quot;`
- `'` → `&apos;` or `\'`

#### Placeholders
Use Android string formatting:
```xml
<string name="chat_online_count">%d online</string>
<string name="dm_sent_to">[DM → %1$s]: %2$s</string>
```

#### Testing
```bash
cd android-client
./gradlew test
```

### Electron

#### File Location
```
electron-client/i18n/locales/
├── en.json
├── zh.json
├── fa.json
└── {LANG}.json
```

#### Key Format
Use dot notation directly:
```json
{
    "login.username": "Username"
}
```

#### Placeholders
Use curly braces:
```json
{
    "chat.online_count": "{count} online",
    "dm.sent_to": "[DM → {user}]: {text}"
}
```

#### RTL Support
For RTL languages (Arabic, Farsi), the I18nManager automatically sets the `dir` attribute on the HTML root element.

#### Testing
```bash
cd electron-client
npm test
```

### Go

#### File Location
```
client/i18n/locales/
├── en.json
├── zh.json
├── fa.json
└── {LANG}.json
```

#### Embedding
Files are embedded in the binary using Go embed directives (already configured in `i18n.go`):
```go
//go:embed locales/*.json
var localesFS embed.FS
```

#### Key Format
Same as Electron - use dot notation:
```json
{
    "login.username": "Username"
}
```

#### Placeholders
Use `%s`, `%d`, etc. for fmt.Sprintf:
```json
{
    "chat.online_count": "%d online",
    "error.connection_failed": "Connection error: %s"
}
```

#### Testing
```bash
cd client
go test ./i18n
```

---

## Best Practices

### Translation Quality

1. **Use native speakers**: Whenever possible, have translations reviewed by native speakers
2. **Consider context**: Provide context to translators about where text appears
3. **Test in UI**: Always test translations in the actual UI to check for truncation or layout issues
4. **Maintain tone**: Keep the same tone and formality level across languages
5. **Avoid literal translations**: Translate meaning, not just words

### Maintenance

1. **Run validation regularly**: Make it part of your development workflow
2. **Update all languages together**: When adding new keys, update all language files at once
3. **Document changes**: Update translation-keys.md when adding new keys
4. **Version control**: Commit translation files with descriptive messages
5. **Track coverage**: Monitor translation coverage percentages over time

### Performance

1. **Keep translations concise**: Shorter text loads faster and fits better in UI
2. **Avoid duplication**: Reuse common translations where possible
3. **Lazy loading**: The system already implements lazy loading - only the active language is loaded
4. **Caching**: Translations are cached in memory for fast access

---

## Troubleshooting

### Problem: Language not appearing in UI

**Check:**
1. Language code added to all three platform managers
2. Translation files exist in all three locations
3. Validation script passes for that language
4. App restarted after adding language

### Problem: Some text not translated

**Check:**
1. All keys exist in translation file (run validation script)
2. Keys match exactly (case-sensitive)
3. Code is using translation system (not hardcoded strings)
4. Language is actually selected in settings

### Problem: RTL layout broken

**Check:**
1. Language code added to RTL languages array in Electron
2. CSS supports RTL (use logical properties like `margin-inline-start`)
3. Test with actual RTL text, not just switching language

### Problem: Validation script fails

**Check:**
1. xml2js dependency installed: `npm install xml2js`
2. All file paths are correct
3. JSON/XML files are properly formatted
4. Running from project root directory

---

## Additional Resources

- **Translation Keys Reference**: See `translation-keys.md` for complete key inventory
- **Design Document**: See `design.md` for architecture details
- **Requirements**: See `requirements.md` for feature requirements
- **Android i18n**: https://developer.android.com/guide/topics/resources/localization
- **ISO 639-1 Language Codes**: https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes

---

## Support

For questions or issues with internationalization:
1. Check this guide first
2. Run the validation script to identify issues
3. Review existing translation files for examples
4. Consult the design document for architecture details
