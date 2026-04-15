#!/usr/bin/env node

/**
 * Translation Validation Script
 * 
 * Validates translation file completeness across all three DNSTT Messenger clients:
 * - Android (strings.xml files)
 * - Electron (JSON files)
 * - Go (JSON files)
 * 
 * Checks for:
 * - Missing keys
 * - Extra keys
 * - Coverage percentage
 * - Structural consistency
 */

const fs = require('fs');
const path = require('path');
const { parseStringPromise } = require('xml2js');

// Configuration
const SUPPORTED_LANGUAGES = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi'];
const BASE_LANGUAGE = 'en';
const MIN_COVERAGE = 95; // Minimum required coverage percentage

// Paths
const ELECTRON_LOCALES_DIR = path.join(__dirname, '..', 'electron-client', 'i18n', 'locales');
const GO_LOCALES_DIR = path.join(__dirname, '..', 'client', 'i18n', 'locales');
const ANDROID_RES_DIR = path.join(__dirname, '..', 'android-client', 'app', 'src', 'main', 'res');

// Color codes for terminal output
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
  bold: '\x1b[1m'
};

/**
 * Load JSON translation file
 */
function loadJsonTranslations(filePath) {
  try {
    const content = fs.readFileSync(filePath, 'utf8');
    return JSON.parse(content);
  } catch (error) {
    console.error(`${colors.red}Error loading ${filePath}: ${error.message}${colors.reset}`);
    return null;
  }
}

/**
 * Load Android strings.xml file
 */
async function loadAndroidStrings(filePath) {
  try {
    const content = fs.readFileSync(filePath, 'utf8');
    const result = await parseStringPromise(content);
    
    const translations = {};
    if (result.resources && result.resources.string) {
      result.resources.string.forEach(item => {
        const key = item.$.name.replace(/_/g, '.');
        translations[key] = item._;
      });
    }
    
    return translations;
  } catch (error) {
    console.error(`${colors.red}Error loading ${filePath}: ${error.message}${colors.reset}`);
    return null;
  }
}

/**
 * Get Android strings.xml path for a language
 */
function getAndroidStringsPath(lang) {
  if (lang === 'en') {
    return path.join(ANDROID_RES_DIR, 'values', 'strings.xml');
  }
  return path.join(ANDROID_RES_DIR, `values-${lang}`, 'strings.xml');
}

/**
 * Validate a single translation file against the base
 */
function validateTranslations(baseKeys, translations, lang, platform) {
  const translationKeys = Object.keys(translations);
  
  // Find missing keys
  const missingKeys = baseKeys.filter(key => !translationKeys.includes(key));
  
  // Find extra keys
  const extraKeys = translationKeys.filter(key => !baseKeys.includes(key));
  
  // Calculate coverage
  const coverage = (translationKeys.length / baseKeys.length) * 100;
  
  return {
    lang,
    platform,
    totalKeys: baseKeys.length,
    translatedKeys: translationKeys.length,
    missingKeys,
    extraKeys,
    coverage: coverage.toFixed(1)
  };
}

/**
 * Print validation results
 */
function printResults(results) {
  const coverageColor = results.coverage >= MIN_COVERAGE ? colors.green : colors.red;
  const statusIcon = results.coverage >= MIN_COVERAGE ? '✓' : '✗';
  
  console.log(`\n${colors.bold}${results.platform} - ${results.lang}${colors.reset}`);
  console.log(`  ${coverageColor}${statusIcon} Coverage: ${results.coverage}%${colors.reset} (${results.translatedKeys}/${results.totalKeys} keys)`);
  
  if (results.missingKeys.length > 0) {
    console.log(`  ${colors.yellow}⚠ Missing keys (${results.missingKeys.length}):${colors.reset}`);
    results.missingKeys.slice(0, 10).forEach(key => {
      console.log(`    - ${key}`);
    });
    if (results.missingKeys.length > 10) {
      console.log(`    ... and ${results.missingKeys.length - 10} more`);
    }
  }
  
  if (results.extraKeys.length > 0) {
    console.log(`  ${colors.cyan}ℹ Extra keys (${results.extraKeys.length}):${colors.reset}`);
    results.extraKeys.slice(0, 5).forEach(key => {
      console.log(`    - ${key}`);
    });
    if (results.extraKeys.length > 5) {
      console.log(`    ... and ${results.extraKeys.length - 5} more`);
    }
  }
  
  if (results.missingKeys.length === 0 && results.extraKeys.length === 0) {
    console.log(`  ${colors.green}✓ All keys match!${colors.reset}`);
  }
}

/**
 * Print summary table
 */
function printSummary(allResults) {
  console.log(`\n${colors.bold}${colors.blue}═══════════════════════════════════════════════════════════${colors.reset}`);
  console.log(`${colors.bold}${colors.blue}                    VALIDATION SUMMARY${colors.reset}`);
  console.log(`${colors.bold}${colors.blue}═══════════════════════════════════════════════════════════${colors.reset}\n`);
  
  // Group by platform
  const platforms = ['Electron', 'Go', 'Android'];
  
  platforms.forEach(platform => {
    const platformResults = allResults.filter(r => r.platform === platform);
    if (platformResults.length === 0) return;
    
    console.log(`${colors.bold}${platform}:${colors.reset}`);
    console.log('  Lang    Coverage    Status');
    console.log('  ────────────────────────────');
    
    platformResults.forEach(result => {
      const coverageColor = result.coverage >= MIN_COVERAGE ? colors.green : colors.red;
      const statusIcon = result.coverage >= MIN_COVERAGE ? '✓' : '✗';
      const issues = result.missingKeys.length + result.extraKeys.length;
      const issueText = issues > 0 ? ` (${issues} issues)` : '';
      
      console.log(`  ${result.lang}     ${coverageColor}${result.coverage}%${colors.reset}      ${statusIcon}${issueText}`);
    });
    console.log('');
  });
  
  // Overall statistics
  const totalValidations = allResults.length;
  const passedValidations = allResults.filter(r => r.coverage >= MIN_COVERAGE).length;
  const failedValidations = totalValidations - passedValidations;
  
  console.log(`${colors.bold}Overall:${colors.reset}`);
  console.log(`  Total validations: ${totalValidations}`);
  console.log(`  ${colors.green}Passed: ${passedValidations}${colors.reset}`);
  if (failedValidations > 0) {
    console.log(`  ${colors.red}Failed: ${failedValidations}${colors.reset}`);
  }
  
  console.log(`\n${colors.bold}${colors.blue}═══════════════════════════════════════════════════════════${colors.reset}\n`);
  
  return failedValidations === 0;
}

/**
 * Main validation function
 */
async function main() {
  console.log(`${colors.bold}${colors.cyan}DNSTT Messenger - Translation Validation${colors.reset}\n`);
  console.log(`Validating translations for ${SUPPORTED_LANGUAGES.length} languages across 3 platforms...\n`);
  
  const allResults = [];
  let hasErrors = false;
  
  // ===== Electron Validation =====
  console.log(`${colors.bold}${colors.blue}Validating Electron translations...${colors.reset}`);
  
  const electronBasePath = path.join(ELECTRON_LOCALES_DIR, `${BASE_LANGUAGE}.json`);
  const electronBase = loadJsonTranslations(electronBasePath);
  
  if (!electronBase) {
    console.error(`${colors.red}Failed to load Electron base translations${colors.reset}`);
    hasErrors = true;
  } else {
    const electronBaseKeys = Object.keys(electronBase);
    
    for (const lang of SUPPORTED_LANGUAGES) {
      if (lang === BASE_LANGUAGE) continue;
      
      const filePath = path.join(ELECTRON_LOCALES_DIR, `${lang}.json`);
      if (!fs.existsSync(filePath)) {
        console.error(`${colors.red}Missing file: ${filePath}${colors.reset}`);
        hasErrors = true;
        continue;
      }
      
      const translations = loadJsonTranslations(filePath);
      if (translations) {
        const results = validateTranslations(electronBaseKeys, translations, lang, 'Electron');
        allResults.push(results);
        printResults(results);
      }
    }
  }
  
  // ===== Go Validation =====
  console.log(`\n${colors.bold}${colors.blue}Validating Go translations...${colors.reset}`);
  
  const goBasePath = path.join(GO_LOCALES_DIR, `${BASE_LANGUAGE}.json`);
  const goBase = loadJsonTranslations(goBasePath);
  
  if (!goBase) {
    console.error(`${colors.red}Failed to load Go base translations${colors.reset}`);
    hasErrors = true;
  } else {
    const goBaseKeys = Object.keys(goBase);
    
    for (const lang of SUPPORTED_LANGUAGES) {
      if (lang === BASE_LANGUAGE) continue;
      
      const filePath = path.join(GO_LOCALES_DIR, `${lang}.json`);
      if (!fs.existsSync(filePath)) {
        console.error(`${colors.red}Missing file: ${filePath}${colors.reset}`);
        hasErrors = true;
        continue;
      }
      
      const translations = loadJsonTranslations(filePath);
      if (translations) {
        const results = validateTranslations(goBaseKeys, translations, lang, 'Go');
        allResults.push(results);
        printResults(results);
      }
    }
  }
  
  // ===== Android Validation =====
  console.log(`\n${colors.bold}${colors.blue}Validating Android translations...${colors.reset}`);
  
  const androidBasePath = getAndroidStringsPath(BASE_LANGUAGE);
  const androidBase = await loadAndroidStrings(androidBasePath);
  
  if (!androidBase) {
    console.error(`${colors.red}Failed to load Android base translations${colors.reset}`);
    hasErrors = true;
  } else {
    const androidBaseKeys = Object.keys(androidBase);
    
    for (const lang of SUPPORTED_LANGUAGES) {
      if (lang === BASE_LANGUAGE) continue;
      
      const filePath = getAndroidStringsPath(lang);
      if (!fs.existsSync(filePath)) {
        console.error(`${colors.red}Missing file: ${filePath}${colors.reset}`);
        hasErrors = true;
        continue;
      }
      
      const translations = await loadAndroidStrings(filePath);
      if (translations) {
        const results = validateTranslations(androidBaseKeys, translations, lang, 'Android');
        allResults.push(results);
        printResults(results);
      }
    }
  }
  
  // ===== Print Summary =====
  const allPassed = printSummary(allResults);
  
  // Exit with appropriate code
  if (hasErrors || !allPassed) {
    console.log(`${colors.red}${colors.bold}Validation failed!${colors.reset}`);
    process.exit(1);
  } else {
    console.log(`${colors.green}${colors.bold}All validations passed!${colors.reset}`);
    process.exit(0);
  }
}

// Run the validation
main().catch(error => {
  console.error(`${colors.red}Unexpected error: ${error.message}${colors.reset}`);
  console.error(error.stack);
  process.exit(1);
});
