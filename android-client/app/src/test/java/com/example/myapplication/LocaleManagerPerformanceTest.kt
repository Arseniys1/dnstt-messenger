package com.example.myapplication

import android.content.Context
import android.os.Build
import androidx.test.core.app.ApplicationProvider
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.system.measureTimeMillis

/**
 * Performance tests for LocaleManager language switching.
 * Validates: Requirement 10.3 - Language switching must complete within 500ms
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [Build.VERSION_CODES.P])
class LocaleManagerPerformanceTest {
    
    private lateinit var context: Context
    
    @Before
    fun setup() {
        context = ApplicationProvider.getApplicationContext()
    }
    
    @Test
    fun `language switching completes within 500ms`() {
        val languages = listOf("en", "zh", "fa", "ru", "ar", "tr", "vi")
        val results = mutableListOf<Pair<String, Long>>()
        
        // Warm up - first switch might be slower due to initialization
        LocaleManager.setLocale(context, "en")
        
        // Measure each language switch
        for (i in 0 until languages.size - 1) {
            val fromLang = languages[i]
            val toLang = languages[i + 1]
            
            // Set initial language
            LocaleManager.setLocale(context, fromLang)
            
            // Measure switch time
            val switchTime = measureTimeMillis {
                LocaleManager.setLocale(context, toLang)
            }
            
            results.add(Pair("$fromLang -> $toLang", switchTime))
            
            // Verify requirement: < 500ms
            assert(switchTime < 500) {
                "Language switch from $fromLang to $toLang took ${switchTime}ms, exceeds 500ms requirement"
            }
        }
        
        // Print results for analysis
        println("\n=== Language Switching Performance Results ===")
        results.forEach { (transition, time) ->
            println("$transition: ${time}ms")
        }
        
        val avgTime = results.map { it.second }.average()
        val maxTime = results.maxOf { it.second }
        
        println("\nAverage switch time: ${avgTime.toLong()}ms")
        println("Maximum switch time: ${maxTime}ms")
        println("Requirement: < 500ms")
        println("Status: ${if (maxTime < 500) "PASS" else "FAIL"}")
    }
    
    @Test
    fun `rapid consecutive language switches complete within 500ms each`() {
        val languages = listOf("en", "zh", "ru", "ar", "en")
        val results = mutableListOf<Long>()
        
        // Perform rapid switches
        for (i in 0 until languages.size - 1) {
            val switchTime = measureTimeMillis {
                LocaleManager.setLocale(context, languages[i + 1])
            }
            results.add(switchTime)
            
            assert(switchTime < 500) {
                "Rapid switch to ${languages[i + 1]} took ${switchTime}ms, exceeds 500ms requirement"
            }
        }
        
        println("\n=== Rapid Switching Performance ===")
        results.forEachIndexed { index, time ->
            println("Switch ${index + 1}: ${time}ms")
        }
    }
    
    @Test
    fun `language switch with persistence completes within 500ms`() {
        val switchTime = measureTimeMillis {
            LocaleManager.setLocale(context, "zh")
            // Verify persistence happened
            val savedLocale = LocaleManager.getCurrentLocale(context)
            assert(savedLocale.language == "zh")
        }
        
        println("\n=== Language Switch with Persistence ===")
        println("Time: ${switchTime}ms")
        println("Requirement: < 500ms")
        println("Status: ${if (switchTime < 500) "PASS" else "FAIL"}")
        
        assert(switchTime < 500) {
            "Language switch with persistence took ${switchTime}ms, exceeds 500ms requirement"
        }
    }
    
    @Test
    fun `RTL language switch completes within 500ms`() {
        val rtlLanguages = listOf("ar", "fa")
        val results = mutableListOf<Pair<String, Long>>()
        
        for (lang in rtlLanguages) {
            val switchTime = measureTimeMillis {
                LocaleManager.setLocale(context, lang)
            }
            results.add(Pair(lang, switchTime))
            
            assert(switchTime < 500) {
                "RTL language switch to $lang took ${switchTime}ms, exceeds 500ms requirement"
            }
        }
        
        println("\n=== RTL Language Switching Performance ===")
        results.forEach { (lang, time) ->
            println("$lang: ${time}ms")
        }
    }
}
