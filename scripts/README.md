# DNSTT Messenger - Utility Scripts

This directory contains utility scripts for the DNSTT Messenger project.

## Translation Validation Script

The `validate-translations.js` script validates translation file completeness across all three DNSTT Messenger clients (Android, Electron, and Go).

### Features

- **Multi-platform validation**: Validates translations for Android (strings.xml), Electron (JSON), and Go (JSON)
- **Missing key detection**: Identifies translation keys that are missing in non-English languages
- **Extra key detection**: Identifies translation keys that exist in translations but not in the base English file
- **Coverage reporting**: Calculates and displays coverage percentage for each language
- **Color-coded output**: Easy-to-read terminal output with color-coded status indicators
- **Exit codes**: Returns exit code 1 if validation fails (useful for CI/CD pipelines)

### Requirements

- Node.js (v14 or higher)
- npm

### Installation

Install dependencies:

```bash
cd scripts
npm install
```

Or from the root directory:

```bash
make install-deps
```

### Usage

There are three ways to run the validation script:

#### 1. Direct Node.js execution

```bash
node scripts/validate-translations.js
```

#### 2. Using npm script (from electron-client directory)

```bash
cd electron-client
npm run validate-translations
```

#### 3. Using Makefile (from root directory)

```bash
make validate-translations
```

### Output

The script provides detailed output including:

- Per-language validation results for each platform
- List of missing keys (up to 10 shown, with count of remaining)
- List of extra keys (up to 5 shown, with count of remaining)
- Coverage percentage for each language
- Summary table showing all validations
- Overall pass/fail status

Example output:

```
DNSTT Messenger - Translation Validation

Validating translations for 7 languages across 3 platforms...

Validating Electron translations...

Electron - zh
  ✓ Coverage: 100.0% (130/130 keys)
  ✓ All keys match!

...

═══════════════════════════════════════════════════════════
                    VALIDATION SUMMARY
═══════════════════════════════════════════════════════════

Electron:
  Lang    Coverage    Status
  ────────────────────────────
  zh     100.0%      ✓
  fa     100.0%      ✓
  ...

Overall:
  Total validations: 18
  Passed: 18

═══════════════════════════════════════════════════════════

All validations passed!
```

### Configuration

The script validates the following languages:
- English (en) - base language
- Chinese (zh)
- Farsi (fa)
- Russian (ru)
- Arabic (ar)
- Turkish (tr)
- Vietnamese (vi)

Minimum required coverage: **95%**

### File Locations

The script validates translation files in the following locations:

- **Electron**: `electron-client/i18n/locales/*.json`
- **Go**: `client/i18n/locales/*.json`
- **Android**: `android-client/app/src/main/res/values*/strings.xml`

### CI/CD Integration

The script returns appropriate exit codes for CI/CD integration:

- **Exit code 0**: All validations passed
- **Exit code 1**: One or more validations failed

Example GitHub Actions workflow:

```yaml
- name: Validate translations
  run: node scripts/validate-translations.js
```

Example GitLab CI:

```yaml
validate-translations:
  script:
    - node scripts/validate-translations.js
```

### Troubleshooting

**Error: Cannot find module 'xml2js'**

Make sure you've installed the dependencies:

```bash
cd scripts
npm install
```

**Missing translation files**

Ensure all translation files exist for all supported languages in all three platforms.

**Coverage below 95%**

Add the missing translation keys to the respective language files. The script will list which keys are missing.
