# DNSTT Messenger

Secure messenger with end-to-end encryption (E2E) and DNS tunnel transport for restrictive or heavily filtered networks.

## Documentation Languages

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## Contents

- [Project Purpose](#project-purpose)
- [Key Features](#key-features)
- [Part A: Server Setup](#part-a-server-setup)
- [Part B: Client Setup](#part-b-client-setup)
- [Part C: Build From Source](#part-c-build-from-source)
- [Troubleshooting](#troubleshooting)
- [Security Recommendations](#security-recommendations)
- [License and Responsibility](#license-and-responsibility)

## Project Purpose

DNSTT Messenger is designed for cases where standard communication channels are unstable or blocked by DPI and DNS filtering.
DNS transport helps preserve connectivity where HTTPS/VPN may be limited.

## Key Features

- End-to-end encrypted messaging between clients.
- Message transport over DNS tunnel (MasterDnsVPN).
- Clients for Windows (Go CLI), Electron (desktop), and Android.
- Multilingual UI support.
- Optional federation between multiple servers.

## Part A: Server Setup

### A1. Requirements

- Linux VPS with public IP.
- Domain with DNS zone management access.
- Basic SSH/terminal access.
- Open ports:
  - `53/udp` and `53/tcp` for DNS tunnel.
  - `9999/tcp` for messenger server.
  - `9998/tcp` only when federation is enabled.

### A2. DNS Records for Tunnel

Example:

- base domain: `example.com`
- tunnel zone: `t.example.com`
- NS host: `ns1.example.com`

Create records:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

Verify:

- `dig NS example.com`
- `dig t.example.com`

### A3. Install MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

Edit `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

Run:

```bash
./MasterDnsVPN_Server
```

systemd example:

```ini
[Unit]
Description=MasterDnsVPN Server
After=network.target

[Service]
WorkingDirectory=/opt/MasterDnsVPN_Server_Linux_AMD64
ExecStart=/opt/MasterDnsVPN_Server_Linux_AMD64/MasterDnsVPN_Server
Restart=always

[Install]
WantedBy=multi-user.target
```

### A4. Run Messenger Server

Create `config.json` next to `server` binary:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Run:

```bash
./server
```

### A5. Federation (Optional)

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50,
  "s2s_addr": "0.0.0.0:9998",
  "public_addr": "PUBLIC_IP_OR_HOST:9999",
  "gossip_enabled": true,
  "gossip_interval_sec": 60,
  "peers": ["PEER_S2S_ADDR:9998"],
  "s2s_secret": "shared_secret_for_all_nodes",
  "federation_sync_days": 7
}
```

`s2s_secret` must be identical on all federation nodes.

### A6. Data to Share With Users

- `server_addr` (for messenger, e.g. `1.2.3.4:9999`)
- `DOMAINS` (tunnel domain)
- `ENCRYPTION_KEY` (tunnel key)

## Part B: Client Setup

### B1. Configure MasterDnsVPN Client

1. Download from [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases).
2. Extract archive.
3. Edit `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. Start MasterDnsVPN and keep it running.

### B2. Messenger Config

Edit `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. Start Clients

- Go client (CLI): `client.exe` on Windows or `./client` on Linux/macOS
- Electron client: launch from `electron-client`
- Android client: APK from `android-client` or release artifact

### B4. Sign-In and Registration

1. Start client.
2. Enter username and password.
3. Create account on first run.
4. Check language/proxy/server settings after login.

### B5. Android Note

- Use the same `server_addr`, `DOMAINS`, `ENCRYPTION_KEY` values.
- Ensure proxy settings match your Android setup.

## Part C: Build From Source

### C1. Requirements

- `Go 1.21+`
- `Node.js 18+` and `npm`
- `Android Studio` + `Android SDK` (for Android client)

### C2. Build Go Server and Go Client

```bash
go build -o server ./server
go build -o client ./client
```

Cross-build examples:

```bash
GOOS=linux GOARCH=amd64 go build -o server-linux-amd64 ./server
GOOS=windows GOARCH=amd64 go build -o server.exe ./server
GOOS=darwin GOARCH=arm64 go build -o server-mac-arm64 ./server
```

PowerShell:

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o server-linux-amd64 ./server
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o server.exe ./server
Remove-Item Env:GOOS; Remove-Item Env:GOARCH
```

### C3. Electron

```bash
cd electron-client
npm install
npm start
```

Package builds:

```bash
npm run build:win
npm run build:linux
npm run build:mac
```

### C4. Android

```bash
cd android-client
./gradlew assembleRelease
```

Windows:

```cmd
gradlew.bat assembleRelease
```

## Troubleshooting

### Client Cannot Connect

- Verify `server_addr`.
- Check firewall and open ports.
- Ensure server processes are running.

### Tunnel Connected but Chat Fails

- `DOMAINS` and `ENCRYPTION_KEY` must match server values.
- SOCKS5 endpoint must be `127.0.0.1:18000`.

### Login Failure

- Check username/password.
- Inspect server logs for authentication errors.

### Localization Issues (Mojibake)

- Ensure translation files are UTF-8.
- Verify fallback locale and translation keys.

## Security Recommendations

- Use long random keys and rotate them regularly.
- Do not publish production server addresses publicly.
- Separate test and production environments.
- Restrict SSH access (keys, firewall, fail2ban).
- Minimize sensitive data in logs.

## License and Responsibility

Before production deployment, check the current project license in this repository.
You are responsible for compliance with your local laws and your organization policies.
