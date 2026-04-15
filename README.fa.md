# DNSTT Messenger - راهنمای کامل (فارسی)

DNSTT Messenger یک پیام‌رسان با رمزنگاری سرتاسری (E2E) است که می‌تواند با MasterDnsVPN از تونل DNS در شبکه‌های محدود استفاده کند.

## فهرست

- بخش A: راه‌اندازی سرور (MasterDnsVPN + سرور پیام‌رسان)
- بخش B: تنظیم کلاینت (Windows/Electron/Android)
- بخش C: ساخت از سورس
- عیب‌یابی و نکات امنیتی

---

## بخش A: راه‌اندازی سرور

### A1. پیش‌نیازها

- VPS لینوکسی با IP عمومی
- دامنه تحت کنترل شما
- دسترسی ترمینال
- پورت‌های لازم:
  - `53/udp` و `53/tcp` برای DNS
  - `9999/tcp` برای سرور پیام‌رسان
  - `9998/tcp` فقط برای federation

### A2. تنظیم DNS برای دامنه تونل

نمونه:

- دامنه اصلی: `example.com`
- دامنه تونل: `t.example.com`
- هاست NS: `ns1.example.com`

رکوردها:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

### A3. نصب و اجرای MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

در `server_config.toml`:

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

### A6. اطلاعات مورد نیاز کاربر

- `server_addr`
- دامنه تونل (`DOMAINS`)
- کلید رمزنگاری تونل (`ENCRYPTION_KEY`)

---

## بخش B: تنظیم کلاینت

### B1. تنظیم MasterDnsVPN Client

1. دریافت از Releases:  
   [https://github.com/masterking32/MasterDnsVPN/releases](https://github.com/masterking32/MasterDnsVPN/releases)
2. استخراج فایل‌ها
3. ویرایش `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. برنامه MasterDnsVPN را باز و فعال نگه دارید.

### B2. تنظیم Messenger Client

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. ورود و تنظیمات داخل برنامه

- وارد Settings شوید (بدون خروج از حساب)
- زبان/سرور/پروکسی را ذخیره کنید
- ثبت‌نام یا ورود انجام دهید

### B4. Android

- APK را نصب کنید یا از `android-client` بسازید
- همان مقادیر اتصال دسکتاپ را وارد کنید

---

## بخش C: ساخت از سورس

### C1. ابزارهای لازم

- Go `1.21+`
- Node.js `18+` و `npm`
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

---

## عیب‌یابی

- اتصال برقرار نمی‌شود:
  - `server_addr` را بررسی کنید
  - پورت‌های فایروال را بررسی کنید
  - اجرای سرویس‌ها را بررسی کنید
- تونل وصل است اما پیام رد نمی‌شود:
  - `DOMAINS` و `ENCRYPTION_KEY` باید یکسان باشند
  - SOCKS5 روی `127.0.0.1:18000` باشد
- خطای ورود:
  - نام کاربری/رمز را بررسی کنید
  - لاگ سرور را بررسی کنید

---

## نکات امنیتی

- کلید تونل را محرمانه نگه دارید.
- از رمز عبور قوی استفاده کنید.
- سرویس‌ها و وابستگی‌ها را به‌روز نگه دارید.
- در صورت احتمال نشت، کلیدها را سریع تغییر دهید.
