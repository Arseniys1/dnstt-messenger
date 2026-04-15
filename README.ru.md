# DNSTT Messenger

Безопасный мессенджер со сквозным шифрованием (E2E) и транспортом через DNS-туннель для сетей с жесткой фильтрацией и блокировками.

## Языки документации

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## Содержание

- [Назначение проекта](#назначение-проекта)
- [Основные возможности](#основные-возможности)
- [Часть A: Настройка сервера](#часть-a-настройка-сервера)
- [Часть B: Настройка клиента](#часть-b-настройка-клиента)
- [Часть C: Сборка из исходников](#часть-c-сборка-из-исходников)
- [Диагностика и частые проблемы](#диагностика-и-частые-проблемы)
- [Рекомендации по безопасности](#рекомендации-по-безопасности)
- [Лицензия и ответственность](#лицензия-и-ответственность)

## Назначение проекта

DNSTT Messenger предназначен для ситуаций, где обычные каналы связи нестабильны или блокируются DPI/фильтрацией DNS.
Транспорт через DNS позволяет сохранить связность там, где HTTPS/VPN может быть ограничен.

## Основные возможности

- Сквозное шифрование сообщений между клиентами (E2E).
- Передача данных через DNS-туннель (MasterDnsVPN).
- Клиенты для Windows (Go CLI), Electron (desktop) и Android.
- Поддержка многоязычного интерфейса.
- Опциональная федерация между несколькими серверами.

## Часть A: Настройка сервера

### A1. Что требуется

- Linux VPS с публичным IP.
- Домен с доступом к управлению DNS-зоной.
- Базовый доступ по SSH.
- Открытые порты:
  - `53/udp` и `53/tcp` для DNS-туннеля.
  - `9999/tcp` для сервера мессенджера.
  - `9998/tcp` только если используется федерация.

### A2. DNS-записи для туннеля

Пример:

- основной домен: `example.com`
- зона туннеля: `t.example.com`
- NS-хост: `ns1.example.com`

Создайте записи у регистратора:

1. `A`: `ns1.example.com -> <IP_VPS>`
2. `NS`: `t.example.com -> ns1.example.com`

Проверка:

- `dig NS example.com`
- `dig t.example.com`

### A3. Установка MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

Настройте `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
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

### A5. Федерация (опционально)

Для сети из нескольких серверов:

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

`s2s_secret` должен совпадать на всех узлах.

### A6. Что передать пользователям

- `server_addr` (адрес сервера мессенджера, например `1.2.3.4:9999`)
- `DOMAINS` (домен DNS-туннеля)
- `ENCRYPTION_KEY` (ключ туннеля)

## Часть B: Настройка клиента

### B1. Настройка MasterDnsVPN Client

1. Скачайте клиент с [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases).
2. Распакуйте архив.
3. Отредактируйте `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. Запустите MasterDnsVPN и держите его включенным.

### B2. Конфиг мессенджера

Настройте `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. Запуск клиентов

- Go-клиент (консоль): `client.exe` (Windows) или `./client` (Linux/macOS)
- Electron-клиент: запуск из папки `electron-client`
- Android-клиент: APK из `android-client` или готовый релиз

### B4. Вход и регистрация

1. Запустите клиент.
2. Введите логин и пароль.
3. Для первого входа создайте аккаунт.
4. После входа проверьте язык/прокси/адрес сервера в настройках.

### B5. Android заметка

- На Android используйте те же `server_addr`, `DOMAINS`, `ENCRYPTION_KEY`.
- Убедитесь, что локальный прокси настроен как в вашей схеме запуска.

## Часть C: Сборка из исходников

### C1. Требования

- `Go 1.21+`
- `Node.js 18+` и `npm`
- `Android Studio` + `Android SDK` (для Android-клиента)

### C2. Сборка Go-сервера и Go-клиента

```bash
go build -o server ./server
go build -o client ./client
```

Примеры кросс-сборки:

```bash
GOOS=linux GOARCH=amd64 go build -o server-linux-amd64 ./server
GOOS=windows GOARCH=amd64 go build -o server.exe ./server
GOOS=darwin GOARCH=arm64 go build -o server-mac-arm64 ./server
```

PowerShell (Windows):

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

Сборка пакетов:

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

## Диагностика и частые проблемы

### Клиент не подключается

- Проверьте `server_addr`.
- Проверьте firewall и доступность портов.
- Проверьте, что серверы действительно запущены.

### Туннель поднят, но чат не работает

- `DOMAINS` и `ENCRYPTION_KEY` должны совпадать с серверными.
- SOCKS5 endpoint должен быть `127.0.0.1:18000`.

### Ошибка входа

- Проверьте логин и пароль.
- Проверьте логи сервера на ошибки аутентификации.

### Проблемы с локализацией (mojibake)

- Убедитесь, что файлы переводов сохранены в UTF-8.
- Проверьте fallback-локаль и наличие ключей.

## Рекомендации по безопасности

- Используйте длинные случайные ключи и периодически ротируйте их.
- Не публикуйте адреса production-серверов в открытых каналах.
- Разделяйте тестовый и боевой контуры.
- Ограничьте SSH-доступ (ключи, firewall, fail2ban).
- Минимизируйте чувствительные данные в логах.

## Лицензия и ответственность

Перед production-развёртыванием проверьте актуальную лицензию проекта в репозитории.
Вы несете ответственность за соблюдение законов вашей юрисдикции и внутренних политик вашей организации.
