# DNSTT Messenger - Полный гайд (Русский)

Мессенджер со сквозным шифрованием (E2E), который может работать через DNS-туннель (MasterDnsVPN) в сетях с жёсткой цензурой.

## Содержание

- Часть A - Настройка сервера (MasterDnsVPN + сервер мессенджера)
- Часть B - Настройка клиента (Windows/Electron/Android)
- Часть C - Сборка из исходников
- Диагностика и заметки по безопасности

---

## Часть A - Настройка сервера

### A1. Что нужно

- Linux VPS с белым IP
- Ваш домен
- Базовый доступ к терминалу
- Открытые порты:
  - `53/udp` и `53/tcp` для DNS
  - `9999/tcp` для сервера мессенджера
  - `9998/tcp` только если включена federation

### A2. DNS-записи для туннельного домена

Пример:

- Базовый домен: `example.com`
- Зона туннеля: `t.example.com`
- DNS-хост: `ns1.example.com`

Создайте записи:

1. `A`: `ns1.example.com -> <IP_VPS>`
2. `NS`: `t.example.com -> ns1.example.com`

После распространения DNS запросы к `*.t.example.com` должны приходить на ваш VPS.

### A3. Установка и запуск MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

Отредактируйте `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "замените_на_длинный_случайный_ключ"
```

Запуск:

```bash
./MasterDnsVPN_Server
```

Пример systemd:

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

### A4. Запуск сервера мессенджера

Создайте `config.json` рядом с бинарником `server`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Запуск:

```bash
./server
```

### A5. Federation (опционально)

Если у вас несколько серверов:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50,
  "s2s_addr": "0.0.0.0:9998",
  "public_addr": "ПУБЛИЧНЫЙ_IP_ИЛИ_ХОСТ:9999",
  "gossip_enabled": true,
  "gossip_interval_sec": 60,
  "peers": ["АДРЕС_ПИРА:9998"],
  "s2s_secret": "общий_секрет_на_всех_узлах",
  "federation_sync_days": 7
}
```

`s2s_secret` должен совпадать на всех нодах.

### A6. Что нужно передать клиентам

- `server_addr` (мессенджер)
- домен туннеля (`DOMAINS`)
- ключ шифрования туннеля (`ENCRYPTION_KEY`)

---

## Часть B - Настройка клиента

### B1. MasterDnsVPN client

1. Скачайте релиз:  
   [https://github.com/masterking32/MasterDnsVPN/releases](https://github.com/masterking32/MasterDnsVPN/releases)
2. Распакуйте архив.
3. Отредактируйте `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "тот_же_ключ_что_на_сервере"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. Запустите MasterDnsVPN и не закрывайте.

### B2. Конфиг мессенджера

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "IP_ИЛИ_ХОСТ_СЕРВЕРА:9999",
  "direct_mode": false
}
```

### B3. Вход и настройки

- Откройте настройки (доступны прямо внутри активной сессии)
- Выставьте язык, сервер, proxy
- Сохраните и войдите/зарегистрируйтесь

### B4. Android

- Поставьте APK или соберите из `android-client`
- Используйте те же параметры подключения, что и на desktop

---

## Часть C - Сборка из исходников

### C1. Требования

- Go `1.21+`
- Node.js `18+` и `npm`
- Android Studio / Android SDK (для Android)

### C2. Go сервер/клиент

```bash
go build -o server ./server
go build -o client ./client
```

Кросс-сборка:

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

Пакеты:

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

## Частые проблемы

- Connection refused:
  - проверьте `server_addr`
  - проверьте firewall
  - проверьте, что сервер запущен
- Туннель поднят, но чата нет:
  - `DOMAINS` и `ENCRYPTION_KEY` должны совпадать
  - SOCKS5 должен слушать `127.0.0.1:18000`
- Ошибка входа:
  - проверьте логин/пароль
  - проверьте логи сервера

---

## Безопасность

- Храните ключи туннеля в секрете.
- Используйте сложные уникальные пароли.
- Обновляйте сервер и зависимости.
- При подозрении на компрометацию меняйте ключи немедленно.
