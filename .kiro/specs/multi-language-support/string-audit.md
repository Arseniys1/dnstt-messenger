# String Audit Report

This document maps all hardcoded strings found in the three clients to their corresponding translation keys.

## Android Client (Kotlin)

### MainActivity.kt

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "🔐 DNSTT Messenger" | `app.name` | "🔐 DNSTT Messenger" |
| "Вход" | `login.tab_login` | "Login" |
| "Регистрация" | `login.tab_register` | "Register" |
| "Настройки" | `login.tab_settings` | "Settings" |
| "Логин" | `login.username` | "Username" |
| "Пароль" | `login.password` | "Password" |
| "Войти" | `login.button_login` | "Log In" |
| "Зарегистрироваться" | `login.button_register` | "Register" |
| "Адрес сервера" | `settings.server_address` | "Server Address" |
| "SOCKS5 прокси (dnstt)" | `settings.proxy_address` | "SOCKS5 Proxy (dnstt)" |
| "Прямое подключение (без прокси)" | `settings.direct_mode` | "Direct connection (without proxy)" |
| "Сохранить" | `settings.button_save` | "Save" |
| "Серверы сети (нажми чтобы выбрать)" | `settings.known_servers` | "Network servers (click to select)" |
| "# Общий чат" | `chat.title_general` | "# General" |
| "Сообщение..." | `chat.input_placeholder` | "Message..." |
| "Сообщение для {user}..." | `chat.input_placeholder_dm` | "Message to {user}..." |
| "Сообщение в комнату..." | `chat.input_placeholder_room` | "Message to room..." |
| "Отправить" | `chat.button_send` | "Send" |
| "Меню" | `sidebar.button_menu` | "Menu" |
| "Выйти" | `sidebar.button_logout` | "Log Out" |
| "Чаты" | `sidebar.section_chats` | "Chats" |
| "Личные сообщения" | `sidebar.section_dms` | "Direct Messages" |
| "Комнаты" | `sidebar.section_rooms` | "Rooms" |
| "Онлайн ({count})" | `sidebar.online_count` | "Online ({count})" |
| "Загрузка комнаты..." | `chat.loading_room` | "Loading room..." |
| "{count} участников" | `room.members_count` | "{count} members" |
| "Новое личное сообщение" | `dm.new_conversation` | "New Direct Message" |
| "Логин пользователя" | `dm.target_user` | "Username" |
| "Отмена" | `dm.button_cancel` | "Cancel" |
| "Открыть" | `dm.button_open` | "Open" |
| "Создать комнату" | `room.create_title` | "Create Room" |
| "Название комнаты" | `room.name` | "Room Name" |
| "Описание (необязательно)" | `room.description` | "Description (optional)" |
| "Публичная (видна всем)" | `room.public` | "Public (visible to all)" |
| "Создать" | `room.button_create` | "Create" |
| "Пригласить" | `room.button_invite` | "Invite" |
| "Покинуть" | `room.button_leave` | "Leave" |
| "Пригласить в комнату" | `room.invite_title` | "Invite to Room" |

### MessengerViewModel.kt

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "Заполните все поля" | `error.fill_all_fields` | "Please fill in all fields" |
| "Подключение..." | `status.connecting` | "Connecting..." |
| "Ошибка подключения: {error}" | `error.connection_failed` | "Connection error: {error}" |
| "Таймаут регистрации" | `error.registration_timeout` | "Registration timeout" |
| "Аккаунт создан! Теперь войдите." | `success.account_created` | "Account created! Now log in." |
| "Логин уже занят" | `error.username_taken` | "Username already taken" |
| "Сервис не готов, попробуйте снова" | `error.service_not_ready` | "Service not ready, please try again" |
| "Авторизация..." | `status.authorizing` | "Authorizing..." |
| "Неверный логин или пароль" | `error.invalid_credentials` | "Invalid username or password" |
| "Ошибка входа: {error}" | `error.login_failed` | "Login error: {error}" |
| "Соединение разорвано" | `status.connection_lost` | "Connection lost" |

### Notifications.kt

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "Сообщения" | `notification.messages_channel` | "Messages" |
| "Входящие сообщения" | `notification.incoming_messages` | "Incoming messages" |
| "Фоновое соединение" | `notification.background_connection` | "Background connection" |
| "Поддержание соединения с сервером" | `notification.background_description` | "Maintaining server connection" |

## Electron Client (JavaScript)

### index.html

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "🔐 DNSTT Messenger" | `app.name` | "🔐 DNSTT Messenger" |
| "Вход" | `login.tab_login` | "Login" |
| "Регистрация" | `login.tab_register` | "Register" |
| "Настройки" | `login.tab_settings` | "Settings" |
| "Логин" | `login.username` | "Username" |
| "Пароль" | `login.password` | "Password" |
| "Войти" | `login.button_login` | "Log In" |
| "Зарегистрироваться" | `login.button_register` | "Register" |
| "Адрес сервера" | `settings.server_address` | "Server Address" |
| "SOCKS5 прокси (dnstt)" | `settings.proxy_address` | "SOCKS5 Proxy (dnstt)" |
| "Прямое подключение (без прокси)" | `settings.direct_mode` | "Direct connection (without proxy)" |
| "Сохранить" | `settings.button_save` | "Save" |
| "Серверы сети (нажми чтобы переключиться)" | `settings.known_servers` | "Network servers (click to select)" |
| "Появится после входа" | `settings.known_servers_hint` | "Will appear after login" |
| "Выйти" | `sidebar.button_logout` | "Log Out" |
| "Глобальный чат" | `chat.title_global` | "Global Chat" |
| "Общий" | `chat.title_general` | "# General" |
| "Личные сообщения" | `sidebar.section_dms` | "Direct Messages" |
| "Комнаты" | `sidebar.section_rooms` | "Rooms" |
| "Онлайн" | `sidebar.section_online` | "Online" |
| "Общий чат" | `chat.title_general` | "# General" |
| "Сообщение..." | `chat.input_placeholder` | "Message..." |
| "Новое личное сообщение" | `dm.new_conversation` | "New Direct Message" |
| "Логин пользователя" | `dm.target_user` | "Username" |
| "Отмена" | `dm.button_cancel` | "Cancel" |
| "Открыть" | `dm.button_open` | "Open" |
| "Создать комнату" | `room.create_title` | "Create Room" |
| "Название комнаты" | `room.name` | "Room Name" |
| "Описание (необязательно)" | `room.description` | "Description (optional)" |
| "Публичная (видна всем)" | `room.public` | "Public (visible to all)" |
| "Создать" | `room.button_create` | "Create" |
| "Пригласить в комнату" | `room.invite_title` | "Invite to Room" |
| "Пригласить" | `room.button_invite_ok` | "Invite" |

### app.js

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "Настройки сохранены" | `settings.saved` | "Settings saved" |
| "Заполните все поля" | `error.fill_all_fields` | "Please fill in all fields" |
| "Подключение..." | `status.connecting` | "Connecting..." |
| "Ошибка: {error}" | `error.connection_failed` | "Connection error: {error}" |
| "Аккаунт создан! Теперь войдите." | `success.account_created` | "Account created! Now log in." |
| "Логин уже занят" | `error.username_taken` | "Username already taken" |
| "Ошибка подключения: {error}" | `error.connection_failed` | "Connection error: {error}" |
| "Неверный логин или пароль" | `error.invalid_credentials` | "Invalid username or password" |
| "— конец истории —" | `chat.history_divider` | "— end of history —" |
| "Соединение разорвано" | `status.connection_lost` | "Connection lost" |
| "Вас пригласил в комнату #{name} пользователь {inviter}" | `room.invited_by` | "You were invited to room #{name} by {inviter}" |
| "{user} вошёл в комнату" | `room.member_joined` | "{user} joined the room" |
| "{user} покинул комнату" | `room.member_left` | "{user} left the room" |
| "# Общий чат" | `chat.title_general` | "# General" |
| "# Комната {id}" | `room.title` | "# Room {id}" |
| "Покинуть" | `room.button_leave` | "Leave" |
| "Вы" | `chat.you` | "You" |
| "Список пуст" | `settings.server_list_empty` | "Server list is empty" |
| "Сервер выбран: {addr}" | `settings.server_selected` | "Server selected: {server}" |

## Go CLI Client (main.go)

| Current Russian String | Translation Key | English Value |
|------------------------|-----------------|---------------|
| "🌐 Режим: Direct Connect \| Подключение к: {server}..." | `status.mode_direct` | "Mode: Direct Connect \| Connecting to: {server}..." |
| "🌐 Режим: DNSTT Proxy (SOCKS5) \| Прокси: {proxy} -> Сервер: {server}..." | `status.mode_proxy` | "Mode: DNSTT Proxy (SOCKS5) \| Proxy: {proxy} -> Server: {server}..." |
| "❌ Ошибка создания SOCKS5 диалера: {error}" | `error.socks5_failed` | "SOCKS5 dialer creation error: {error}" |
| "❌ Ошибка подключения: {error}" | `error.connection_failed` | "Connection error: {error}" |
| "🔐 Защищённый канал установлен." | `status.connected` | "Secure channel established" |
| "❌ ECDH хендшейк не удался: {error}" | `error.ecdh_failed` | "ECDH handshake failed: {error}" |
| "❌ Ошибка E2E ключа: {error}" | `error.e2e_key_failed` | "E2E key error: {error}" |
| "\n1. Вход\n2. Регистрация" | `login.prompt_choice` | "1. Login\n2. Register" |
| "❌ Введите 1 или 2." | `error.invalid_choice` | "Please enter 1 or 2" |
| "Логин: " | `login.prompt_username` | "Username: " |
| "❌ Логин не может быть пустым." | `error.username_empty` | "Username cannot be empty" |
| "Пароль: " | `login.prompt_password` | "Password: " |
| "❌ Пароль не может быть пустым." | `error.password_empty` | "Password cannot be empty" |
| "❌ Логин и пароль не должны превышать 255 символов." | `error.credentials_too_long` | "Username and password must not exceed 255 characters" |
| "❌ Ошибка связи: {error}" | `error.communication` | "Communication error: {error}" |
| "✨ Аккаунт создан! Теперь войдите." | `success.account_created` | "Account created! Now log in." |
| "❌ Логин уже занят." | `error.username_taken` | "Username already taken" |
| "❌ Неверный логин или пароль." | `error.invalid_credentials` | "Invalid username or password" |
| "\n--- История чата ---" | `status.history_header` | "--- Chat History ---" |
| "--- Конец истории ---" | `status.history_end` | "--- End of History ---" |
| "✅ Авторизация успешна! (/exit — выход, /servers — серверы, /dm <user> <text> — личное сообщение," | `status.login_success` + `command.help_*` | "Authorization successful!" + command help |
| "   /rooms — список комнат, /join <id> — войти в комнату, /leave <id> — покинуть," | `command.help_*` | Command help strings |
| "   /create <name> [pub] — создать комнату, /room <id> <text> — сообщение в комнату," | `command.help_*` | Command help strings |
| "   /invite <roomID> <user> — пригласить)" | `command.help_*` | Command help strings |
| "📡 Список серверов пуст." | `server.list_empty` | "Server list is empty" |
| "📡 Известные серверы ({count}):" | `server.list_title` | "Known servers ({count}):" |
| "🏠 Нет доступных комнат." | `room.list_empty` | "No available rooms" |
| "🏠 Комнаты ({count}):" | `room.list_title` | "Rooms ({count}):" |
| "Использование: /dm <user> <text>" | `command.usage_dm` | "Usage: /dm <user> <text>" |
| "Использование: /join <roomID>" | `command.usage_join` | "Usage: /join <roomID>" |
| "Использование: /leave <roomID>" | `command.usage_leave` | "Usage: /leave <roomID>" |
| "Использование: /create <name> [pub]" | `command.usage_create` | "Usage: /create <name> [pub]" |
| "Использование: /room <roomID> <text>" | `command.usage_room` | "Usage: /room <roomID> <text>" |
| "Использование: /invite <roomID> <user>" | `command.usage_invite` | "Usage: /invite <roomID> <user>" |
| "❌ E2E ключ не инициализирован" | `error.e2e_not_initialized` | "E2E key not initialized" |
| "⏳ Ожидаем ключи ({users})..." | `server.waiting_for_keys` | "Waiting for keys ({users})..." |
| "❌ Сообщение слишком большое для фрагментации" | `error.message_too_large` | "Message too large for fragmentation" |
| "\n📡 Соединение закрыто сервером." | `status.disconnected` | "Connection closed" |
| "🟢 Онлайн ({count}): {users}" | `misc.online_users` | "Online ({count}): {users}" |
| "\n📡 Серверы сети ({count}): {servers}" | `server.network_servers` | "Network servers ({count}): {servers}" |
| "\n📨 [{time}] [{sender}]: {text}" | `misc.message_received` | "[{time}] [{sender}]: {text}" |
| "  [{time}] {sender}: {text}" | `misc.message_history` | "[{time}] {sender}: {text}" |
| ">> " | `misc.prompt` | ">> " |
| "🔑 E2E ключ загружен из e2e_key.json" | `misc.e2e_key_loaded` | "E2E key loaded from e2e_key.json" |
| "🔑 Новый E2E ключ сгенерирован и сохранён в e2e_key.json" | `misc.e2e_key_generated` | "New E2E key generated and saved to e2e_key.json" |
| "⚠️ Конфиг не найден, использую настройки по умолчанию." | `server.config_not_found` | "Config not found, using default settings" |
| "💬 [DM → {user}]: {text}" | `dm.sent_to` | "[DM → {user}]: {text}" |

## Summary

- **Total unique translation keys**: 130+
- **Android client strings**: ~50 hardcoded Russian strings
- **Electron client strings**: ~45 hardcoded Russian strings  
- **Go CLI client strings**: ~60 hardcoded Russian strings

All strings have been mapped to a consistent translation key namespace that will be used across all three platforms.
