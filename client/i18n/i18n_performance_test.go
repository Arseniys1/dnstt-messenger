package i18n

import (
	"fmt"
	"testing"
	"time"
)

// TestLanguageSwitchingPerformance validates Requirement 10.3:
// Language switching must complete within 500ms
func TestLanguageSwitchingPerformance(t *testing.T) {
	languages := []string{"en", "zh", "fa", "ru", "ar", "tr", "vi"}
	results := make([]struct {
		transition string
		duration   time.Duration
	}, 0)

	m := NewManager()

	// Warm up - first switch might be slower due to initialization
	if err := m.LoadLanguage("en"); err != nil {
		t.Fatalf("Failed to load initial language: %v", err)
	}

	// Measure each language switch
	for i := 0; i < len(languages)-1; i++ {
		fromLang := languages[i]
		toLang := languages[i+1]

		// Set initial language
		if err := m.LoadLanguage(fromLang); err != nil {
			t.Fatalf("Failed to load language %s: %v", fromLang, err)
		}

		// Measure switch time
		start := time.Now()
		if err := m.SetLanguage(toLang); err != nil {
			t.Fatalf("Failed to switch to language %s: %v", toLang, err)
		}
		switchTime := time.Since(start)

		results = append(results, struct {
			transition string
			duration   time.Duration
		}{
			transition: fmt.Sprintf("%s -> %s", fromLang, toLang),
			duration:   switchTime,
		})

		// Verify requirement: < 500ms
		if switchTime > 500*time.Millisecond {
			t.Errorf("Language switch from %s to %s took %v, exceeds 500ms requirement",
				fromLang, toLang, switchTime)
		}
	}

	// Print results for analysis
	t.Log("\n=== Language Switching Performance Results ===")
	var totalTime time.Duration
	var maxTime time.Duration
	for _, result := range results {
		t.Logf("%s: %v", result.transition, result.duration)
		totalTime += result.duration
		if result.duration > maxTime {
			maxTime = result.duration
		}
	}

	avgTime := totalTime / time.Duration(len(results))
	t.Logf("\nAverage switch time: %v", avgTime)
	t.Logf("Maximum switch time: %v", maxTime)
	t.Logf("Requirement: < 500ms")
	if maxTime < 500*time.Millisecond {
		t.Log("Status: PASS")
	} else {
		t.Error("Status: FAIL")
	}
}

// TestRapidConsecutiveSwitches tests rapid language switching performance
func TestRapidConsecutiveSwitches(t *testing.T) {
	languages := []string{"en", "zh", "ru", "ar", "en"}
	results := make([]time.Duration, 0)

	m := NewManager()

	// Perform rapid switches
	for i := 0; i < len(languages)-1; i++ {
		start := time.Now()
		if err := m.SetLanguage(languages[i+1]); err != nil {
			t.Fatalf("Failed to switch to language %s: %v", languages[i+1], err)
		}
		switchTime := time.Since(start)
		results = append(results, switchTime)

		if switchTime > 500*time.Millisecond {
			t.Errorf("Rapid switch to %s took %v, exceeds 500ms requirement",
				languages[i+1], switchTime)
		}
	}

	t.Log("\n=== Rapid Switching Performance ===")
	for i, duration := range results {
		t.Logf("Switch %d: %v", i+1, duration)
	}
}

// TestLanguageSwitchWithTranslation tests language switch with immediate translation
func TestLanguageSwitchWithTranslation(t *testing.T) {
	m := NewManager()

	if err := m.LoadLanguage("en"); err != nil {
		t.Fatalf("Failed to load English: %v", err)
	}

	start := time.Now()
	if err := m.SetLanguage("zh"); err != nil {
		t.Fatalf("Failed to switch to Chinese: %v", err)
	}
	// Perform some translations immediately after switch
	_ = m.T("app.name")
	_ = m.T("login.title")
	_ = m.T("chat.title")
	totalTime := time.Since(start)

	t.Log("\n=== Language Switch with Translation Lookup ===")
	t.Logf("Time: %v", totalTime)
	t.Logf("Requirement: < 500ms")
	if totalTime < 500*time.Millisecond {
		t.Log("Status: PASS")
	} else {
		t.Error("Status: FAIL")
	}

	if totalTime > 500*time.Millisecond {
		t.Errorf("Language switch with translation took %v, exceeds 500ms requirement", totalTime)
	}
}

// TestRTLLanguageSwitchPerformance tests RTL language switching performance
func TestRTLLanguageSwitchPerformance(t *testing.T) {
	rtlLanguages := []string{"ar", "fa"}
	results := make(map[string]time.Duration)

	m := NewManager()

	for _, lang := range rtlLanguages {
		start := time.Now()
		if err := m.SetLanguage(lang); err != nil {
			t.Fatalf("Failed to switch to RTL language %s: %v", lang, err)
		}
		switchTime := time.Since(start)
		results[lang] = switchTime

		if switchTime > 500*time.Millisecond {
			t.Errorf("RTL language switch to %s took %v, exceeds 500ms requirement",
				lang, switchTime)
		}
	}

	t.Log("\n=== RTL Language Switching Performance ===")
	for lang, duration := range results {
		t.Logf("%s: %v", lang, duration)
	}
}

// TestTranslationLookupPerformance tests that translation lookups have no file I/O overhead
func TestTranslationLookupPerformance(t *testing.T) {
	m := NewManager()

	if err := m.LoadLanguage("en"); err != nil {
		t.Fatalf("Failed to load English: %v", err)
	}

	// Measure translation lookup time (should be pure memory access)
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_ = m.T("app.name")
		_ = m.T("login.title")
		_ = m.T("chat.title")
	}

	totalTime := time.Since(start)
	avgTime := totalTime / time.Duration(iterations)

	t.Log("\n=== Translation Lookup Performance ===")
	t.Logf("%d iterations: %v", iterations, totalTime)
	t.Logf("Average per lookup: %v", avgTime)
	t.Log("Expected: < 1ms per lookup (memory access only)")

	// Each lookup should be extremely fast (< 1ms) since it's just memory access
	if avgTime > time.Millisecond {
		t.Errorf("Translation lookup took %v on average, expected < 1ms", avgTime)
	}
}

// TestLanguageSwitchWithFallback tests language switching with fallback behavior
func TestLanguageSwitchWithFallback(t *testing.T) {
	m := NewManager()

	if err := m.LoadLanguage("en"); err != nil {
		t.Fatalf("Failed to load English: %v", err)
	}

	start := time.Now()
	if err := m.SetLanguage("zh"); err != nil {
		t.Fatalf("Failed to switch to Chinese: %v", err)
	}
	// Try to translate a key that might not exist (tests fallback)
	_ = m.T("nonexistent.key")
	totalTime := time.Since(start)

	t.Log("\n=== Language Switch with Fallback ===")
	t.Logf("Time: %v", totalTime)

	if totalTime > 500*time.Millisecond {
		t.Errorf("Language switch with fallback took %v, exceeds 500ms requirement", totalTime)
	}
}

// BenchmarkLanguageSwitching provides benchmark data for language switching
func BenchmarkLanguageSwitching(b *testing.B) {
	m := NewManager()
	languages := []string{"en", "zh", "fa", "ru", "ar", "tr", "vi"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lang := languages[i%len(languages)]
		if err := m.SetLanguage(lang); err != nil {
			b.Fatalf("Failed to switch language: %v", err)
		}
	}
}

// BenchmarkTranslationLookup provides benchmark data for translation lookups
func BenchmarkTranslationLookup(b *testing.B) {
	m := NewManager()
	if err := m.LoadLanguage("en"); err != nil {
		b.Fatalf("Failed to load language: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.T("app.name")
	}
}

// BenchmarkTranslationWithParams provides benchmark data for translation with parameters
func BenchmarkTranslationWithParams(b *testing.B) {
	m := NewManager()
	if err := m.LoadLanguage("en"); err != nil {
		b.Fatalf("Failed to load language: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.T("chat.online_count", "count", 5)
	}
}
