# 🔒 Секретный мессенджер — инструкция для чайников

> Этот мессенджер прячет твои сообщения внутри обычных DNS-запросов. Даже если кто-то следит за твоим интернетом — он увидит только запросы к сайтам, а не твои слова. Все сообщения зашифрованы — даже сервер не может их прочитать.

---

## Оглавление

- [Часть A — Настройка СЕРВЕРА](#часть-a--настройка-сервера) ← если ты хочешь поднять собственный сервер
- [Часть B — Настройка КЛИЕНТА](#часть-b--настройка-клиента) ← если ты просто хочешь общаться

---

# Часть C — Сборка из исходников

> Эта часть для тех, кто хочет собрать `.exe`, APK и сервер самостоятельно.

---

## Требования

| Компонент | Что нужно |
|-----------|-----------|
| Go-клиент + Сервер | [Go 1.21+](https://go.dev/dl/) |
| Electron-клиент | [Node.js 18+](https://nodejs.org/) + npm |
| Android-клиент | [Android Studio](https://developer.android.com/studio) с SDK 35 |

---

## C1 — Сборка сервера и Go-клиента под все платформы

Go позволяет собирать под любую платформу с любой машины — достаточно задать две переменные `GOOS` и `GOARCH`.

### Быстрая шпаргалка

| Целевая платформа | Команда |
|-------------------|---------|
| Linux x64 (VPS) | `GOOS=linux GOARCH=amd64 go build -o server-linux-amd64 ./server` |
| Linux ARM64 (Raspberry Pi, Oracle Cloud) | `GOOS=linux GOARCH=arm64 go build -o server-linux-arm64 ./server` |
| Windows x64 | `GOOS=windows GOARCH=amd64 go build -o server.exe ./server` |
| macOS Intel | `GOOS=darwin GOARCH=amd64 go build -o server-mac-amd64 ./server` |
| macOS Apple Silicon (M1/M2/M3) | `GOOS=darwin GOARCH=arm64 go build -o server-mac-arm64 ./server` |

Замени `./server` на `./client` чтобы собрать клиент вместо сервера.

### На Windows — через PowerShell

PowerShell не понимает `GOOS=...` перед командой. Используй так:

```powershell
# Linux x64
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o server-linux-amd64 ./server

# Windows x64
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o server.exe ./server

# После сборки — сбрось переменные, иначе все следующие билды тоже будут под Linux
Remove-Item Env:GOOS; Remove-Item Env:GOARCH
```

### Собрать сразу все варианты (bash-скрипт)

```bash
#!/bin/bash
set -e
for TARGET in "linux/amd64" "linux/arm64" "windows/amd64" "darwin/amd64" "darwin/arm64"; do
  OS=${TARGET%/*}
  ARCH=${TARGET#*/}
  EXT=""
  [ "$OS" = "windows" ] && EXT=".exe"
  echo "Building server $OS/$ARCH..."
  GOOS=$OS GOARCH=$ARCH go build -o "dist/server-$OS-$ARCH$EXT" ./server
  echo "Building client $OS/$ARCH..."
  GOOS=$OS GOARCH=$ARCH go build -o "dist/client-$OS-$ARCH$EXT" ./client
done
echo "Done! Files in ./dist/"
```

Сохрани как `build-all.sh`, дай права (`chmod +x build-all.sh`) и запусти.

---

## C2 — Сборка Electron-клиента под разные платформы

Установи зависимости (один раз):

```bash
cd dnstt-messenger/electron-client
npm install
```

Запуск в режиме разработки (без сборки):

```bash
npm start
```

### Сборка под конкретную платформу

| Платформа | Команда | Результат |
|-----------|---------|-----------|
| Windows (NSIS installer) | `npm run build:win` | `dist/DNSTT Messenger Setup x.x.x.exe` |
| Linux (AppImage) | `npm run build:linux` | `dist/DNSTT Messenger-x.x.x.AppImage` |
| macOS (DMG) | `npm run build:mac` | `dist/DNSTT Messenger-x.x.x.dmg` |

> ⚠️ **macOS-билд можно собрать только на macOS** — это ограничение Apple (требуется нотаризация).
> Windows и Linux можно собирать с любой машины.

### Собрать под Windows и Linux одновременно

```bash
npx electron-builder --win --linux
```

Готовые файлы появятся в папке `electron-client/dist/`.

---

## C4 — Сборка Android-клиента

### Вариант А: через Android Studio (проще)

1. Открой Android Studio
2. Нажми **File → Open** и выбери папку `dnstt-messenger/android-client`
3. Подожди пока Gradle скачает зависимости
4. Нажми **Build → Generate Signed Bundle / APK → APK**
5. Следуй инструкциям: создай или выбери keystore, выбери `release`
6. APK появится в `android-client/app/release/app-release.apk`

### Вариант Б: через командную строку

```bash
cd dnstt-messenger/android-client
./gradlew assembleRelease
```

На Windows:

```cmd
cd dnstt-messenger\android-client
gradlew.bat assembleRelease
```

APK появится в `app/build/outputs/apk/release/app-release.apk`.

> ⚠️ Для командной строки нужен `ANDROID_HOME` — путь до Android SDK. Обычно это
> `C:\Users\ИМЯ\AppData\Local\Android\Sdk` на Windows или `~/Android/Sdk` на Linux.
> Можно задать в `android-client/local.properties`:
> ```
> sdk.dir=C\:\\Users\\ИМЯ\\AppData\\Local\\Android\\Sdk
> ```

---

# Часть A — Настройка СЕРВЕРА

> Эта часть нужна только тому, кто хочет **поднять свой сервер**. Если ты просто хочешь подключиться к чужому серверу — пропусти и иди в [Часть B](#часть-b--настройка-клиента).

Для сервера тебе нужно:
- **VPS** (виртуальный сервер) с Linux — например, на Hetzner, DigitalOcean, Selectel
- **Домен** — любой, купленный у регистратора (например, `myvpn.example.com`)
- Умение работать с терминалом

---

## A1 — Настройка DNS для MasterDnsVPN сервера

MasterDnsVPN работает как DNS-туннель. Твой домен должен «смотреть» на твой сервер как NS-запись.

### Что нужно сделать у регистратора домена:

1. Зайди в панель управления доменом
2. Создай A-запись: `ns1.myvpn.example.com` → `IP_твоего_сервера`
3. Создай NS-запись: `t.myvpn.example.com` → `ns1.myvpn.example.com`

Итог: все DNS-запросы к `*.t.myvpn.example.com` будут приходить на твой сервер.

> ⏰ DNS обновляется до 24 часов. Подожди перед тестированием.

---

## A2 — Запуск MasterDnsVPN сервера

Скачай серверную версию MasterDnsVPN:

```bash
# На сервере (Linux)
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

Открой файл `server_config.toml` и найди строку с доменом:

```toml
DOMAINS = ["t.myvpn.example.com"]   # ← замени на свой домен
```

Также запомни или задай ключ шифрования:

```toml
ENCRYPTION_KEY = "сюда_напиши_любой_случайный_набор_букв_32_символа"
```

Запусти сервер:

```bash
./MasterDnsVPN_Server &
```

Или создай systemd-сервис чтобы он запускался автоматически:

```bash
sudo nano /etc/systemd/system/masterdnsvpn.service
```

```ini
[Unit]
Description=MasterDnsVPN Server
After=network.target

[Service]
WorkingDirectory=/root/MasterDnsVPN_Server_Linux_AMD64
ExecStart=/root/MasterDnsVPN_Server_Linux_AMD64/MasterDnsVPN_Server
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable masterdnsvpn
sudo systemctl start masterdnsvpn
```

---

## A3 — Запуск сервера мессенджера

Скачай или скомпилируй `server.exe` / `server` из этого репозитория.

На сервере создай файл `config.json` рядом с `server`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

**Описание полей:**

| Поле | Значение |
|------|----------|
| `listen_addr` | Адрес и порт, на котором слушает мессенджер. `0.0.0.0` = принимать со всех интерфейсов |
| `db_path` | Путь к базе данных (создастся автоматически) |
| `history_limit` | Сколько последних сообщений отдавать при подключении |

Запусти сервер мессенджера:

```bash
./server
```

Или как systemd-сервис (аналогично выше).

> 🔒 Порт `9999` должен быть открыт в firewall:
> ```bash
> sudo ufw allow 9999/tcp
> ```

---

## A4 — Если нужна федерация (несколько серверов)

Если ты хочешь объединить несколько серверов мессенджера в сеть — добавь в `config.json`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50,
  "s2s_addr": "0.0.0.0:9998",
  "public_addr": "IP_этого_сервера:9999",
  "gossip_enabled": true,
  "gossip_interval_sec": 60,
  "peers": ["IP_другого_сервера:9998"],
  "s2s_secret": "одинаковый_секрет_на_всех_серверах",
  "federation_sync_days": 7
}
```

> 📌 `s2s_secret` должен быть **одинаковым** на всех серверах федерации. Также открой порт 9998:
> ```bash
> sudo ufw allow 9998/tcp
> ```

---

## A5 — Настройка клиентов для подключения к твоему серверу

Теперь скажи пользователям изменить в их `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "IP_твоего_сервера:9999",
  "direct_mode": false
}
```

И в их `client_config.toml` (MasterDnsVPN):

```toml
DOMAINS = ["t.myvpn.example.com"]
ENCRYPTION_KEY = "тот_же_ключ_что_на_сервере"
```

---

# Часть B — Настройка КЛИЕНТА

> Эта часть для тех, кто хочет просто общаться через уже готовый сервер.

---

## Что тебе понадобится

- Компьютер с Windows (или телефон Android — см. ниже)
- Программа **MasterDnsVPN** (скачаем ниже)
- Сам **мессенджер** (уже здесь)
- 5 минут времени

---

## Шаг 1 — Скачай MasterDnsVPN

1. Открой браузер и перейди на страницу:
   **https://github.com/masterking32/MasterDnsVPN/releases**

2. Найди последний релиз (сверху). Скачай файл с названием вроде:
   `MasterDnsVPN_Client_Windows_AMD64.zip`

3. Разархивируй скачанный zip в любую папку, например `C:\MasterDnsVPN\`

   > 📁 Внутри будут файлы: `MasterDnsVPN.exe`, `client_config.toml` и другие.

---

## Шаг 2 — Настрой MasterDnsVPN

Открой файл `client_config.toml` в блокноте (правая кнопка мыши → «Открыть с помощью» → «Блокнот»).

Найди и проверь эти строки:

```toml
DOMAINS        = ["t.myvpn.example.com"]   # ← адрес сервера (спроси у администратора)
ENCRYPTION_KEY = "ключ_шифрования"          # ← ключ (спроси у администратора)

LISTEN_IP      = "127.0.0.1"
LISTEN_PORT    = 18000
PROTOCOL_TYPE  = "SOCKS5"
```

Сохрани файл.

> 📌 `DOMAINS` и `ENCRYPTION_KEY` — это то, что тебе должен дать администратор сервера.
> Порт `18000` и остальное менять не нужно.

---

## Шаг 3 — Запусти MasterDnsVPN

Дважды кликни на `MasterDnsVPN.exe`.

Появится чёрное окно — это нормально! Это и есть твой «невидимый туннель». **Не закрывай его** — пусть работает в фоне.

```
[*] MasterDnsVPN запущен
[*] Слушаю на 127.0.0.1:18000
```

Если видишь что-то похожее — всё работает. ✅

---

## Шаг 4 — Настрой мессенджер

Открой файл `client/client_config.json` в блокноте и убедись, что там написано:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "IP_сервера:9999",
  "direct_mode": false
}
```

> 📌 `server_addr` — адрес сервера мессенджера. Спроси у администратора.
> `proxy_addr` должен совпадать с `LISTEN_PORT` из MasterDnsVPN — оба `18000`.

---

## Шаг 5 — Запусти мессенджер

### Вариант А: Программа с окошком (Electron)

Зайди в папку `electron-client` и запусти `messenger.exe`.

### Вариант Б: Консольная версия

Открой командную строку (Win+R → напиши `cmd` → Enter) и выполни:

```
cd C:\путь\до\мессенджера
client.exe
```

---

## Шаг 6 — Зарегистрируйся или войди

При первом запуске мессенджер спросит:

```
Введите логин:
```

Придумай себе имя (только латинские буквы и цифры, например `ivan123`).

Затем:

```
Введите пароль:
```

Придумай пароль. **Запомни его** — восстановить нельзя!

Если такой логин уже занят — попробуй другой (например `ivan_777`).

При следующих запусках просто введи свой логин и пароль снова.

---

## Шаг 7 — Напиши сообщение

Как только войдёшь — увидишь список пользователей онлайн.

Просто набирай текст и нажимай **Enter** — сообщение уйдёт всем онлайн!

```
> Привет всем!
```

---

## Телефон Android


---

## Частые вопросы

**❓ Мессенджер не подключается**

Убедись, что MasterDnsVPN запущен и его чёрное окно открыто.

**❓ Написано "соединение отклонено"**

Проверь:
1. `client_config.json` — правильный `server_addr`?
2. `client_config.toml` — правильный `DOMAINS` и `ENCRYPTION_KEY`?
3. MasterDnsVPN запущен?

**❓ Антивирус ругается на MasterDnsVPN**

Это нормально для VPN-программ. Добавь папку с MasterDnsVPN в исключения антивируса.

**❓ Могут ли прочитать мои сообщения?**

Нет. Сообщения зашифрованы прямо на твоём устройстве до отправки (E2E шифрование). Сервер хранит только непонятный набор байт — даже администратор сервера не может их прочитать.

---

## Как это работает (совсем просто)

```
Ты пишешь сообщение
       ↓
Мессенджер его шифрует (как будто прячет в конверт со сложным замком)
       ↓
MasterDnsVPN прячет этот конверт внутри обычных DNS-запросов
(как будто пишет письмо, спрятанное в счёт за телефон)
       ↓
Интернет-провайдер видит только "обычные DNS-запросы" — ничего подозрительного
       ↓
Сервер мессенджера передаёт зашифрованный конверт получателю
       ↓
Получатель расшифровывает своим ключом и читает
```

Ни провайдер, ни сервер, ни злоумышленник не могут прочитать содержимое — только ты и твой собеседник.
