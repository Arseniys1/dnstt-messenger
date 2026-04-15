# DNSTT Messenger - Tam Rehber (Türkçe)

DNSTT Messenger, MasterDnsVPN ile DNS tünelleme üzerinden çalışabilen, uçtan uca şifreli (E2E) bir mesajlaşma sistemidir.

## İçindekiler

- Bölüm A - Sunucu kurulumu (MasterDnsVPN + mesaj sunucusu)
- Bölüm B - İstemci kurulumu (Windows/Electron/Android)
- Bölüm C - Kaynaktan derleme
- Sorun giderme ve güvenlik notları

---

## Bölüm A - Sunucu Kurulumu

### A1. Gereksinimler

- Genel IP'li Linux VPS
- Kontrol ettiğiniz bir alan adı
- Temel terminal bilgisi
- Açık portlar:
  - `53/udp` ve `53/tcp` (DNS)
  - `9999/tcp` (mesaj sunucusu)
  - `9998/tcp` (yalnızca federation için)

### A2. Tünel alan adı için DNS kayıtları

Örnek:

- Ana alan adı: `example.com`
- Tünel alanı: `t.example.com`
- NS host: `ns1.example.com`

Kayıtlar:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

### A3. MasterDnsVPN Server kurulum/çalıştırma

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

`server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

Çalıştır:

```bash
./MasterDnsVPN_Server
```

### A4. Mesaj sunucusunu çalıştırma

`server` ikilisinin yanına `config.json`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Başlat:

```bash
./server
```

### A5. Federation (isteğe bağlı)

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

### A6. İstemcilere verilecek bilgiler

- `server_addr`
- tünel alan adı (`DOMAINS`)
- tünel şifreleme anahtarı (`ENCRYPTION_KEY`)

---

## Bölüm B - İstemci Kurulumu

### B1. MasterDnsVPN Client ayarı

1. Releases sayfasından indir:  
   [https://github.com/masterking32/MasterDnsVPN/releases](https://github.com/masterking32/MasterDnsVPN/releases)
2. Arşivi çıkar.
3. `client_config.toml` dosyasını düzenle:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. MasterDnsVPN’i çalıştır ve açık bırak.

### B2. Messenger istemci ayarı

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. Giriş ve uygulama içi ayarlar

- Uygulama içi Settings ekranını aç
- Dil/sunucu/proxy ayarlarını kaydet
- Kayıt ol veya giriş yap

### B4. Android

- APK kur veya `android-client` klasöründen derle
- Masaüstündeki aynı bağlantı değerlerini kullan

---

## Bölüm C - Kaynaktan Derleme

### C1. Gerekli araçlar

- Go `1.21+`
- Node.js `18+` + `npm`
- Android Studio / Android SDK

### C2. Go

```bash
go build -o server ./server
go build -o client ./client
```

### C3. Electron

```bash
cd electron-client
npm install
npm start
```

Paketleme:

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

---

## Sorun Giderme

- Bağlantı kurulamıyor:
  - `server_addr` kontrol et
  - firewall portlarını kontrol et
  - servislerin çalıştığını doğrula
- Tünel açık ama mesajlar gitmiyor:
  - `DOMAINS` ve `ENCRYPTION_KEY` aynı olmalı
  - SOCKS5 `127.0.0.1:18000` olmalı
- Giriş hatası:
  - kullanıcı adı/parola kontrol et
  - sunucu loglarını incele

---

## Güvenlik Notları

- Tünel anahtarını gizli tut.
- Güçlü parolalar kullan.
- Sunucu ve bağımlılıkları güncel tut.
- Sızıntı şüphesinde anahtarları hemen değiştir.
