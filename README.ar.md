# DNSTT Messenger

تطبيق مراسلة آمن بتشفير طرفي كامل (E2E) ونقل عبر نفق DNS للعمل في الشبكات المقيّدة أو الخاضعة للترشيح الشديد.

## لغات التوثيق

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## المحتويات

- [هدف المشروع](#هدف-المشروع)
- [الميزات الأساسية](#الميزات-الأساسية)
- [الجزء A: إعداد الخادم](#الجزء-a-إعداد-الخادم)
- [الجزء B: إعداد العميل](#الجزء-b-إعداد-العميل)
- [الجزء C: البناء من المصدر](#الجزء-c-البناء-من-المصدر)
- [استكشاف الأخطاء](#استكشاف-الأخطاء)
- [توصيات الأمان](#توصيات-الأمان)
- [الترخيص والمسؤولية](#الترخيص-والمسؤولية)

## هدف المشروع

DNSTT Messenger مخصص للحالات التي تتعرض فيها قنوات الاتصال العادية للحجب عبر DPI أو ترشيح DNS.
النقل عبر DNS يساعد على الحفاظ على الاتصال عندما يكون HTTPS/VPN محدودًا.

## الميزات الأساسية

- رسائل مشفرة بين العملاء بتشفير طرفي (E2E).
- نقل الرسائل عبر نفق DNS باستخدام MasterDnsVPN.
- عملاء لـ Windows (Go CLI) وElectron (سطح المكتب) وAndroid.
- واجهة متعددة اللغات.
- دعم اختياري للفدرالية بين عدة خوادم.

## الجزء A: إعداد الخادم

### A1. المتطلبات

- خادم Linux VPS بعنوان IP عام.
- نطاق مع صلاحية إدارة DNS.
- وصول SSH/Terminal أساسي.
- المنافذ المفتوحة:
  - `53/udp` و `53/tcp` لنفق DNS.
  - `9999/tcp` لخادم المراسلة.
  - `9998/tcp` فقط في حالة الفدرالية.

### A2. سجلات DNS للنفق

مثال:

- النطاق الأساسي: `example.com`
- نطاق النفق: `t.example.com`
- خادم NS: `ns1.example.com`

أنشئ السجلات:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

تحقق:

- `dig NS example.com`
- `dig t.example.com`

### A3. تثبيت MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

عدّل `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

التشغيل:

```bash
./MasterDnsVPN_Server
```

### A4. تشغيل خادم المراسلة

أنشئ `config.json` بجانب ملف `server`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

التشغيل:

```bash
./server
```

### A5. الفدرالية (اختياري)

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

يجب أن يكون `s2s_secret` متطابقًا على جميع عقد الفدرالية.

### A6. البيانات التي يجب مشاركتها مع المستخدمين

- `server_addr` (مثال: `1.2.3.4:9999`)
- `DOMAINS` (نطاق النفق)
- `ENCRYPTION_KEY` (مفتاح النفق)

## الجزء B: إعداد العميل

### B1. إعداد MasterDnsVPN Client

1. حمّل من [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases).
2. فك الضغط.
3. عدّل `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. شغّل MasterDnsVPN واتركه يعمل.

### B2. إعداد Messenger

عدّل `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. تشغيل العملاء

- عميل Go CLI: `client.exe` على Windows أو `./client` على Linux/macOS
- عميل Electron: من مجلد `electron-client`
- عميل Android: APK من `android-client` أو إصدار جاهز

### B4. تسجيل الدخول والتسجيل

1. شغّل العميل.
2. أدخل اسم المستخدم وكلمة المرور.
3. أنشئ حسابًا في أول تشغيل.
4. بعد الدخول تأكد من إعدادات اللغة/الوكيل/الخادم.

### B5. ملاحظة Android

- استخدم نفس قيم `server_addr` و`DOMAINS` و`ENCRYPTION_KEY` الخاصة بسطح المكتب.
- تأكد من أن إعدادات الوكيل متوافقة مع طريقة التشغيل.

## الجزء C: البناء من المصدر

### C1. المتطلبات

- `Go 1.21+`
- `Node.js 18+` و `npm`
- `Android Studio` + `Android SDK`

### C2. بناء خادم وعميل Go

```bash
go build -o server ./server
go build -o client ./client
```

أمثلة cross-build:

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

بناء الحزم:

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

## استكشاف الأخطاء

### العميل لا يتصل

- تحقق من `server_addr`.
- تحقق من الجدار الناري والمنافذ.
- تأكد أن خدمات الخادم تعمل.

### النفق يعمل لكن الدردشة لا تعمل

- يجب أن تتطابق `DOMAINS` و`ENCRYPTION_KEY` مع قيم الخادم.
- يجب أن يكون SOCKS5 على `127.0.0.1:18000`.

### فشل تسجيل الدخول

- تحقق من اسم المستخدم وكلمة المرور.
- راجع سجلات الخادم لأخطاء المصادقة.

### مشاكل اللغة (Mojibake)

- تأكد أن ملفات الترجمة محفوظة بترميز UTF-8.
- تحقق من fallback locale ومفاتيح الترجمة.

## توصيات الأمان

- استخدم مفاتيح عشوائية قوية وبدلها دوريًا.
- لا تنشر عناوين خوادم الإنتاج علنًا.
- افصل بين بيئة الاختبار وبيئة الإنتاج.
- قيد الوصول عبر SSH (مفاتيح، firewall، fail2ban).
- قلل البيانات الحساسة في السجلات.

## الترخيص والمسؤولية

قبل النشر في الإنتاج، راجع ترخيص المشروع الحالي في هذا المستودع.
أنت مسؤول عن الالتزام بالقوانين المحلية وسياسات مؤسستك.
