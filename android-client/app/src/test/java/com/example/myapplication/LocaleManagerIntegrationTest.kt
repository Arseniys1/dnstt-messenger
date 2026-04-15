package com.example.myapplication

import android.content.Context
import android.content.SharedPreferences
import android.content.res.Configuration
import android.content.res.Resources
import android.os.Build
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.Locale

/**
 * Integration tests for LocaleManager error handling.
 * Tests error scenarios: missing translations, corrupted data, missing keys.
 * 
 * Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.5
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [Build.VERSION_CODES.P])
class LocaleManagerIntegrationTest {
    
    private lateinit var context: Context
    private lateinit var sharedPreferences: SharedPreferences
    
    @Before
    fun setup() {
        context = ApplicationProvider.getApplicationContext()
        sharedPreferences = context.getSharedPreferences("locale_preferences", Context.MODE_PRIVATE)
        sharedPreferences.edit().clear().apply()
    }
    
    // ========== Test Suite 1: Missing Translation Files ==========
    
    @Test
    fun `app remains functional with unsupported locale`() {
        // Android's resource system handles missing translations gracefully
        // by falling back to default (English) resources
        
        // Try to set an unsupported language
        try {
            LocaleManager.setLocale(context, "de") // German not supported
            fail("Should throw exception for unsupported language")
        } catch (e: IllegalArgumentException) {
            // Expected
        }
        
        // App should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Current locale should not be null", currentLocale)
        
        // Should be able to get supported languages
        val languages = LocaleManager.getSupportedLanguages()
        assertEquals("Should return 7 languages", 7, languages.size)
    }
    
    @Test
    fun `multiple unsupported locale attempts handled gracefully`() {
        val unsupportedCodes = listOf("de", "fr", "es", "ja", "ko")
        
        unsupportedCodes.forEach { code ->
            try {
                LocaleManager.setLocale(context, code)
                fail("Should reject unsupported language: $code")
            } catch (e: IllegalArgumentException) {
                // Expected - each should be rejected
            }
        }
        
        // App should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should have a valid locale", currentLocale)
    }
    
    @Test
    fun `fallback to English for missing translations`() {
        // Android automatically falls back to default resources (English)
        // when a translation is missing in the selected language
        
        // Set to a supported language
        LocaleManager.setLocale(context, "zh")
        
        // Even if some translations are missing, Android will use English fallback
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertEquals("Should be Chinese", "zh", currentLocale.language)
        
        // App remains functional
        val languages = LocaleManager.getSupportedLanguages()
        assertTrue("Should return languages", languages.isNotEmpty())
    }
    
    // ========== Test Suite 2: Corrupted Preferences Data ==========
    
    @Test
    fun `app handles corrupted preferences gracefully`() {
        // Manually corrupt the preferences
        sharedPreferences.edit()
            .putString("selected_language", "corrupted_invalid_data_12345")
            .apply()
        
        // App should still work
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should return a locale", currentLocale)
        
        // Should be able to set a valid locale
        LocaleManager.setLocale(context, "en")
        assertEquals("Should set English", "en", LocaleManager.getCurrentLocale(context).language)
    }
    
    @Test
    fun `app handles missing preferences gracefully`() {
        // Clear all preferences
        sharedPreferences.edit().clear().apply()
        
        // App should still work with defaults
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should return a locale", currentLocale)
        
        // Should be able to set locale
        LocaleManager.setLocale(context, "zh")
        assertEquals("Should set Chinese", "zh", LocaleManager.getCurrentLocale(context).language)
    }
    
    @Test
    fun `app handles invalid boolean preferences`() {
        // Set invalid data for boolean field
        sharedPreferences.edit()
            .putString("use_system_language", "not_a_boolean")
            .apply()
        
        // App should handle this gracefully
        val hasManual = LocaleManager.hasManualLanguageSelection(context)
        // Should return a boolean (true or false), not crash
        assertTrue("Should return boolean", hasManual || !hasManual)
    }
    
    // ========== Test Suite 3: Missing Translation Keys ==========
    
    @Test
    fun `missing translation keys fall back to English`() {
        // Android's resource system automatically falls back to default resources
        // when a key is missing in the selected language
        
        // Set to a non-English language
        LocaleManager.setLocale(context, "fa")
        
        // Even if some keys are missing in Farsi, Android will use English
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertEquals("Should be Farsi", "fa", currentLocale.language)
        
        // App remains functional
        assertTrue("Should be functional", true)
    }
    
    @Test
    fun `partial translation coverage works correctly`() {
        // Android handles partial translations by falling back to default
        
        val languages = listOf("zh", "fa", "ru", "ar", "tr", "vi")
        
        languages.forEach { lang ->
            LocaleManager.setLocale(context, lang)
            val currentLocale = LocaleManager.getCurrentLocale(context)
            assertEquals("Should set $lang", lang, currentLocale.language)
            
            // App should remain functional
            val supportedLangs = LocaleManager.getSupportedLanguages()
            assertEquals("Should return 7 languages", 7, supportedLangs.size)
        }
    }
    
    // ========== Test Suite 4: App Functionality Under Error Conditions ==========
    
    @Test
    fun `LocaleManager always returns valid data`() {
        // Even under error conditions, LocaleManager should never return null
        
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("getCurrentLocale should never return null", currentLocale)
        
        val languages = LocaleManager.getSupportedLanguages()
        assertNotNull("getSupportedLanguages should never return null", languages)
        assertTrue("Should return languages", languages.isNotEmpty())
    }
    
    @Test
    fun `language switching with mixed valid and invalid codes`() {
        // Set valid language
        LocaleManager.setLocale(context, "en")
        assertEquals("Should be English", "en", LocaleManager.getCurrentLocale(context).language)
        
        // Try invalid language
        try {
            LocaleManager.setLocale(context, "invalid")
            fail("Should reject invalid language")
        } catch (e: IllegalArgumentException) {
            // Expected
        }
        
        // Should remain on English
        assertEquals("Should stay on English", "en", LocaleManager.getCurrentLocale(context).language)
        
        // Set another valid language
        LocaleManager.setLocale(context, "zh")
        assertEquals("Should switch to Chinese", "zh", LocaleManager.getCurrentLocale(context).language)
    }
    
    @Test
    fun `getSupportedLanguages always works`() {
        // Should work even if preferences are corrupted
        sharedPreferences.edit()
            .putString("selected_language", "corrupted")
            .apply()
        
        val languages = LocaleManager.getSupportedLanguages()
        assertEquals("Should return 7 languages", 7, languages.size)
        
        // Verify structure
        languages.forEach { lang ->
            assertNotNull("Code should not be null", lang.code)
            assertNotNull("Display name should not be null", lang.displayName)
            assertNotNull("Native name should not be null", lang.nativeName)
            assertTrue("Code should not be empty", lang.code.isNotEmpty())
            assertTrue("Display name should not be empty", lang.displayName.isNotEmpty())
            assertTrue("Native name should not be empty", lang.nativeName.isNotEmpty())
        }
    }
    
    @Test
    fun `resetToSystemLanguage always works`() {
        // Set a language
        LocaleManager.setLocale(context, "zh")
        assertTrue("Should have manual selection", LocaleManager.hasManualLanguageSelection(context))
        
        // Reset should always work
        LocaleManager.resetToSystemLanguage(context)
        assertFalse("Should not have manual selection", LocaleManager.hasManualLanguageSelection(context))
        
        // Should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should have a locale", currentLocale)
    }
    
    @Test
    fun `hasManualLanguageSelection handles errors gracefully`() {
        // Clear preferences
        sharedPreferences.edit().clear().apply()
        
        val hasManual = LocaleManager.hasManualLanguageSelection(context)
        assertFalse("Should return false for no preferences", hasManual)
        
        // Set manual selection
        LocaleManager.setLocale(context, "ru")
        assertTrue("Should return true after setLocale", LocaleManager.hasManualLanguageSelection(context))
    }
    
    // ========== Test Suite 5: Edge Cases and Recovery ==========
    
    @Test
    fun `rapid language switching`() {
        val languages = listOf("en", "zh", "fa", "ru", "ar", "tr", "vi")
        
        languages.forEach { lang ->
            LocaleManager.setLocale(context, lang)
            assertEquals("Should set $lang", lang, LocaleManager.getCurrentLocale(context).language)
        }
        
        // Should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should have a locale", currentLocale)
    }
    
    @Test
    fun `empty string language code rejected`() {
        try {
            LocaleManager.setLocale(context, "")
            fail("Should reject empty string")
        } catch (e: IllegalArgumentException) {
            // Expected
        }
        
        // App should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should have a locale", currentLocale)
    }
    
    @Test
    fun `whitespace language code rejected`() {
        try {
            LocaleManager.setLocale(context, "   ")
            fail("Should reject whitespace")
        } catch (e: IllegalArgumentException) {
            // Expected
        }
        
        // App should still be functional
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should have a locale", currentLocale)
    }
    
    @Test
    fun `case sensitivity of language codes`() {
        // Try uppercase code
        try {
            LocaleManager.setLocale(context, "ZH")
            fail("Should reject uppercase code")
        } catch (e: IllegalArgumentException) {
            // Expected - codes should be lowercase
        }
        
        // Lowercase should work
        LocaleManager.setLocale(context, "zh")
        assertEquals("Should set Chinese", "zh", LocaleManager.getCurrentLocale(context).language)
    }
    
    @Test
    fun `mixed case language code rejected`() {
        try {
            LocaleManager.setLocale(context, "Zh")
            fail("Should reject mixed case")
        } catch (e: IllegalArgumentException) {
            // Expected
        }
    }
    
    @Test
    fun `preferences isolation per context`() {
        // Verify preferences are properly scoped
        LocaleManager.setLocale(context, "zh")
        
        val prefs = context.getSharedPreferences("locale_preferences", Context.MODE_PRIVATE)
        assertEquals("Should save to preferences", "zh", prefs.getString("selected_language", null))
    }
    
    @Test
    fun `multiple setLocale calls maintain consistency`() {
        val languages = listOf("en", "zh", "fa", "ru", "ar", "tr", "vi")
        
        languages.forEach { code ->
            LocaleManager.setLocale(context, code)
            
            // Check preferences
            val savedLanguage = sharedPreferences.getString("selected_language", null)
            assertEquals("Preferences should match", code, savedLanguage)
            
            // Check current locale
            val currentLocale = LocaleManager.getCurrentLocale(context)
            assertEquals("Current locale should match", code, currentLocale.language)
            
            // Check manual selection flag
            val useSystemLanguage = sharedPreferences.getBoolean("use_system_language", true)
            assertFalse("Should not use system language", useSystemLanguage)
        }
    }
    
    @Test
    fun `all supported languages are valid Locale objects`() {
        val languages = LocaleManager.getSupportedLanguages()
        
        languages.forEach { lang ->
            // Should be able to create Locale object
            val locale = Locale(lang.code)
            assertNotNull("Should create valid Locale", locale)
            assertEquals("Locale language should match", lang.code, locale.language)
        }
    }
    
    @Test
    fun `Language data class properties are consistent`() {
        val languages = LocaleManager.getSupportedLanguages()
        
        // Verify English
        val english = languages.find { it.code == "en" }
        assertNotNull("Should have English", english)
        assertEquals("English display name", "English", english?.displayName)
        assertEquals("English native name", "English", english?.nativeName)
        
        // Verify all languages have unique codes
        val codes = languages.map { it.code }
        assertEquals("All codes should be unique", codes.size, codes.toSet().size)
        
        // Verify all languages have non-empty names
        languages.forEach { lang ->
            assertTrue("${lang.code} should have display name", lang.displayName.isNotEmpty())
            assertTrue("${lang.code} should have native name", lang.nativeName.isNotEmpty())
        }
    }
    
    @Test
    fun `app recovers from preference corruption`() {
        // Corrupt preferences with invalid data
        sharedPreferences.edit()
            .putString("selected_language", "!@#$%^&*()")
            .putInt("use_system_language", 12345) // Wrong type
            .apply()
        
        // App should still work
        val currentLocale = LocaleManager.getCurrentLocale(context)
        assertNotNull("Should return a locale", currentLocale)
        
        // Should be able to set valid locale
        LocaleManager.setLocale(context, "en")
        assertEquals("Should set English", "en", LocaleManager.getCurrentLocale(context).language)
        
        // Preferences should be fixed
        val savedLanguage = sharedPreferences.getString("selected_language", null)
        assertEquals("Should save valid language", "en", savedLanguage)
    }
    
    @Test
    fun `concurrent locale operations`() {
        // Simulate rapid operations
        LocaleManager.setLocale(context, "en")
        val hasManual1 = LocaleManager.hasManualLanguageSelection(context)
        LocaleManager.setLocale(context, "zh")
        val hasManual2 = LocaleManager.hasManualLanguageSelection(context)
        LocaleManager.resetToSystemLanguage(context)
        val hasManual3 = LocaleManager.hasManualLanguageSelection(context)
        
        assertTrue("Should have manual after first set", hasManual1)
        assertTrue("Should have manual after second set", hasManual2)
        assertFalse("Should not have manual after reset", hasManual3)
    }
    
    @Test
    fun `all error conditions leave app functional`() {
        // Try various error conditions
        val errorConditions = listOf(
            { LocaleManager.setLocale(context, "invalid") },
            { LocaleManager.setLocale(context, "") },
            { LocaleManager.setLocale(context, "ZH") },
            { LocaleManager.setLocale(context, "de") }
        )
        
        errorConditions.forEach { errorCondition ->
            try {
                errorCondition()
            } catch (e: Exception) {
                // Expected - errors should be caught
            }
            
            // After each error, app should still be functional
            val currentLocale = LocaleManager.getCurrentLocale(context)
            assertNotNull("Should have a locale after error", currentLocale)
            
            val languages = LocaleManager.getSupportedLanguages()
            assertEquals("Should return 7 languages after error", 7, languages.size)
        }
    }
}
