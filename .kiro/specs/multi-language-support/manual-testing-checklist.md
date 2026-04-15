# Manual Testing Checklist: Multi-Language Support

## Overview

This document provides a comprehensive manual testing checklist for validating multi-language support across all three DNSTT Messenger client platforms. Complete this checklist for each supported language to ensure translation quality, UI layout correctness, and functional behavior.

**Supported Languages:**
- English (en) - Default
- Chinese (zh) - 中文
- Farsi (fa) - فارسی (RTL)
- Russian (ru) - Русский
- Arabic (ar) - العربية (RTL)
- Turkish (tr) - Türkçe
- Vietnamese (vi) - Tiếng Việt

**Testing Requirements:** 1.6, 3.4, 3.5, 4.3, 4.5, 4.6, 5.3, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.5

---

## Android Client Testing

### Language: _________________ (Code: _____)

#### A1. Language Detection and Selection
- [ ] **First Launch Detection**: Launch app for first time with system language set to test language
  - Expected: App displays in detected language (or English if unsupported)
  - Actual: _______________________

- [ ] **Manual Language Selection**: Open Settings → Language → Select test language
  - Expected: UI updates immediately to selected language
  - Actual: _______________________

- [ ] **Persistence**: Close and reopen app
  - Expected: App remembers selected language
  - Actual: _______________________

#### A2. Login & Registration Screen
- [ ] **Tab Labels**: Verify "Login", "Register", "Settings" tabs
  - Expected: All tabs translated correctly
  - Actual: _______________________

- [ ] **Input Fields**: Check "Username" and "Password" labels and placeholders
  - Expected: Labels and placeholders translated
  - Actual: _______________________

- [ ] **Buttons**: Verify "Log In" and "Register" button text
  - Expected: Button text translated and fits within button
  - Actual: _______________________

- [ ] **Error Messages**: Trigger error (e.g., empty fields, invalid credentials)
  - Expected: Error messages appear in selected language
  - Actual: _______________________

#### A3. Chat Interface
- [ ] **Screen Title**: Check "Global Chat" or "# General" title
  - Expected: Title translated correctly
  - Actual: _______________________

- [ ] **Message Input**: Check placeholder text "Message..."
  - Expected: Placeholder translated
  - Actual: _______________________

- [ ] **Send Button**: Verify "Send" button text
  - Expected: Button text translated and fits
  - Actual: _______________________

- [ ] **History Divider**: Check "— end of history —" text
  - Expected: Divider text translated
  - Actual: _______________________

#### A4. Sidebar Navigation
- [ ] **Section Headers**: Verify "Chats", "Direct Messages", "Rooms", "Online"
  - Expected: All section headers translated
  - Actual: _______________________

- [ ] **Online Count**: Check "Online ({count})" with dynamic number
  - Expected: Text translated with correct number substitution
  - Actual: _______________________

- [ ] **Buttons**: Verify "Log Out" and "Menu" buttons
  - Expected: Button text translated and fits
  - Actual: _______________________

#### A5. Direct Messages
- [ ] **Modal Title**: Open DM modal, check "New Direct Message" title
  - Expected: Title translated
  - Actual: _______________________

- [ ] **Input Labels**: Check "Username" label
  - Expected: Label translated
  - Actual: _______________________

- [ ] **Buttons**: Verify "Cancel" and "Open" buttons
  - Expected: Button text translated and fits
  - Actual: _______________________

#### A6. Rooms
- [ ] **Create Room Modal**: Open create room modal
  - Expected: "Create Room", "Room Name", "Description", "Public" all translated
  - Actual: _______________________

- [ ] **Room Members Count**: Check "{count} members" with dynamic number
  - Expected: Text translated with correct pluralization
  - Actual: _______________________

- [ ] **Room Actions**: Verify "Invite", "Leave", "Create" buttons
  - Expected: All buttons translated and fit
  - Actual: _______________________

#### A7. Settings Screen
- [ ] **Settings Labels**: Check "Server Address", "SOCKS5 Proxy", "Direct connection"
  - Expected: All labels translated
  - Actual: _______________________

- [ ] **Language Selector**: Verify language dropdown shows native names
  - Expected: Languages displayed in native script (e.g., "中文", "العربية")
  - Actual: _______________________

- [ ] **Save Button**: Check "Save" button and "Settings saved" confirmation
  - Expected: Button and confirmation translated
  - Actual: _______________________

#### A8. Notifications
- [ ] **Notification Text**: Trigger notification (send message while app in background)
  - Expected: Notification title and body in selected language
  - Actual: _______________________

#### A9. RTL Layout (Arabic and Farsi only)
- [ ] **Text Direction**: Check all text aligns right-to-left
  - Expected: Text flows from right to left
  - Actual: _______________________

- [ ] **UI Mirroring**: Check if UI elements are mirrored (back buttons, icons)
  - Expected: Layout mirrors appropriately for RTL
  - Actual: _______________________

- [ ] **Mixed Content**: Check messages with numbers or English words
  - Expected: Mixed content displays correctly
  - Actual: _______________________

#### A10. Text Fitting and Layout
- [ ] **Button Text**: Check all buttons for text overflow or truncation
  - Expected: All button text fits without truncation
  - Actual: _______________________

- [ ] **Long Translations**: Check labels with longer translations (e.g., German-length text)
  - Expected: UI adapts or text wraps appropriately
  - Actual: _______________________

- [ ] **Input Placeholders**: Verify placeholders don't overflow input fields
  - Expected: Placeholders fit within input fields
  - Actual: _______________________

#### A11. Dynamic Text and Pluralization
- [ ] **Online Count**: Test with 0, 1, 2, 5, 10+ users online
  - Expected: Correct plural form for each count (e.g., "1 user" vs "5 users")
  - Actual: _______________________

- [ ] **Room Members**: Test with 0, 1, 2, 5+ members
  - Expected: Correct plural form for each count
  - Actual: _______________________

- [ ] **Message Placeholders**: Send DM and check "Message to {user}..." placeholder
  - Expected: Username correctly substituted
  - Actual: _______________________

---

## Electron Client Testing

### Language: _________________ (Code: _____)

#### E1. Language Detection and Selection
- [ ] **First Launch Detection**: Launch app with OS language set to test language
  - Expected: App displays in detected language (or English if unsupported)
  - Actual: _______________________

- [ ] **Manual Language Selection**: Open Settings tab → Language dropdown → Select test language
  - Expected: UI updates immediately without restart
  - Actual: _______________________

- [ ] **Persistence**: Close and reopen app
  - Expected: App remembers selected language
  - Actual: _______________________

#### E2. Login & Registration Screen
- [ ] **Tab Labels**: Verify "Login", "Register", "Settings" tabs
  - Expected: All tabs translated correctly
  - Actual: _______________________

- [ ] **Input Fields**: Check "Username" and "Password" labels and placeholders
  - Expected: Labels and placeholders translated
  - Actual: _______________________

- [ ] **Buttons**: Verify "Log In" and "Register" button text
  - Expected: Button text translated and fits within button
  - Actual: _______________________

- [ ] **Error Messages**: Trigger error (e.g., empty fields, invalid credentials)
  - Expected: Error messages appear in selected language
  - Actual: _______________________

#### E3. Chat Interface
- [ ] **Screen Title**: Check "Global Chat" or "# General" title
  - Expected: Title translated correctly
  - Actual: _______________________

- [ ] **Message Input**: Check placeholder text "Message..."
  - Expected: Placeholder translated
  - Actual: _______________________

- [ ] **Send Button**: Verify "Send" button text
  - Expected: Button text translated and fits
  - Actual: _______________________

- [ ] **History Divider**: Check "— end of history —" text
  - Expected: Divider text translated
  - Actual: _______________________

#### E4. Sidebar Navigation
- [ ] **Section Headers**: Verify "Chats", "Direct Messages", "Rooms", "Online"
  - Expected: All section headers translated
  - Actual: _______________________

- [ ] **Online Count**: Check "Online ({count})" with dynamic number
  - Expected: Text translated with correct number substitution
  - Actual: _______________________

- [ ] **Buttons**: Verify "Log Out" and "Menu" buttons
  - Expected: Button text translated and fits
  - Actual: _______________________

#### E5. Direct Messages
- [ ] **Modal Title**: Open DM modal, check "New Direct Message" title
  - Expected: Title translated
  - Actual: _______________________

- [ ] **Input Labels**: Check "Username" label
  - Expected: Label translated
  - Actual: _______________________

- [ ] **Buttons**: Verify "Cancel" and "Open" buttons
  - Expected: Button text translated and fits
  - Actual: _______________________

#### E6. Rooms
- [ ] **Create Room Modal**: Open create room modal
  - Expected: "Create Room", "Room Name", "Description", "Public" all translated
  - Actual: _______________________

- [ ] **Room Members Count**: Check "{count} members" with dynamic number
  - Expected: Text translated with correct pluralization
  - Actual: _______________________

- [ ] **Room Actions**: Verify "Invite", "Leave", "Create" buttons
  - Expected: All buttons translated and fit
  - Actual: _______________________

#### E7. Settings Tab
- [ ] **Settings Labels**: Check "Server Address", "SOCKS5 Proxy", "Direct connection"
  - Expected: All labels translated
  - Actual: _______________________

- [ ] **Language Selector**: Verify language dropdown shows native names
  - Expected: Languages displayed in native script (e.g., "中文", "العربية")
  - Actual: _______________________

- [ ] **Save Button**: Check "Save" button and "Settings saved" confirmation
  - Expected: Button and confirmation translated
  - Actual: _______________________

#### E8. Status Messages
- [ ] **Connection Status**: Check "Connecting...", "Connected", "Disconnected" messages
  - Expected: All status messages translated
  - Actual: _______________________

- [ ] **Mode Messages**: Check "Mode: Direct Connect" and "Mode: DNSTT Proxy" messages
  - Expected: Mode messages translated with correct parameter substitution
  - Actual: _______________________

#### E9. RTL Layout (Arabic and Farsi only)
- [ ] **Text Direction**: Check all text aligns right-to-left
  - Expected: Text flows from right to left
  - Actual: _______________________

- [ ] **HTML dir Attribute**: Inspect root element for dir="rtl"
  - Expected: <html dir="rtl"> or <body dir="rtl">
  - Actual: _______________________

- [ ] **CSS RTL Styles**: Check if layout mirrors (sidebar, buttons, inputs)
  - Expected: Layout mirrors appropriately for RTL
  - Actual: _______________________

- [ ] **Mixed Content**: Check messages with numbers or English words
  - Expected: Mixed content displays correctly
  - Actual: _______________________

#### E10. Text Fitting and Layout
- [ ] **Button Text**: Check all buttons for text overflow or truncation
  - Expected: All button text fits without truncation
  - Actual: _______________________

- [ ] **Tab Labels**: Verify tab labels fit within tab width
  - Expected: Tab labels don't overflow
  - Actual: _______________________

- [ ] **Sidebar Sections**: Check section headers fit within sidebar width
  - Expected: Headers fit without wrapping or truncation
  - Actual: _______________________

- [ ] **Modal Dialogs**: Check modal titles and content fit properly
  - Expected: Modal content displays without overflow
  - Actual: _______________________

#### E11. Dynamic Text and Pluralization
- [ ] **Online Count**: Test with 0, 1, 2, 5, 10+ users online
  - Expected: Correct plural form for each count
  - Actual: _______________________

- [ ] **Room Members**: Test with 0, 1, 2, 5+ members
  - Expected: Correct plural form for each count
  - Actual: _______________________

- [ ] **Server Count**: Check "Known servers ({count})" with different counts
  - Expected: Correct pluralization
  - Actual: _______________________

- [ ] **Message Placeholders**: Send DM and check "Message to {user}..." placeholder
  - Expected: Username correctly substituted
  - Actual: _______________________

#### E12. Language Switching Without Restart
- [ ] **Switch Language**: Change language in settings while app is running
  - Expected: All visible text updates immediately
  - Actual: _______________________

- [ ] **Switch to RTL**: Switch from LTR language to Arabic or Farsi
  - Expected: Layout direction changes immediately
  - Actual: _______________________

- [ ] **Switch from RTL**: Switch from Arabic/Farsi to LTR language
  - Expected: Layout direction changes back to LTR
  - Actual: _______________________

---

## Go CLI Client Testing

### Language: _________________ (Code: _____)

#### G1. Language Detection and Selection
- [ ] **Environment Variable Detection**: Set LANG=xx_XX.UTF-8 and launch client
  - Expected: Client displays in detected language (or English if unsupported)
  - Actual: _______________________

- [ ] **Command-Line Flag**: Launch with `--lang=xx` flag
  - Expected: Client displays in specified language
  - Actual: _______________________

- [ ] **Config File**: Set "language": "xx" in client_config.json and launch
  - Expected: Client displays in configured language
  - Actual: _______________________

- [ ] **Priority Order**: Test flag > config > env variable priority
  - Expected: Flag overrides config, config overrides env
  - Actual: _______________________

- [ ] **Persistence**: Select language, restart client
  - Expected: Client remembers language in config file
  - Actual: _______________________

#### G2. Login & Registration Prompts
- [ ] **Choice Prompt**: Check "1. Login\n2. Register" prompt
  - Expected: Prompt translated correctly
  - Actual: _______________________

- [ ] **Input Prompts**: Check "Username: " and "Password: " prompts
  - Expected: Prompts translated
  - Actual: _______________________

- [ ] **Error Messages**: Trigger errors (empty input, invalid credentials)
  - Expected: Error messages in selected language
  - Actual: _______________________

- [ ] **Success Messages**: Complete registration
  - Expected: "Account created! Now log in." translated
  - Actual: _______________________

#### G3. Chat Interface
- [ ] **Message Prompt**: Check ">> " prompt
  - Expected: Prompt displayed (may not need translation)
  - Actual: _______________________

- [ ] **Message Format**: Check "[{time}] [{sender}]: {text}" format
  - Expected: Format preserved with translated labels if any
  - Actual: _______________________

- [ ] **History Header**: Check "--- Chat History ---" and "--- End of History ---"
  - Expected: Headers translated
  - Actual: _______________________

#### G4. Command Help
- [ ] **Help Commands**: Type each command to see help text
  - `/exit` - Expected: "exit" translated
  - `/servers` - Expected: "servers" translated
  - `/dm` - Expected: "direct message" translated
  - `/rooms` - Expected: "list rooms" translated
  - `/join` - Expected: "join room" translated
  - `/leave` - Expected: "leave room" translated
  - `/create` - Expected: "create room" translated
  - `/room` - Expected: "message to room" translated
  - `/invite` - Expected: "invite" translated
  - Actual: _______________________

- [ ] **Usage Messages**: Trigger usage errors (e.g., `/dm` without arguments)
  - Expected: "Usage: /dm <user> <text>" translated
  - Actual: _______________________

#### G5. Status Messages
- [ ] **Connection Status**: Check "Connecting...", "Authorizing...", "Connected" messages
  - Expected: All status messages translated
  - Actual: _______________________

- [ ] **Mode Messages**: Check "Mode: Direct Connect" and "Mode: DNSTT Proxy" messages
  - Expected: Mode messages translated with correct parameter substitution
  - Actual: _______________________

- [ ] **Disconnection**: Check "Connection closed" and "Connection lost" messages
  - Expected: Messages translated
  - Actual: _______________________

#### G6. Error Messages
- [ ] **Connection Errors**: Trigger connection error (wrong server address)
  - Expected: "Connection error: {error}" translated
  - Actual: _______________________

- [ ] **Validation Errors**: Trigger validation errors (empty username, etc.)
  - Expected: All validation errors translated
  - Actual: _______________________

- [ ] **Command Errors**: Use invalid command syntax
  - Expected: Error messages translated
  - Actual: _______________________

#### G7. Server and Room Messages
- [ ] **Server List**: Use `/servers` command
  - Expected: "Known servers ({count}):" translated with correct count
  - Actual: _______________________

- [ ] **Room List**: Use `/rooms` command
  - Expected: "Rooms ({count}):" translated with correct count
  - Actual: _______________________

- [ ] **Room Events**: Join/leave room, check event messages
  - Expected: "{user} joined the room" and "{user} left the room" translated
  - Actual: _______________________

#### G8. Dynamic Text and Pluralization
- [ ] **Online Count**: Check "Online ({count}): {users}" with different counts
  - Expected: Correct plural form for each count
  - Actual: _______________________

- [ ] **Server Count**: Check "Known servers ({count})" with 0, 1, 2, 5+ servers
  - Expected: Correct pluralization
  - Actual: _______________________

- [ ] **Room Count**: Check "Rooms ({count})" with different counts
  - Expected: Correct pluralization
  - Actual: _______________________

- [ ] **Members Count**: Check "{count} members" with different counts
  - Expected: Correct pluralization
  - Actual: _______________________

#### G9. Text Formatting
- [ ] **Console Width**: Check if long translated text wraps properly in terminal
  - Expected: Text wraps at terminal width without breaking
  - Actual: _______________________

- [ ] **Special Characters**: Check if language-specific characters display correctly
  - Expected: Characters render properly in terminal (UTF-8 support)
  - Actual: _______________________

- [ ] **RTL in Terminal** (Arabic and Farsi): Check if RTL text displays
  - Expected: Text may not mirror in terminal, but characters should be correct
  - Note: Terminal RTL support varies by terminal emulator
  - Actual: _______________________

---

## Cross-Platform Consistency Testing

### Language: _________________ (Code: _____)

#### C1. Translation Consistency
- [ ] **Same Keys**: Compare same UI element across all three platforms
  - Example: "Log In" button on Android, Electron, and Go
  - Expected: Same translation used across all platforms
  - Actual: _______________________

- [ ] **Terminology**: Check consistent terminology (e.g., "Direct Message" vs "Private Message")
  - Expected: Same terms used across platforms
  - Actual: _______________________

#### C2. Pluralization Consistency
- [ ] **Online Count**: Compare "Online ({count})" across platforms with same count
  - Expected: Same plural form used on all platforms
  - Actual: _______________________

- [ ] **Room Members**: Compare "{count} members" across platforms
  - Expected: Same plural form used on all platforms
  - Actual: _______________________

#### C3. Parameter Substitution
- [ ] **Username Substitution**: Compare "Message to {user}..." across platforms
  - Expected: Same format and substitution behavior
  - Actual: _______________________

- [ ] **Server Substitution**: Compare "Server selected: {server}" across platforms
  - Expected: Same format and substitution behavior
  - Actual: _______________________

---

## Language-Specific Testing Notes

### Chinese (zh - 中文)
- **Character Set**: Verify Simplified Chinese characters display correctly
- **Text Length**: Chinese translations are typically shorter than English
- **No Pluralization**: Chinese doesn't have plural forms, check count displays correctly

### Farsi (fa - فارسی)
- **RTL Layout**: CRITICAL - Verify complete RTL layout on Android and Electron
- **Arabic Script**: Verify Persian-specific characters (پ، چ، ژ، گ) display correctly
- **Numerals**: Check if Western (1,2,3) or Persian (۱،۲،۳) numerals are used
- **Mixed Content**: Test messages with English words or URLs

### Russian (ru - Русский)
- **Cyrillic Script**: Verify all Cyrillic characters display correctly
- **Text Length**: Russian translations are typically longer than English
- **Pluralization**: Russian has complex plural rules (one, few, many) - test thoroughly

### Arabic (ar - العربية)
- **RTL Layout**: CRITICAL - Verify complete RTL layout on Android and Electron
- **Arabic Script**: Verify all Arabic characters and diacritics display correctly
- **Numerals**: Check if Western (1,2,3) or Arabic-Indic (٠،١،٢) numerals are used
- **Mixed Content**: Test messages with English words or URLs
- **Pluralization**: Arabic has complex plural rules (zero, one, two, few, many)

### Turkish (tr - Türkçe)
- **Special Characters**: Verify Turkish-specific characters (ı, ğ, ü, ş, ö, ç) display correctly
- **Text Length**: Turkish translations are typically similar to English length
- **Pluralization**: Turkish has simpler plural rules than English

### Vietnamese (vi - Tiếng Việt)
- **Diacritics**: Verify Vietnamese tone marks display correctly (á, à, ả, ã, ạ, etc.)
- **Text Length**: Vietnamese translations are typically similar to English length
- **No Pluralization**: Vietnamese doesn't have plural forms, check count displays correctly

---

## Testing Summary

### Overall Results

**Android Client:**
- Languages Tested: _____ / 7
- Issues Found: _____
- Critical Issues: _____

**Electron Client:**
- Languages Tested: _____ / 7
- Issues Found: _____
- Critical Issues: _____

**Go CLI Client:**
- Languages Tested: _____ / 7
- Issues Found: _____
- Critical Issues: _____

### Critical Issues Log

| Platform | Language | Issue Description | Severity | Status |
|----------|----------|-------------------|----------|--------|
|          |          |                   |          |        |
|          |          |                   |          |        |
|          |          |                   |          |        |

### Non-Critical Issues Log

| Platform | Language | Issue Description | Status |
|----------|----------|-------------------|--------|
|          |          |                   |        |
|          |          |                   |        |
|          |          |                   |        |

---

## Sign-Off

**Tester Name:** _______________________

**Date:** _______________________

**Platforms Tested:**
- [ ] Android Client
- [ ] Electron Client
- [ ] Go CLI Client

**Languages Tested:**
- [ ] English (en)
- [ ] Chinese (zh)
- [ ] Farsi (fa)
- [ ] Russian (ru)
- [ ] Arabic (ar)
- [ ] Turkish (tr)
- [ ] Vietnamese (vi)

**Overall Assessment:**
- [ ] All tests passed - Ready for release
- [ ] Minor issues found - Can release with known issues
- [ ] Major issues found - Requires fixes before release
- [ ] Critical issues found - Cannot release

**Additional Notes:**

_______________________________________________________________________________________

_______________________________________________________________________________________

_______________________________________________________________________________________
