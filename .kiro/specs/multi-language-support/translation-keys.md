# Translation Key Namespace

This document defines the shared translation key namespace used across all three DNSTT Messenger clients (Android, Electron, Go CLI).

## Key Structure

Translation keys follow a hierarchical dot-notation structure:
- `app.*` - Application-level strings
- `login.*` - Login and registration screen
- `chat.*` - Chat interface
- `settings.*` - Settings and configuration
- `error.*` - Error messages
- `notification.*` - Notification text
- `dm.*` - Direct messages
- `room.*` - Room/channel functionality
- `status.*` - Status messages
- `command.*` - CLI command help (Go client only)

## Complete Key Inventory

### Application
- `app.name` - "DNSTT Messenger"
- `app.title` - "DNSTT Messenger"

### Login & Registration
- `login.title` - "Login"
- `login.tab_login` - "Login"
- `login.tab_register` - "Register"
- `login.tab_settings` - "Settings"
- `login.username` - "Username"
- `login.password` - "Password"
- `login.button_login` - "Log In"
- `login.button_register` - "Register"
- `login.prompt_choice` - "1. Login\n2. Register"
- `login.prompt_input` - "> "
- `login.prompt_username` - "Username: "
- `login.prompt_password` - "Password: "

### Settings
- `settings.title` - "Settings"
- `settings.server_address` - "Server Address"
- `settings.proxy_address` - "SOCKS5 Proxy (dnstt)"
- `settings.direct_mode` - "Direct connection (without proxy)"
- `settings.button_save` - "Save"
- `settings.saved` - "Settings saved"
- `settings.known_servers` - "Network servers (click to select)"
- `settings.known_servers_hint` - "Will appear after login"
- `settings.server_list_empty` - "Server list is empty"
- `settings.server_selected` - "Server selected: {server}"
- `settings.language` - "Language"

### Chat Interface
- `chat.title_global` - "Global Chat"
- `chat.title_general` - "# General"
- `chat.input_placeholder` - "Message..."
- `chat.input_placeholder_dm` - "Message to {user}..."
- `chat.input_placeholder_room` - "Message to room..."
- `chat.button_send` - "Send"
- `chat.history_divider` - "— end of history —"
- `chat.loading_room` - "Loading room..."
- `chat.you` - "You"

### Sidebar Navigation
- `sidebar.section_chats` - "Chats"
- `sidebar.section_dms` - "Direct Messages"
- `sidebar.section_rooms` - "Rooms"
- `sidebar.section_online` - "Online"
- `sidebar.online_count` - "Online ({count})"
- `sidebar.button_logout` - "Log Out"
- `sidebar.button_menu` - "Menu"

### Direct Messages
- `dm.title` - "Direct Messages"
- `dm.new_conversation` - "New Direct Message"
- `dm.target_user` - "Username"
- `dm.button_cancel` - "Cancel"
- `dm.button_open` - "Open"
- `dm.sent_to` - "[DM → {user}]: {text}"

### Rooms
- `room.title` - "Rooms"
- `room.create` - "Create Room"
- `room.create_title` - "Create Room"
- `room.name` - "Room Name"
- `room.description` - "Description (optional)"
- `room.public` - "Public (visible to all)"
- `room.button_create` - "Create"
- `room.button_cancel` - "Cancel"
- `room.button_invite` - "Invite"
- `room.button_leave` - "Leave"
- `room.invite_title` - "Invite to Room"
- `room.invite_username` - "Username"
- `room.button_invite_ok` - "Invite"
- `room.members_count` - "{count} members"
- `room.member_joined` - "{user} joined the room"
- `room.member_left` - "{user} left the room"
- `room.invited_by` - "You were invited to room #{name} by {inviter}"
- `room.list_empty` - "No available rooms"
- `room.list_title` - "Rooms ({count}):"
- `room.created` - "Room created: {name} (ID: {id}, public: {public})"

### Status Messages
- `status.connecting` - "Connecting..."
- `status.authorizing` - "Authorizing..."
- `status.connected` - "Secure channel established"
- `status.disconnected` - "Connection closed"
- `status.connection_lost` - "Connection lost"
- `status.mode_direct` - "Mode: Direct Connect | Connecting to: {server}..."
- `status.mode_proxy` - "Mode: DNSTT Proxy (SOCKS5) | Proxy: {proxy} -> Server: {server}..."
- `status.login_success` - "Authorization successful!"
- `status.history_header` - "--- Chat History ---"
- `status.history_end` - "--- End of History ---"

### Error Messages
- `error.connection_failed` - "Connection error: {error}"
- `error.connection_timeout` - "Connection timeout"
- `error.invalid_credentials` - "Invalid username or password"
- `error.login_failed` - "Login error: {error}"
- `error.registration_timeout` - "Registration timeout"
- `error.username_taken` - "Username already taken"
- `error.fill_all_fields` - "Please fill in all fields"
- `error.username_empty` - "Username cannot be empty"
- `error.password_empty` - "Password cannot be empty"
- `error.credentials_too_long` - "Username and password must not exceed 255 characters"
- `error.invalid_choice` - "Please enter 1 or 2"
- `error.communication` - "Communication error: {error}"
- `error.socks5_failed` - "SOCKS5 dialer creation error: {error}"
- `error.ecdh_failed` - "ECDH handshake failed: {error}"
- `error.e2e_key_failed` - "E2E key error: {error}"
- `error.e2e_not_initialized` - "E2E key not initialized"
- `error.message_too_large` - "Message too large for fragmentation"
- `error.service_not_ready` - "Service not ready, please try again"
- `error.not_connected` - "Not connected"

### Success Messages
- `success.account_created` - "Account created! Now log in."
- `success.settings_saved` - "Settings saved"

### Notification Messages
- `notification.new_message` - "New message"
- `notification.incoming_messages` - "Incoming messages"
- `notification.background_connection` - "Background connection"
- `notification.background_description` - "Maintaining server connection"
- `notification.messages_channel` - "Messages"
- `notification.messages_description` - "Incoming messages"

### Command Help (Go CLI)
- `command.help_exit` - "/exit — exit"
- `command.help_servers` - "/servers — servers"
- `command.help_dm` - "/dm <user> <text> — direct message"
- `command.help_rooms` - "/rooms — list rooms"
- `command.help_join` - "/join <id> — join room"
- `command.help_leave` - "/leave <id> — leave room"
- `command.help_create` - "/create <name> [pub] — create room"
- `command.help_room_message` - "/room <id> <text> — message to room"
- `command.help_invite` - "/invite <roomID> <user> — invite"
- `command.usage_dm` - "Usage: /dm <user> <text>"
- `command.usage_join` - "Usage: /join <roomID>"
- `command.usage_leave` - "Usage: /leave <roomID>"
- `command.usage_create` - "Usage: /create <name> [pub]"
- `command.usage_room` - "Usage: /room <roomID> <text>"
- `command.usage_invite` - "Usage: /invite <roomID> <user>"

### Server & Network
- `server.list_title` - "Known servers ({count}):"
- `server.list_empty` - "Server list is empty"
- `server.network_servers` - "Network servers ({count}): {servers}"
- `server.waiting_for_keys` - "Waiting for keys ({users})..."
- `server.config_not_found` - "Config not found, using default settings"

### Miscellaneous
- `misc.online_users` - "Online ({count}): {users}"
- `misc.message_received` - "[{time}] [{sender}]: {text}"
- `misc.message_history` - "[{time}] {sender}: {text}"
- `misc.prompt` - ">> "
- `misc.e2e_key_loaded` - "E2E key loaded from e2e_key.json"
- `misc.e2e_key_generated` - "New E2E key generated and saved to e2e_key.json"

## Notes

1. Keys with `{variable}` placeholders support parameter substitution
2. All clients should implement the same keys for consistency
3. Android uses string resources (strings.xml), Electron and Go use JSON files
4. Some keys are platform-specific (e.g., CLI commands for Go client)
5. RTL languages (Arabic, Farsi) may require layout adjustments in addition to text translation
