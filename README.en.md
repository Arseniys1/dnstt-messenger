# DNSTT Messenger (English)

DNSTT Messenger is an end-to-end encrypted messenger that can route traffic through DNS tunneling (via MasterDnsVPN) for restrictive network environments.

## Quick Start (Client)

1. Download and run MasterDnsVPN client.
2. Configure `client_config.toml` (domain + encryption key from your server operator).
3. Start MasterDnsVPN and keep it running.
4. Start DNSTT Messenger client.
5. Open **Settings** (available inside the app), set server/proxy/language, and save.
6. Register or sign in.

## Run Your Own Server

1. Set up DNS records for your tunnel domain.
2. Run MasterDnsVPN server.
3. Run messenger server (`server`) with `config.json`.
4. Share client connection parameters with users:
   - `server_addr`
   - tunnel domain
   - tunnel encryption key

## Build

- Go client/server: `go build`
- Electron client: `npm install && npm start` (or `npm run build:*`)
- Android client: `./gradlew assembleRelease` (or Android Studio)

## Security Notes

- Keep your MasterDnsVPN encryption key private.
- Use strong account passwords.
- Use trusted server infrastructure.
- Rotate keys if compromise is suspected.
