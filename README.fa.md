# DNSTT Messenger

یک پیام‌رسان امن با رمزنگاری سرتاسری (E2E) و انتقال داده از طریق تونل DNS برای شبکه‌های محدود یا سانسورشده.

## زبان‌های مستندات

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## فهرست

- [هدف پروژه](#هدف-پروژه)
- [قابلیت‌های اصلی](#قابلیتهای-اصلی)
- [بخش A: راه‌اندازی سرور](#بخش-a-راهاندازی-سرور)
- [بخش B: تنظیم کلاینت](#بخش-b-تنظیم-کلاینت)
- [بخش C: ساخت از سورس](#بخش-c-ساخت-از-سورس)
- [عیب‌یابی](#عیبیابی)
- [توصیه‌های امنیتی](#توصیههای-امنیتی)
- [مجوز و مسئولیت](#مجوز-و-مسئولیت)

## هدف پروژه

DNSTT Messenger برای شرایطی طراحی شده که کانال‌های رایج ارتباطی توسط DPI یا فیلترینگ DNS مختل می‌شوند.
انتقال از طریق DNS کمک می‌کند ارتباط حتی در محدودیت HTTPS/VPN برقرار بماند.

## قابلیت‌های اصلی

- پیام‌رسانی با رمزنگاری سرتاسری بین کلاینت‌ها (E2E).
- انتقال پیام روی تونل DNS با MasterDnsVPN.
- کلاینت برای Windows (Go CLI)، Electron (دسکتاپ) و Android.
- پشتیبانی از رابط چندزبانه.
- پشتیبانی اختیاری از federation بین چند سرور.

## بخش A: راه‌اندازی سرور

### A1. پیش‌نیازها

- VPS لینوکسی با IP عمومی.
- دامنه با دسترسی مدیریت DNS.
- دسترسی پایه SSH/ترمینال.
- پورت‌های باز:
  - `53/udp` و `53/tcp` برای تونل DNS.
  - `9999/tcp` برای سرور پیام‌رسان.
  - `9998/tcp` فقط در حالت federation.

### A2. تنظیم DNS تونل

نمونه:

- دامنه اصلی: `example.com`
- دامنه تونل: `t.example.com`
- میزبان NS: `ns1.example.com`

رکوردها:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

بررسی:

- `dig NS example.com`
- `dig t.example.com`

### A3. نصب MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

ویرایش `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

اجرا:

```bash
./MasterDnsVPN_Server
```

### A4. اجرای سرور پیام‌رسان

فایل `config.json` کنار باینری `server`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

اجرا:

```bash
./server
```

### A5. Federation (اختیاری)

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

`s2s_secret` باید در تمام نودهای federation یکسان باشد.

### A6. اطلاعاتی که باید به کاربران بدهید

- `server_addr` (مثال: `1.2.3.4:9999`)
- `DOMAINS` (دامنه تونل)
- `ENCRYPTION_KEY` (کلید تونل)

## بخش B: تنظیم کلاینت

### B1. تنظیم MasterDnsVPN Client

1. دریافت از [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases).
2. استخراج فایل‌ها.
3. ویرایش `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. MasterDnsVPN را اجرا کرده و باز نگه دارید.

### B2. تنظیم Messenger

ویرایش `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. اجرای کلاینت‌ها

- Go CLI: در ویندوز `client.exe` و در لینوکس/macOS `./client`
- Electron: اجرا از پوشه `electron-client`
- Android: نصب APK از `android-client` یا خروجی release

### B4. ورود و ثبت‌نام

1. کلاینت را اجرا کنید.
2. نام کاربری و رمز عبور را وارد کنید.
3. در اجرای اول حساب بسازید.
4. بعد از ورود، زبان/پروکسی/سرور را بررسی کنید.

### B5. نکته Android

- همان مقادیر `server_addr`، `DOMAINS` و `ENCRYPTION_KEY` دسکتاپ را استفاده کنید.
- مطمئن شوید پراکسی مطابق روش اجرای شما تنظیم شده است.

## بخش C: ساخت از سورس

### C1. پیش‌نیاز ابزارها

- `Go 1.21+`
- `Node.js 18+` و `npm`
- `Android Studio` + `Android SDK`

### C2. ساخت Go server/client

```bash
go build -o server ./server
go build -o client ./client
```

نمونه cross-build:

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

بسته‌سازی:

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

## عیب‌یابی

### کلاینت وصل نمی‌شود

- `server_addr` را بررسی کنید.
- پورت‌ها و firewall را بررسی کنید.
- مطمئن شوید سرویس‌ها در حال اجرا هستند.

### تونل وصل است ولی چت کار نمی‌کند

- `DOMAINS` و `ENCRYPTION_KEY` باید با سرور یکسان باشد.
- آدرس SOCKS5 باید `127.0.0.1:18000` باشد.

### خطای ورود

- نام کاربری/رمز عبور را بررسی کنید.
- لاگ‌های احراز هویت سرور را بررسی کنید.

### مشکل زبان یا کاراکترهای خراب (Mojibake)

- فایل‌های ترجمه باید UTF-8 باشند.
- fallback locale و کلیدهای ترجمه را چک کنید.

## توصیه‌های امنیتی

- از کلیدهای بلند و تصادفی استفاده کرده و دوره‌ای بچرخانید.
- آدرس سرور production را عمومی منتشر نکنید.
- محیط تست و production را جدا نگه دارید.
- دسترسی SSH را محدود کنید (کلید، firewall، fail2ban).
- داده حساس را در لاگ‌ها به حداقل برسانید.

## مجوز و مسئولیت

قبل از استقرار production، مجوز فعلی پروژه را در همین مخزن بررسی کنید.
رعایت قوانین محلی و سیاست‌های سازمانی بر عهده شما است.
