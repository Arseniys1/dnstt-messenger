# DNSTT Messenger - Full Guide (English)

End-to-end encrypted messenger for restrictive networks.  
Traffic can be tunneled through DNS with MasterDnsVPN.

## Contents

- Part A - Server setup (MasterDnsVPN + messenger server)
- Part B - Client setup (Windows/Electron/Android)
- Part C - Build from source
- Troubleshooting and security notes

---

## Part A - Server Setup

### A1. Prerequisites

- Linux VPS with public IP
- Domain you control
- Basic terminal access
- Open ports:
  - `53/udp` and `53/tcp` for DNS
  - `9999/tcp` for messenger server
  - `9998/tcp` only if federation is enabled

### A2. DNS records for tunnel domain

Example:

- Base domain: `example.com`
- Tunnel zone: `t.example.com`
- Name server host: `ns1.example.com`

Create DNS records at your registrar/DNS provider:

1. `A` record: `ns1.example.com -> <VPS_IP>`
2. `NS` record: `t.example.com -> ns1.example.com`

After propagation, requests to `*.t.example.com` should reach your VPS DNS service.

### A3. Install and run MasterDnsVPN server

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

Optional systemd service:

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

### A4. Run messenger server

Create `config.json` near `server` binary:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Start:

```bash
./server
```

### A5. Optional federation

If you run multiple servers:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50,
  "s2s_addr": "0.0.0.0:9998",
  "public_addr": "YOUR_PUBLIC_IP_OR_HOST:9999",
  "gossip_enabled": true,
  "gossip_interval_sec": 60,
  "peers": ["PEER_S2S_ADDR:9998"],
  "s2s_secret": "shared_secret_on_all_federated_nodes",
  "federation_sync_days": 7
}
```

`s2s_secret` must match on all peers.

### A6. What clients need

Share with users:

- `server_addr` (for messenger)
- tunnel domain (`DOMAINS`)
- tunnel encryption key (`ENCRYPTION_KEY`)

---

## Part B - Client Setup

### B1. MasterDnsVPN client

1. Download from releases:  
   [https://github.com/masterking32/MasterDnsVPN/releases](https://github.com/masterking32/MasterDnsVPN/releases)
2. Extract archive.
3. Edit `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. Start MasterDnsVPN client and keep it running.

### B2. Messenger config

Set client config:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. Sign in and settings

- Open app settings (available inside session)
- Set language, server, proxy, and save
- Register account or sign in

### B4. Android note

- Install APK or build from `android-client`
- Use the same server/proxy/tunnel parameters as desktop clients

---

## Part C - Build From Source

### C1. Requirements

- Go `1.21+`
- Node.js `18+` and `npm`
- Android Studio / Android SDK (for Android client)

### C2. Go server/client

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

### C3. Electron

```bash
cd electron-client
npm install
npm start
```

Build packages:

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

---

## Troubleshooting

- Connection refused:
  - Check `server_addr`
  - Check firewall rules
  - Check server process is running
- Tunnel connected but no chat:
  - Verify `DOMAINS` and `ENCRYPTION_KEY` match server
  - Verify SOCKS5 endpoint `127.0.0.1:18000`
- Login fails:
  - Confirm username/password
  - Check server logs for errors

---

## Security Notes

- Keep tunnel keys private.
- Use strong unique passwords.
- Rotate keys if compromise is suspected.
- Keep server and dependencies updated.
