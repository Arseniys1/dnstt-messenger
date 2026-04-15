/**
 * Performance tests for I18nManager language switching
 * Validates: Requirement 10.3 - Language switching must complete within 500ms
 * 
 * Run with: node manager.performance.test.js
 */

const I18nManager = require('./manager');
const fs = require('fs');
const path = require('path');

// Mock electron app module
const mockApp = {
    getLocale: () => 'en-US',
    isPackaged: false
};

// Override require for electron module
const Module = require('module');
const originalRequire = Module.prototype.require;
Module.prototype.require = function(id) {
    if (id === 'electron') {
        return { app: mockApp };
    }
    return originalRequire.apply(this, arguments);
};

// Test utilities
function assertEquals(actual, expected, message) {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${expected}, got ${actual}`);
    }
}

function assertLessThan(actual, expected, message) {
    if (actual >= expected) {
        throw new Error(`${message}: expected < ${expected}, got ${actual}`);
    }
}

// Test setup
const testLocalesPath = path.join(__dirname, 'test-locales-perf');

function setupTestEnvironment() {
    // Create test locales directory
    if (!fs.existsSync(testLocalesPath)) {
        fs.mkdirSync(testLocalesPath, { recursive: true });
    }

    // Create test translation files for all supported languages
    const languages = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi'];
    const testTranslations = {
        'app.name': 'Test App',
        'login.title': 'Login',
        'chat.title': 'Chat',
        'settings.title': 'Settings',
        'chat.online_count': '{count} online',
        'chat.online_count.one': '{count} user online',
        'chat.online_count.many': '{count} users online'
    };

    languages.forEach(lang => {
        const filePath = path.join(testLocalesPath, `${lang}.json`);
        fs.writeFileSync(filePath, JSON.stringify(testTranslations, null, 2));
    });
}

function cleanupTestEnvironment() {
    // Clean up test files
    if (fs.existsSync(testLocalesPath)) {
        fs.readdirSync(testLocalesPath).forEach(file => {
            fs.unlinkSync(path.join(testLocalesPath, file));
        });
        fs.rmdirSync(testLocalesPath);
    }
}

// Performance tests
async function runPerformanceTests() {
    console.log('=== I18nManager Performance Tests ===\n');
    
    let testsPassed = 0;
    let testsFailed = 0;

    try {
        setupTestEnvironment();

        // Test 1: Language switching completes within 500ms
        console.log('Test 1: Language switching completes within 500ms');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            const languages = ['en', 'zh', 'fa', 'ru', 'ar', 'tr', 'vi'];
            const results = [];

            // Warm up - first load might be slower
            await manager.loadLanguage('en');

            // Measure each language switch
            for (let i = 0; i < languages.length - 1; i++) {
                const fromLang = languages[i];
                const toLang = languages[i + 1];

                // Set initial language
                await manager.loadLanguage(fromLang);

                // Measure switch time
                const startTime = performance.now();
                await manager.setLanguage(toLang);
                const switchTime = performance.now() - startTime;

                results.push({ transition: `${fromLang} -> ${toLang}`, time: switchTime });

                // Verify requirement: < 500ms
                assertLessThan(switchTime, 500, `Switch from ${fromLang} to ${toLang}`);
            }

            // Print results for analysis
            console.log('\n  Results:');
            results.forEach(({ transition, time }) => {
                console.log(`  ${transition}: ${time.toFixed(2)}ms`);
            });

            const avgTime = results.reduce((sum, r) => sum + r.time, 0) / results.length;
            const maxTime = Math.max(...results.map(r => r.time));

            console.log(`\n  Average switch time: ${avgTime.toFixed(2)}ms`);
            console.log(`  Maximum switch time: ${maxTime.toFixed(2)}ms`);
            console.log(`  Requirement: < 500ms`);
            console.log(`  Status: ${maxTime < 500 ? 'PASS' : 'FAIL'}\n`);

            assertLessThan(maxTime, 500, 'Maximum switch time');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

        // Test 2: Rapid consecutive language switches
        console.log('Test 2: Rapid consecutive language switches complete within 500ms each');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            const languages = ['en', 'zh', 'ru', 'ar', 'en'];
            const results = [];

            // Perform rapid switches
            for (let i = 0; i < languages.length - 1; i++) {
                const startTime = performance.now();
                await manager.setLanguage(languages[i + 1]);
                const switchTime = performance.now() - startTime;

                results.push(switchTime);
                assertLessThan(switchTime, 500, `Rapid switch to ${languages[i + 1]}`);
            }

            console.log('\n  Results:');
            results.forEach((time, index) => {
                console.log(`  Switch ${index + 1}: ${time.toFixed(2)}ms`);
            });
            console.log('  Status: PASS\n');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

        // Test 3: Language switch with translation lookup
        console.log('Test 3: Language switch with translation lookup completes within 500ms');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            await manager.loadLanguage('en');

            const startTime = performance.now();
            await manager.setLanguage('zh');
            // Perform some translations immediately after switch
            manager.translate('app.name');
            manager.translate('login.title');
            manager.translate('chat.title');
            const totalTime = performance.now() - startTime;

            console.log(`\n  Time: ${totalTime.toFixed(2)}ms`);
            console.log(`  Requirement: < 500ms`);
            console.log(`  Status: ${totalTime < 500 ? 'PASS' : 'FAIL'}\n`);

            assertLessThan(totalTime, 500, 'Language switch with translation');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

        // Test 4: RTL language switch
        console.log('Test 4: RTL language switch completes within 500ms');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            const rtlLanguages = ['ar', 'fa'];
            const results = [];

            for (const lang of rtlLanguages) {
                const startTime = performance.now();
                await manager.setLanguage(lang);
                const switchTime = performance.now() - startTime;

                results.push({ lang, time: switchTime });
                assertLessThan(switchTime, 500, `RTL switch to ${lang}`);
            }

            console.log('\n  Results:');
            results.forEach(({ lang, time }) => {
                console.log(`  ${lang}: ${time.toFixed(2)}ms`);
            });
            console.log('  Status: PASS\n');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

        // Test 5: Translation lookup has no file I/O overhead
        console.log('Test 5: Translation lookup has no file I/O overhead');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            await manager.loadLanguage('en');

            // Measure translation lookup time (should be pure memory access)
            const iterations = 1000;
            const startTime = performance.now();
            
            for (let i = 0; i < iterations; i++) {
                manager.translate('app.name');
                manager.translate('login.title');
                manager.translate('chat.title');
            }
            
            const totalTime = performance.now() - startTime;
            const avgTime = totalTime / iterations;

            console.log(`\n  ${iterations} iterations: ${totalTime.toFixed(2)}ms`);
            console.log(`  Average per lookup: ${avgTime.toFixed(4)}ms`);
            console.log(`  Expected: < 1ms per lookup (memory access only)`);
            console.log(`  Status: ${avgTime < 1 ? 'PASS' : 'FAIL'}\n`);

            // Each lookup should be extremely fast (< 1ms) since it's just memory access
            assertLessThan(avgTime, 1, 'Average translation lookup time');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

        // Test 6: Language switching with fallback
        console.log('Test 6: Language switching with fallback completes within 500ms');
        try {
            const manager = new I18nManager();
            manager.getLocalesPath = () => testLocalesPath;
            
            await manager.loadLanguage('en');

            // Switch to a language and test fallback behavior
            const startTime = performance.now();
            await manager.setLanguage('zh');
            // Try to translate a key that might not exist (tests fallback)
            manager.translate('nonexistent.key');
            const totalTime = performance.now() - startTime;

            console.log(`\n  Time: ${totalTime.toFixed(2)}ms`);
            console.log(`  Status: ${totalTime < 500 ? 'PASS' : 'FAIL'}\n`);

            assertLessThan(totalTime, 500, 'Language switch with fallback');
            testsPassed++;
        } catch (error) {
            console.error(`  FAILED: ${error.message}\n`);
            testsFailed++;
        }

    } finally {
        cleanupTestEnvironment();
    }

    // Summary
    console.log('=== Test Summary ===');
    console.log(`Tests passed: ${testsPassed}`);
    console.log(`Tests failed: ${testsFailed}`);
    console.log(`Total tests: ${testsPassed + testsFailed}`);
    
    if (testsFailed > 0) {
        process.exit(1);
    }
}

// Run tests
runPerformanceTests().catch(error => {
    console.error('Test execution failed:', error);
    process.exit(1);
});
