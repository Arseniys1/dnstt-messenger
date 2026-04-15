# Language Selector Fix

## Problem
The language selector dropdown in the Electron client settings tab was empty (no language options displayed).

## Root Cause
The issue was caused by a race condition in the initialization order:

1. The `populateLanguageSelector()` function was called in an async IIFE that executed immediately
2. This happened before the DOM was fully loaded
3. The `getElementById('cfg-language')` call returned `null` because the element didn't exist yet
4. Additionally, there was a separate `window.api.getConfig().then()` call that tried to set the language value before options were populated

## Solution
Fixed by wrapping the i18n initialization in a `DOMContentLoaded` event listener:

```javascript
// Before (problematic):
(async () => {
  const cfg = await window.api.getConfig();
  await window.i18n.initialize(cfg.language);
  populateLanguageSelector();
  updateUILanguage();
})();

// After (fixed):
document.addEventListener('DOMContentLoaded', async () => {
  const cfg = await window.api.getConfig();
  await window.i18n.initialize(cfg.language);
  populateLanguageSelector();
  updateUILanguage();
});
```

Also wrapped the config loading in DOMContentLoaded to ensure proper initialization order.

## Changes Made
1. **electron-client/renderer/app.js**:
   - Wrapped i18n initialization in `DOMContentLoaded` event listener
   - Wrapped config loading in `DOMContentLoaded` event listener
   - Added console logging to `populateLanguageSelector()` for debugging
   - Removed redundant language selector value setting (handled by populateLanguageSelector)

## Testing
To verify the fix:
1. Start the Electron app: `npm start` (from electron-client directory)
2. Click on the "Settings" tab
3. Check that the "Language" dropdown shows all 7 languages:
   - English (English)
   - 中文 (Chinese)
   - فارسی (Farsi)
   - Русский (Russian)
   - العربية (Arabic)
   - Türkçe (Turkish)
   - Tiếng Việt (Vietnamese)
4. Select a different language and verify the UI updates
5. Check browser console for debug messages confirming selector population

## Expected Console Output
```
Populating language selector with 7 languages
Current language: en
Language selector populated with 7 options
```
