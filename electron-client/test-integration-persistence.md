# Integration Test: Language Persistence

## Test Scenario: Complete Language Persistence Flow

This document describes how to manually test the language persistence feature end-to-end.

## Prerequisites

- Electron app is installed and ready to run
- No existing config.json (or delete it to test first-run scenario)

## Test Steps

### Test 1: First Run - System Language Detection

1. **Delete existing config** (if any):
   - Location: `~/.config/dnstt-messenger/config.json` (Linux)
   - Location: `%APPDATA%/dnstt-messenger/config.json` (Windows)
   - Location: `~/Library/Application Support/dnstt-messenger/config.json` (macOS)

2. **Start the app**:
   ```bash
   cd electron-client
   npm start
   ```

3. **Expected Result**:
   - App detects system language
   - If system language is supported (en, zh, fa, ru, ar, tr, vi), UI shows in that language
   - If system language is not supported, UI shows in English
   - Language selector in settings shows the detected language

4. **Verify config.json is created**:
   - Check the config file location
   - Should contain default settings (no language field yet)

### Test 2: Change Language and Verify Persistence

1. **With app running**, go to Settings tab

2. **Change language** from dropdown:
   - Select "中文 (Chinese)"
   - Click outside dropdown or press Enter

3. **Expected Result**:
   - UI immediately updates to Chinese
   - Text direction remains LTR
   - Status message shows "设置已保存" (Settings saved)

4. **Verify config.json is updated**:
   ```bash
   # Linux/macOS
   cat ~/.config/dnstt-messenger/config.json
   
   # Windows
   type %APPDATA%\dnstt-messenger\config.json
   ```
   
   Should contain:
   ```json
   {
     "proxy_addr": "127.0.0.1:18000",
     "server_addr": "127.0.0.1:9999",
     "direct_mode": false,
     "language": "zh"
   }
   ```

5. **Close the app** (Ctrl+Q or close window)

6. **Restart the app**:
   ```bash
   npm start
   ```

7. **Expected Result**:
   - App loads with Chinese language
   - Language selector shows "中文 (Chinese)" selected
   - All UI elements are in Chinese

### Test 3: Change to RTL Language

1. **With app running**, go to Settings tab

2. **Change language** to Arabic:
   - Select "العربية (Arabic)"

3. **Expected Result**:
   - UI immediately updates to Arabic
   - Text direction changes to RTL (right-to-left)
   - Layout mirrors (sidebar on right, buttons aligned right)
   - Status message shows in Arabic

4. **Verify config.json**:
   ```json
   {
     "language": "ar"
   }
   ```

5. **Restart app** and verify Arabic persists

### Test 4: Change to Another Language

1. **Change language** to Russian:
   - Select "Русский (Russian)"

2. **Expected Result**:
   - UI updates to Russian
   - Text direction changes back to LTR
   - Layout returns to normal (sidebar on left)

3. **Verify config.json**:
   ```json
   {
     "language": "ru"
   }
   ```

4. **Restart app** and verify Russian persists

### Test 5: Verify Other Settings Persist

1. **In Settings tab**, change:
   - Server address: "example.com:9999"
   - Proxy address: "127.0.0.1:8080"
   - Direct mode: checked

2. **Click "Save Settings"**

3. **Change language** to English

4. **Verify config.json** contains all settings:
   ```json
   {
     "proxy_addr": "127.0.0.1:8080",
     "server_addr": "example.com:9999",
     "direct_mode": true,
     "language": "en"
   }
   ```

5. **Restart app** and verify:
   - Language is English
   - Server address is "example.com:9999"
   - Proxy address is "127.0.0.1:8080"
   - Direct mode is checked

## Expected Results Summary

✅ **All tests should pass with the following behaviors:**

1. **First Run**: System language is detected and used
2. **Language Change**: UI updates immediately without restart
3. **Persistence**: Language preference is saved to config.json
4. **Restart**: Saved language is loaded on app startup
5. **RTL Support**: Arabic and Farsi trigger RTL layout
6. **Config Integrity**: Other settings are preserved when language changes

## Troubleshooting

### Issue: Language doesn't persist after restart

**Possible Causes:**
- Config file is not writable
- Config path is incorrect
- App doesn't have permission to write to userData directory

**Solution:**
- Check file permissions
- Verify config path: `console.log(app.getPath('userData'))`
- Run app with appropriate permissions

### Issue: UI doesn't update after language change

**Possible Causes:**
- Event listener not attached
- Translation files not loaded
- UI update function not called

**Solution:**
- Check browser console for errors
- Verify translation files exist in `i18n/locales/`
- Ensure `updateUILanguage()` is called after language change

### Issue: Config.json is empty or missing language field

**Possible Causes:**
- Save function not called
- IPC communication failed
- JSON serialization error

**Solution:**
- Check main process console for errors
- Verify IPC handlers are registered
- Test `window.api.saveConfig()` in browser console

## Conclusion

If all tests pass, language persistence is working correctly and meets the requirements:
- ✅ Requirement 1.5: Language preference persists across restarts
- ✅ Requirement 4.4: Language selector in settings tab
- ✅ Requirement 4.5: UI updates without restart
