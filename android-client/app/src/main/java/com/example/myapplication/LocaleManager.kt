package com.example.myapplication

import android.content.Context
import android.content.res.Configuration
import android.os.Build
import androidx.appcompat.app.AppCompatDelegate
import androidx.core.os.LocaleListCompat
import java.util.Locale

/**
 * Manages locale/language settings for the application.
 * Supports automatic detection from system settings and manual language selection.
 * 
 * Validates: Requirements 1.1, 1.2, 1.3, 1.5
 */
object LocaleManager {
    
    private const val PREFS_NAME = "locale_preferences"
    private const val KEY_SELECTED_LANGUAGE = "selected_language"
    private const val KEY_USE_SYSTEM_LANGUAGE = "use_system_language"
    
    /**
     * Supported languages with their metadata.
     * Validates: Requirement 1.6
     */
    private val supportedLanguages = listOf(
        Language("en", "English", "English"),
        Language("zh", "Chinese", "中文"),
        Language("fa", "Farsi", "فارسی"),
        Language("ru", "Russian", "Русский"),
        Language("ar", "Arabic", "العربية"),
        Language("tr", "Turkish", "Türkçe"),
        Language("vi", "Vietnamese", "Tiếng Việt")
    )
    
    /**
     * Gets the current locale being used by the application.
     * 
     * @param context Application context
     * @return Current locale
     * 
     * Validates: Requirement 1.1
     */
    fun getCurrentLocale(context: Context): Locale {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val savedLanguage = prefs.getString(KEY_SELECTED_LANGUAGE, null)
        
        return if (savedLanguage != null) {
            Locale(savedLanguage)
        } else {
            getSystemLocale(context)
        }
    }
    
    /**
     * Sets the application locale to the specified language code.
     * Persists the preference to SharedPreferences.
     * 
     * @param context Application context
     * @param languageCode ISO 639-1 language code (e.g., "en", "zh", "fa")
     * 
     * Validates: Requirements 1.2, 1.5
     */
    fun setLocale(context: Context, languageCode: String) {
        // Validate language code
        if (!supportedLanguages.any { it.code == languageCode }) {
            throw IllegalArgumentException("Unsupported language code: $languageCode")
        }
        
        // Save preference
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        prefs.edit()
            .putString(KEY_SELECTED_LANGUAGE, languageCode)
            .putBoolean(KEY_USE_SYSTEM_LANGUAGE, false)
            .apply()
        
        // Apply locale using AppCompatDelegate
        val localeList = LocaleListCompat.forLanguageTags(languageCode)
        AppCompatDelegate.setApplicationLocales(localeList)
    }
    
    /**
     * Returns the list of all supported languages.
     * 
     * @return List of supported languages with metadata
     * 
     * Validates: Requirement 1.6
     */
    fun getSupportedLanguages(): List<Language> {
        return supportedLanguages
    }
    
    /**
     * Detects and returns the system locale.
     * Falls back to English if system locale is not supported.
     * 
     * @param context Application context
     * @return System locale or default (English)
     * 
     * Validates: Requirements 1.1, 1.3
     */
    private fun getSystemLocale(context: Context): Locale {
        val systemLocale = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            context.resources.configuration.locales[0]
        } else {
            @Suppress("DEPRECATION")
            context.resources.configuration.locale
        }
        
        // Check if system language is supported
        val languageCode = systemLocale.language
        val isSupported = supportedLanguages.any { it.code == languageCode }
        
        return if (isSupported) {
            Locale(languageCode)
        } else {
            Locale("en") // Default to English
        }
    }
    
    /**
     * Checks if the user has manually selected a language.
     * 
     * @param context Application context
     * @return true if user has selected a language, false if using system default
     */
    fun hasManualLanguageSelection(context: Context): Boolean {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        return prefs.getString(KEY_SELECTED_LANGUAGE, null) != null
    }
    
    /**
     * Resets to system language by clearing the saved preference.
     * 
     * @param context Application context
     */
    fun resetToSystemLanguage(context: Context) {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        prefs.edit()
            .remove(KEY_SELECTED_LANGUAGE)
            .putBoolean(KEY_USE_SYSTEM_LANGUAGE, true)
            .apply()
        
        // Reset to system default
        AppCompatDelegate.setApplicationLocales(LocaleListCompat.getEmptyLocaleList())
    }
}

/**
 * Data class representing a language with its metadata.
 * 
 * @property code ISO 639-1 language code
 * @property displayName English name of the language
 * @property nativeName Native name of the language
 */
data class Language(
    val code: String,
    val displayName: String,
    val nativeName: String
)
