package com.example.myapplication

import android.content.Context
import android.content.res.Configuration
import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import org.json.JSONObject
import java.util.Locale

fun normalizeLanguage(language: String?): String = if (language == "ru") "ru" else "en"

fun localizedContext(base: Context, language: String): Context {
    val locale = Locale(normalizeLanguage(language))
    val config = Configuration(base.resources.configuration)
    config.setLocale(locale)
    return base.createConfigurationContext(config)
}

fun localizedString(base: Context, language: String, @StringRes resId: Int, vararg args: Any): String {
    val ctx = localizedContext(base, language)
    return if (args.isEmpty()) ctx.getString(resId) else ctx.getString(resId, *args)
}

@Composable
fun t(language: String, @StringRes resId: Int, vararg args: Any): String {
    val context = LocalContext.current
    return localizedString(context, language, resId, *args)
}

fun readSavedLanguage(context: Context): String {
    val prefs = context.getSharedPreferences("messenger", Context.MODE_PRIVATE)
    val json = prefs.getString("config", null) ?: return "en"
    return try {
        normalizeLanguage(JSONObject(json).optString("language", "en"))
    } catch (_: Exception) {
        "en"
    }
}
