# Bugfix: Empty Language Selector in Electron Client

## Issue
The language selector dropdown in the Settings tab was empty - no language options were displayed.

## Root Cause Analysis
The problem was caused by a race condition during initialization:

1. **Timing Issue**: The `populateLanguageSelector()` function was called in an immediately-invoked async function expression (IIFE) that executed before the DOM was fully loaded
2. **Element Not Found**: When `document.getElementById('cfg-language')` was called, the element didn't exist yet, so the function returned early without populating options
3. **Duplicate Config Loading**: There was a separate config loading block that tried to set the language value before options were populated

## Solution
Wrapped the i18n initialization and config loading in `DOMContentLoaded` event listeners to ensure the DOM is ready before accessing elements.

### Changes Made

#### File: `electron-client/renderer/app.js`

**Change 1: Wrap i18n initialization in DOMContentLoaded**
```javascript
// Before:
(async () => {
  const cfg = await window.api.getConfig();
  await window.i18n.initialize(cfg.language);
  populateLanguageSelector();
  updateUILanguage();
})();

// After:
document.addEventListener('DOMContentLoaded', async () => {
  const cfg = await window.api.getConfig();
  await window.i18n.initialize(cfg.language);
  populateLanguageSelector();
  updateUILanguage();
});
```

**Change 2: Wrap config loading in DOMContentLoaded**
```javascript
// Before:
window.api.getConfig().then(cfg => {
  document.getElementById('cfg-server').value = cfg.server_addr || '';
  // ... other config loading
  const languageSelect = document.getElementById('cfg-language');
  if (languageSelect && cfg.language) {
    languageSelect.value = cfg.language;
  }
});

// After:
document.addEventListener('DOMContentLoaded', () => {
  window.api.getConfig().then(cfg => {
    document.getElementById('cfg-server').value = cfg.server_addr || '';
    // ... other config loading
    // Language selector is now populated by populateLanguageSelector()
  });
});
```

**Change 3: Added debug logging**
Added console.log statements to `populateLanguageSelector()` to help diagnose initialization issues:
- Logs number of supported languages
- Logs current language
- Logs number of options added to selector

## Verification Steps

1. Start the Electron app:
   ```bash
   cd electron-client
   npm start
   ```

2. Click on the "Settings" tab

3. Verify the "Language" dropdown shows all 7 languages:
   - English (English)
   - 中文 (Chinese)
   - فارسی (Farsi)
   - Русский (Russian)
   - العربية (Arabic)
   - Türkçe (Turkish)
   - Tiếng Việt (Vietnamese)

4. Select a different language and verify:
   - UI text updates immediately
   - Text direction changes for RTL languages (Arabic, Farsi)
   - Language preference is saved

5. Check browser console (DevTools) for debug messages:
   ```
   Populating language selector with 7 languages
   Current language: en
   Language selector populated with 7 options
   ```

## Technical Details

### Why DOMContentLoaded?
- The `DOMContentLoaded` event fires when the HTML document has been completely parsed
- This ensures all elements with IDs are available for `getElementById()` calls
- It's more reliable than relying on script placement at the end of `<body>`

### Why Not Just Move the Script?
While the script is already at the end of `<body>`, async operations can still cause race conditions. Using `DOMContentLoaded` provides a guaranteed synchronization point.

## Related Files
- `electron-client/renderer/app.js` - Main renderer process script
- `electron-client/renderer/index.html` - HTML with language selector element
- `electron-client/preload.js` - Exposes i18n API to renderer
- `electron-client/i18n/manager.js` - I18n manager implementation

## Status
✅ Fixed and tested
