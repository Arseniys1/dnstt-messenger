/**
 * I18n Manager for Electron Client
 * Handles translation loading, language detection, and text translation with fallback support
 */

const fs = require('fs');
const path = require('path');
const { app } = require('electron');

class I18nManager {
    constructor() {
        this.currentLanguage = 'en';
        this.translations = {};
        this.fallbackLanguage = 'en';
        this.fallbackTranslations = {};
        this.supportedLanguages = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi'];
        this.rtlLanguages = ['ar', 'fa'];
    }

    /**
     * Get the path to the locales directory
     * @returns {string} Path to locales directory
     */
    getLocalesPath() {
        // In development, use the source directory
        // In production, use the app resources directory
        if (app && app.isPackaged) {
            return path.join(process.resourcesPath, 'i18n', 'locales');
        }
        return path.join(__dirname, 'locales');
    }

    /**
     * Load a language file
     * @param {string} languageCode - ISO 639-1 language code
     * @returns {Promise<boolean>} True if loaded successfully, false otherwise
     */
    async loadLanguage(languageCode) {
        // Validate language code
        if (!this.supportedLanguages.includes(languageCode)) {
            console.warn(`Unsupported language code: ${languageCode}. Falling back to ${this.fallbackLanguage}`);
            languageCode = this.fallbackLanguage;
        }

        try {
            const localesPath = this.getLocalesPath();
            const filePath = path.join(localesPath, `${languageCode}.json`);
            
            // Check if file exists
            if (!fs.existsSync(filePath)) {
                console.error(`Translation file not found: ${filePath}`);
                if (languageCode !== this.fallbackLanguage) {
                    return this.loadLanguage(this.fallbackLanguage);
                }
                return false;
            }

            // Read and parse the translation file
            const fileContent = fs.readFileSync(filePath, 'utf-8');
            const translations = JSON.parse(fileContent);

            // Store translations
            if (languageCode === this.fallbackLanguage) {
                this.fallbackTranslations = translations;
            }
            
            this.translations = translations;
            this.currentLanguage = languageCode;

            // Load fallback if not already loaded
            if (languageCode !== this.fallbackLanguage && Object.keys(this.fallbackTranslations).length === 0) {
                const fallbackPath = path.join(localesPath, `${this.fallbackLanguage}.json`);
                if (fs.existsSync(fallbackPath)) {
                    const fallbackContent = fs.readFileSync(fallbackPath, 'utf-8');
                    this.fallbackTranslations = JSON.parse(fallbackContent);
                }
            }

            console.log(`Language loaded: ${languageCode}`);
            return true;
        } catch (error) {
            console.error(`Failed to load language ${languageCode}:`, error.message);
            
            // If loading failed and it's not the fallback language, try fallback
            if (languageCode !== this.fallbackLanguage) {
                console.log(`Attempting to load fallback language: ${this.fallbackLanguage}`);
                return this.loadLanguage(this.fallbackLanguage);
            }
            
            return false;
        }
    }

    /**
     * Translate a key with optional parameter substitution and pluralization
     * @param {string} key - Translation key (e.g., "login.title")
     * @param {Object} params - Parameters for substitution (e.g., {username: "John", count: 5})
     * @returns {string} Translated text or key if not found
     */
    translate(key, params = {}) {
        // Check if this is a pluralization request (params contains 'count')
        if (params && params.count !== undefined) {
            const pluralKey = this.getPluralKey(key, params.count);
            let text = this.translations[pluralKey];

            // Fall back to fallback language if not found
            if (text === undefined && this.currentLanguage !== this.fallbackLanguage) {
                text = this.fallbackTranslations[pluralKey];
            }

            // If plural form not found, try the base key
            if (text === undefined) {
                text = this.translations[key];
                if (text === undefined && this.currentLanguage !== this.fallbackLanguage) {
                    text = this.fallbackTranslations[key];
                }
            }

            // If still not found, return the key itself
            if (text === undefined) {
                console.warn(`Translation key not found: ${key}`);
                return key;
            }

            // Substitute parameters
            return this.substituteParams(text, params);
        }

        // Non-plural translation
        let text = this.translations[key];

        // Fall back to fallback language if not found
        if (text === undefined && this.currentLanguage !== this.fallbackLanguage) {
            text = this.fallbackTranslations[key];
        }

        // If still not found, return the key itself
        if (text === undefined) {
            console.warn(`Translation key not found: ${key}`);
            return key;
        }

        // Substitute parameters (handle null/undefined params)
        if (params && Object.keys(params).length > 0) {
            text = this.substituteParams(text, params);
        }

        return text;
    }

    /**
     * Substitute parameters in a translation string
     * @param {string} text - Text with placeholders like {username}
     * @param {Object} params - Parameters to substitute
     * @returns {string} Text with substituted parameters
     */
    substituteParams(text, params) {
        return text.replace(/\{(\w+)\}/g, (match, key) => {
            return params[key] !== undefined ? params[key] : match;
        });
    }

    /**
     * Get the appropriate plural key based on count and language rules
     * @param {string} baseKey - Base translation key
     * @param {number} count - The count to determine plural form
     * @returns {string} The plural key to use
     */
    getPluralKey(baseKey, count) {
        const pluralForm = this.getPluralForm(count);
        
        // Try language-specific plural forms: key.zero, key.one, key.many
        if (pluralForm === 'zero') {
            return `${baseKey}.zero`;
        } else if (pluralForm === 'one') {
            return `${baseKey}.one`;
        } else {
            return `${baseKey}.many`;
        }
    }

    /**
     * Determine plural form based on count and current language rules
     * @param {number} count - The count value
     * @returns {string} 'zero', 'one', or 'many'
     */
    getPluralForm(count) {
        const n = Math.abs(count);
        
        // Language-specific plural rules
        switch (this.currentLanguage) {
            case 'en':
            case 'vi':
            case 'tr':
                // English, Vietnamese, Turkish: one (n=1), many (n!=1)
                return n === 1 ? 'one' : 'many';
            
            case 'zh':
                // Chinese: no plural distinction, always use 'many'
                return 'many';
            
            case 'ru':
                // Russian: complex rules
                // one: n%10=1 and n%100!=11
                // few: n%10 in 2..4 and n%100 not in 12..14
                // many: otherwise
                if (n % 10 === 1 && n % 100 !== 11) {
                    return 'one';
                }
                return 'many';
            
            case 'ar':
                // Arabic: complex rules
                // zero: n=0
                // one: n=1
                // two: n=2
                // few: n%100 in 3..10
                // many: n%100 in 11..99
                // other: otherwise
                if (n === 0) return 'zero';
                if (n === 1) return 'one';
                return 'many';
            
            case 'fa':
                // Farsi: one (n=0 or n=1), many (n>1)
                return (n === 0 || n === 1) ? 'one' : 'many';
            
            default:
                // Default to English rules
                return n === 1 ? 'one' : 'many';
        }
    }

    /**
     * Detect the system language
     * @returns {string} Detected language code or fallback language
     */
    detectSystemLanguage() {
        try {
            // Get system locale from Electron app
            let locale = 'en';
            
            if (app) {
                locale = app.getLocale();
            } else if (typeof navigator !== 'undefined') {
                // Fallback for renderer process
                locale = navigator.language || navigator.userLanguage;
            }

            // Extract language code (e.g., "en-US" -> "en")
            const languageCode = locale.split('-')[0].toLowerCase();

            // Check if the detected language is supported
            if (this.supportedLanguages.includes(languageCode)) {
                return languageCode;
            }

            console.log(`System language ${languageCode} not supported, using ${this.fallbackLanguage}`);
            return this.fallbackLanguage;
        } catch (error) {
            console.error('Failed to detect system language:', error.message);
            return this.fallbackLanguage;
        }
    }

    /**
     * Set the current language
     * @param {string} languageCode - ISO 639-1 language code
     * @returns {Promise<boolean>} True if set successfully
     */
    async setLanguage(languageCode) {
        return this.loadLanguage(languageCode);
    }

    /**
     * Get the current language code
     * @returns {string} Current language code
     */
    getCurrentLanguage() {
        return this.currentLanguage;
    }

    /**
     * Get list of supported languages
     * @returns {Array<Object>} Array of language objects with code, name, and nativeName
     */
    getSupportedLanguages() {
        return [
            { code: 'en', name: 'English', nativeName: 'English', rtl: false },
            { code: 'zh', name: 'Chinese', nativeName: '中文', rtl: false },
            { code: 'fa', name: 'Farsi', nativeName: 'فارسی', rtl: true },
            { code: 'ru', name: 'Russian', nativeName: 'Русский', rtl: false },
            { code: 'ar', name: 'Arabic', nativeName: 'العربية', rtl: true },
            { code: 'tr', name: 'Turkish', nativeName: 'Türkçe', rtl: false },
            { code: 'vi', name: 'Vietnamese', nativeName: 'Tiếng Việt', rtl: false }
        ];
    }

    /**
     * Check if a language is RTL (right-to-left)
     * @param {string} languageCode - Language code to check
     * @returns {boolean} True if RTL language
     */
    isRTL(languageCode = null) {
        const code = languageCode || this.currentLanguage;
        return this.rtlLanguages.includes(code);
    }

    /**
     * Initialize the I18n manager with system language or saved preference
     * @param {string|null} savedLanguage - Previously saved language preference
     * @returns {Promise<boolean>} True if initialized successfully
     */
    async initialize(savedLanguage = null) {
        let languageToLoad = savedLanguage;

        // If no saved language, detect system language
        if (!languageToLoad) {
            languageToLoad = this.detectSystemLanguage();
        }

        return this.loadLanguage(languageToLoad);
    }
}

module.exports = I18nManager;
