# DNSTT Messenger

Kısıtlı veya yoğun filtrelenen ağlar için DNS tüneli üzerinden çalışan, uçtan uca şifrelemeli (E2E) güvenli mesajlaşma sistemi.

## Dokümantasyon Dilleri

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## İçindekiler

- [Proje Amacı](#proje-amacı)
- [Temel Özellikler](#temel-özellikler)
- [Bölüm A: Sunucu Kurulumu](#bölüm-a-sunucu-kurulumu)
- [Bölüm B: İstemci Kurulumu](#bölüm-b-istemci-kurulumu)
- [Bölüm C: Kaynaktan Derleme](#bölüm-c-kaynaktan-derleme)
- [Sorun Giderme](#sorun-giderme)
- [Güvenlik Önerileri](#güvenlik-önerileri)
- [Lisans ve Sorumluluk](#lisans-ve-sorumluluk)

## Proje Amacı

DNSTT Messenger, klasik iletişim kanallarının DPI veya DNS filtreleme nedeniyle kesildiği durumlar için tasarlandı.
DNS taşıma katmanı, HTTPS/VPN erişimi sınırlı olduğunda bağlantıyı korumaya yardımcı olur.

## Temel Özellikler

- İstemciler arası uçtan uca şifreli mesajlaşma (E2E).
- MasterDnsVPN ile DNS tüneli üzerinden mesaj taşıma.
- Windows (Go CLI), Electron (masaüstü) ve Android istemcileri.
- Çok dilli arayüz desteği.
- Birden fazla sunucu arasında isteğe bağlı federation.

## Bölüm A: Sunucu Kurulumu

### A1. Gereksinimler

- Genel IP'li Linux VPS.
- DNS yönetimi erişimi olan alan adı.
- Temel SSH/terminal erişimi.
- Açık portlar:
  - DNS tüneli için `53/udp` ve `53/tcp`.
  - Mesaj sunucusu için `9999/tcp`.
  - Federation için (opsiyonel) `9998/tcp`.

### A2. Tünel DNS Kayıtları

Örnek:

- ana alan adı: `example.com`
- tünel alanı: `t.example.com`
- NS host: `ns1.example.com`

Kayıtlar:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

Kontrol:

- `dig NS example.com`
- `dig t.example.com`

### A3. MasterDnsVPN Server Kurulumu

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

Çalıştırma:

```bash
./MasterDnsVPN_Server
```

### A4. Mesaj Sunucusunu Çalıştırma

`server` ikilisinin yanına `config.json`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Çalıştırma:

```bash
./server
```

### A5. Federation (Opsiyonel)

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

`s2s_secret`, tüm federation düğümlerinde aynı olmalıdır.

### A6. Kullanıcılara Verilecek Bilgiler

- `server_addr` (ör. `1.2.3.4:9999`)
- `DOMAINS` (tünel alanı)
- `ENCRYPTION_KEY` (tünel anahtarı)

## Bölüm B: İstemci Kurulumu

### B1. MasterDnsVPN Client Yapılandırması

1. [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases) sayfasından indirin.
2. Arşivi çıkarın.
3. `client_config.toml` düzenleyin:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. MasterDnsVPN'i çalıştırın ve açık bırakın.

### B2. Messenger Ayarı

`client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. İstemcileri Başlatma

- Go CLI: Windows `client.exe`, Linux/macOS `./client`
- Electron: `electron-client` klasöründen
- Android: `android-client` içinden APK veya release

### B4. Giriş ve Kayıt

1. İstemciyi açın.
2. Kullanıcı adı ve parola girin.
3. İlk kullanımda hesap oluşturun.
4. Giriş sonrası dil/proxy/sunucu ayarlarını kontrol edin.

### B5. Android Notu

- Masaüstü ile aynı `server_addr`, `DOMAINS`, `ENCRYPTION_KEY` değerlerini kullanın.
- Proxy ayarlarının Android kurulumunuzla uyumlu olduğundan emin olun.

## Bölüm C: Kaynaktan Derleme

### C1. Gereksinimler

- `Go 1.21+`
- `Node.js 18+` ve `npm`
- `Android Studio` + `Android SDK`

### C2. Go Sunucu ve İstemci Derleme

```bash
go build -o server ./server
go build -o client ./client
```

Çapraz derleme örnekleri:

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

Windows:

```cmd
gradlew.bat assembleRelease
```

## Sorun Giderme

### İstemci Bağlanamıyor

- `server_addr` kontrol edin.
- Firewall ve port durumunu kontrol edin.
- Sunucu süreçlerinin çalıştığını doğrulayın.

### Tünel Var Ama Sohbet Çalışmıyor

- `DOMAINS` ve `ENCRYPTION_KEY` sunucu ile aynı olmalı.
- SOCKS5 endpoint `127.0.0.1:18000` olmalı.

### Giriş Hatası

- Kullanıcı adı/parola kontrol edin.
- Kimlik doğrulama hataları için sunucu loglarını inceleyin.

### Yerelleştirme Sorunları (Mojibake)

- Çeviri dosyalarının UTF-8 olduğundan emin olun.
- Fallback locale ve çeviri anahtarlarını kontrol edin.

## Güvenlik Önerileri

- Uzun rastgele anahtarlar kullanın ve düzenli döndürün.
- Production sunucu adreslerini herkese açık paylaşmayın.
- Test ve production ortamlarını ayırın.
- SSH erişimini kısıtlayın (anahtar, firewall, fail2ban).
- Loglarda hassas veriyi minimumda tutun.

## Lisans ve Sorumluluk

Production dağıtımı öncesinde bu depodaki güncel lisansı kontrol edin.
Yerel yasalara ve kurum politikalarına uyumluluk sizin sorumluluğunuzdadır.
