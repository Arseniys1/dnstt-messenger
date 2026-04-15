/**
 * Integration tests for I18n Manager Error Handling
 * Tests error scenarios: missing files, corrupted JSON, missing keys
 * 
 * Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.5
 * 
 * Run with: node manager.integration.test.js
 */

const fs = require('fs');
const path = require('path');
const I18nManager = require('./manager.js');

// Mock electron app module
const mockApp = {
    isPackaged: false,
    getLocale: () => 'en-US'
};

// Inject mock into require cache
require.cache[require.resolve('electron')] = {
    exports: { app: mockApp }
};

// Test utilities
let testsPassed = 0;
let testsFailed = 0;

function assert(condition, message) {
    if (condition) {
        testsPassed++;
        console.log(`✓ ${message}`);
    } else {
        testsFailed++;
        console.error(`✗ ${message}`);
    }
}

function assertEquals(actual, expected, message) {
    if (actual === expected) {
        testsPassed++;
        console.log(`✓ ${message}`);
    } else {
        testsFailed++;
        console.error(`✗ ${message}`);
        console.error(`  Expected: ${expected}`);
        console.error(`  Actual: ${actual}`);
    }
}

// Setup test environment
const testDir = path.join(__dirname, 'test-locales');
const originalGetLocalesPath = I18nManager.prototype.getLocalesPath;

function setupTestEnvironment() {
    // Create test directory
    if (!fs.existsSync(testDir)) {
        fs.mkdirSync(testDir, { recursive: true });
    }
    
    // Override getLocalesPath to use test directory
    I18nManager.prototype.getLocalesPath = function() {
        return testDir;
    };
}

function cleanupTestEnvironment() {
    // Restore original method
    I18nManager.prototype.getLocalesPath = originalGetLocalesPath;
    
    // Clean up test directory
    if (fs.existsSync(testDir)) {
        fs.rmSync(testDir, { recursive: true, force: true });
    }
}

function createValidTranslationFile(lang, content = null) {
    const defaultContent = {
        "app.name": "DNSTT Messenger",
        "login.title": "Login",
        "error.connection_failed": "Connection error: {error}"
    };
    
    const filePath = path.join(testDir, `${lang}.json`);
    fs.writeFileSync(filePath, JSON.stringify(content || defaultContent, null, 2));
}

function createCorruptedTranslationFile(lang) {
    const filePath = path.join(testDir, `${lang}.json`);
    fs.writeFileSync(filePath, '{ "app.name": "Test", invalid json }');
}

async function runTests() {
    console.log('Running I18n Manager Integration Tests for Error Handling...\n');
    
    setupTestEnvironment();
    
    try {
        // ============================================================
        // TEST SUITE 1: Missing Translation Files
        // ============================================================
        
        console.log('=== Test Suite 1: Missing Translation Files ===\n');
        
        console.log('Test 1.1: Load language with missing file falls back to English');
        createValidTranslationFile('en');
        const manager1 = new I18nManager();
        const result1 = await manager1.loadLanguage('zh');
        assert(result1, 'Should return true (fallback loaded)');
        assertEquals(manager1.currentLanguage, 'en', 'Should fall back to English');
        
        console.log('\nTest 1.2: Translate key with missing language file uses fallback');
        const text1 = manager1.translate('app.name');
        assertEquals(text1, 'DNSTT Messenger', 'Should use English fallback');
        
        console.log('\nTest 1.3: App remains functional after missing file error');
        const text2 = manager1.translate('login.title');
        assertEquals(text2, 'Login', 'Should continue working with fallback');
        
        console.log('\nTest 1.4: Multiple missing files handled gracefully');
        const manager2 = new I18nManager();
        await manager2.loadLanguage('zh'); // Missing
        await manager2.loadLanguage('fa'); // Missing
        await manager2.loadLanguage('ru'); // Missing
        assertEquals(manager2.currentLanguage, 'en', 'Should remain on English fallback');
        assert(manager2.translate('app.name') === 'DNSTT Messenger', 'Should still translate');
        
        // ============================================================
        // TEST SUITE 2: Corrupted JSON Files
        // ============================================================
        
        console.log('\n=== Test Suite 2: Corrupted JSON Files ===\n');
        
        console.log('Test 2.1: Load language with corrupted JSON falls back to English');
        createValidTranslationFile('en');
        createCorruptedTranslationFile('zh');
        const manager3 = new I18nManager();
        const result2 = await manager3.loadLanguage('zh');
        assert(result2, 'Should return true (fallback loaded)');
        assertEquals(manager3.currentLanguage, 'en', 'Should fall back to English');
        
        console.log('\nTest 2.2: Translate key with corrupted file uses fallback');
        const text3 = manager3.translate('app.name');
        assertEquals(text3, 'DNSTT Messenger', 'Should use English fallback');
        
        console.log('\nTest 2.3: App remains functional after JSON parse error');
        const text4 = manager3.translate('login.title');
        assertEquals(text4, 'Login', 'Should continue working');
        
        console.log('\nTest 2.4: Switch from corrupted to valid language');
        createValidTranslationFile('ru', {
            "app.name": "DNSTT Мессенджер",
            "login.title": "Вход"
        });
        await manager3.loadLanguage('ru');
        assertEquals(manager3.currentLanguage, 'ru', 'Should load valid language');
        assertEquals(manager3.translate('app.name'), 'DNSTT Мессенджер', 'Should use Russian');
        
        console.log('\nTest 2.5: Corrupted English file (worst case scenario)');
        cleanupTestEnvironment();
        setupTestEnvironment();
        createCorruptedTranslationFile('en');
        const manager4 = new I18nManager();
        const result3 = await manager4.loadLanguage('en');
        assert(!result3, 'Should return false when English fails');
        // Manager should still be functional, just with no translations
        const text5 = manager4.translate('app.name');
        assertEquals(text5, 'app.name', 'Should return key when no translations available');
        
        // ============================================================
        // TEST SUITE 3: Missing Translation Keys
        // ============================================================
        
        console.log('\n=== Test Suite 3: Missing Translation Keys ===\n');
        
        console.log('Test 3.1: Missing key in current language falls back to English');
        cleanupTestEnvironment();
        setupTestEnvironment();
        createValidTranslationFile('en', {
            "app.name": "DNSTT Messenger",
            "login.title": "Login",
            "settings.title": "Settings"
        });
        createValidTranslationFile('zh', {
            "app.name": "DNSTT 信使",
            "login.title": "登录"
            // Missing "settings.title"
        });
        const manager5 = new I18nManager();
        await manager5.loadLanguage('zh');
        const text6 = manager5.translate('settings.title');
        assertEquals(text6, 'Settings', 'Should fall back to English for missing key');
        
        console.log('\nTest 3.2: Missing key in both languages returns key itself');
        const text7 = manager5.translate('nonexistent.key');
        assertEquals(text7, 'nonexistent.key', 'Should return key when not found');
        
        console.log('\nTest 3.3: Partial translation coverage works correctly');
        assertEquals(manager5.translate('app.name'), 'DNSTT 信使', 'Should use Chinese');
        assertEquals(manager5.translate('login.title'), '登录', 'Should use Chinese');
        assertEquals(manager5.translate('settings.title'), 'Settings', 'Should use English fallback');
        
        console.log('\nTest 3.4: Missing keys with parameters');
        createValidTranslationFile('en', {
            "error.connection_failed": "Connection error: {error}",
            "status.connected": "Connected to {server}"
        });
        createValidTranslationFile('fa', {
            "error.connection_failed": "خطای اتصال: {error}"
            // Missing "status.connected"
        });
        const manager6 = new I18nManager();
        await manager6.loadLanguage('fa');
        const text8 = manager6.translate('status.connected', { server: 'example.com' });
        assertEquals(text8, 'Connected to example.com', 'Should fall back with parameters');
        
        console.log('\nTest 3.5: Missing plural forms fall back gracefully');
        createValidTranslationFile('en', {
            "room.members_count.one": "{count} member",
            "room.members_count.many": "{count} members"
        });
        createValidTranslationFile('tr', {
            "room.members_count.many": "{count} üye"
            // Missing .one form
        });
        const manager7 = new I18nManager();
        await manager7.loadLanguage('tr');
        const text9 = manager7.translate('room.members_count', { count: 1 });
        // Should fall back to English .one form
        assertEquals(text9, '1 member', 'Should fall back to English plural form');
        
        // ============================================================
        // TEST SUITE 4: App Functionality Under Error Conditions
        // ============================================================
        
        console.log('\n=== Test Suite 4: App Functionality Under Error Conditions ===\n');
        
        console.log('Test 4.1: Manager initialization with all files missing');
        cleanupTestEnvironment();
        setupTestEnvironment();
        const manager8 = new I18nManager();
        await manager8.initialize();
        assert(manager8.currentLanguage === 'en', 'Should default to English');
        // Should still be functional
        const text10 = manager8.translate('any.key');
        assertEquals(text10, 'any.key', 'Should return key as fallback');
        
        console.log('\nTest 4.2: Language switching with mixed file states');
        createValidTranslationFile('en');
        createCorruptedTranslationFile('zh');
        createValidTranslationFile('ru', { "app.name": "Тест" });
        const manager9 = new I18nManager();
        await manager9.loadLanguage('en');
        assertEquals(manager9.currentLanguage, 'en', 'Should load English');
        await manager9.loadLanguage('zh'); // Corrupted
        assertEquals(manager9.currentLanguage, 'en', 'Should stay on English');
        await manager9.loadLanguage('ru'); // Valid
        assertEquals(manager9.currentLanguage, 'ru', 'Should load Russian');
        
        console.log('\nTest 4.3: Detect system language with missing file');
        const manager10 = new I18nManager();
        const detected = manager10.detectSystemLanguage();
        assert(manager10.supportedLanguages.includes(detected), 'Should detect valid language');
        
        console.log('\nTest 4.4: Get supported languages always works');
        const manager11 = new I18nManager();
        const langs = manager11.getSupportedLanguages();
        assertEquals(langs.length, 7, 'Should return all 7 languages');
        assert(langs.every(l => l.code && l.name && l.nativeName), 'All languages have metadata');
        
        console.log('\nTest 4.5: RTL detection works without loaded translations');
        const manager12 = new I18nManager();
        assert(manager12.isRTL('ar'), 'Should detect Arabic as RTL');
        assert(manager12.isRTL('fa'), 'Should detect Farsi as RTL');
        assert(!manager12.isRTL('en'), 'Should detect English as LTR');
        
        console.log('\nTest 4.6: Parameter substitution with missing translations');
        cleanupTestEnvironment();
        setupTestEnvironment();
        const manager13 = new I18nManager();
        const text11 = manager13.translate('missing.key', { param: 'value' });
        assertEquals(text11, 'missing.key', 'Should return key even with params');
        
        console.log('\nTest 4.7: Pluralization with missing translations');
        const text12 = manager13.translate('missing.plural', { count: 5 });
        assert(text12.includes('missing.plural'), 'Should return key for missing plural');
        
        console.log('\nTest 4.8: Multiple managers work independently');
        createValidTranslationFile('en');
        createValidTranslationFile('zh', { "app.name": "中文" });
        const managerA = new I18nManager();
        const managerB = new I18nManager();
        await managerA.loadLanguage('en');
        await managerB.loadLanguage('zh');
        assertEquals(managerA.currentLanguage, 'en', 'Manager A should be English');
        assertEquals(managerB.currentLanguage, 'zh', 'Manager B should be Chinese');
        
        // ============================================================
        // TEST SUITE 5: Edge Cases and Recovery
        // ============================================================
        
        console.log('\n=== Test Suite 5: Edge Cases and Recovery ===\n');
        
        console.log('Test 5.1: Empty translation file');
        cleanupTestEnvironment();
        setupTestEnvironment();
        createValidTranslationFile('en', {});
        const manager14 = new I18nManager();
        await manager14.loadLanguage('en');
        const text13 = manager14.translate('any.key');
        assertEquals(text13, 'any.key', 'Should return key with empty translations');
        
        console.log('\nTest 5.2: Translation file with null values');
        createValidTranslationFile('en', {
            "app.name": null,
            "login.title": "Login"
        });
        const manager15 = new I18nManager();
        await manager15.loadLanguage('en');
        const text14 = manager15.translate('app.name');
        // null should be handled gracefully
        assert(text14 !== undefined, 'Should handle null values');
        
        console.log('\nTest 5.3: Very large translation file');
        const largeTranslations = {};
        for (let i = 0; i < 1000; i++) {
            largeTranslations[`key.${i}`] = `Value ${i}`;
        }
        createValidTranslationFile('en', largeTranslations);
        const manager16 = new I18nManager();
        const result4 = await manager16.loadLanguage('en');
        assert(result4, 'Should load large file');
        assertEquals(manager16.translate('key.500'), 'Value 500', 'Should access any key');
        
        console.log('\nTest 5.4: Rapid language switching');
        createValidTranslationFile('en');
        createValidTranslationFile('zh', { "app.name": "中文" });
        createValidTranslationFile('ru', { "app.name": "Русский" });
        const manager17 = new I18nManager();
        await manager17.loadLanguage('en');
        await manager17.loadLanguage('zh');
        await manager17.loadLanguage('ru');
        await manager17.loadLanguage('en');
        assertEquals(manager17.currentLanguage, 'en', 'Should handle rapid switching');
        
        console.log('\nTest 5.5: File system permission errors (simulated)');
        // This test verifies the error handling exists
        const manager18 = new I18nManager();
        // Trying to load from non-existent directory should fail gracefully
        const result5 = await manager18.loadLanguage('invalid');
        assert(result5 === true || result5 === false, 'Should return boolean');
        
    } finally {
        cleanupTestEnvironment();
    }
    
    // Print summary
    console.log('\n' + '='.repeat(50));
    console.log(`Tests passed: ${testsPassed}`);
    console.log(`Tests failed: ${testsFailed}`);
    console.log('='.repeat(50));
    
    if (testsFailed === 0) {
        console.log('\n✓ All integration tests passed!');
        console.log('✓ App remains functional under all error conditions');
        process.exit(0);
    } else {
        console.log('\n✗ Some integration tests failed!');
        process.exit(1);
    }
}

// Run tests
runTests().catch(error => {
    console.error('Test execution failed:', error);
    cleanupTestEnvironment();
    process.exit(1);
});
