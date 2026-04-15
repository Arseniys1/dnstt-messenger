# DNSTT Messenger

Trình nhắn tin bảo mật với mã hóa đầu-cuối (E2E) và truyền tải qua đường hầm DNS, phù hợp cho mạng bị kiểm duyệt hoặc lọc mạnh.

## Ngôn ngữ tài liệu

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## Mục lục

- [Mục tiêu dự án](#mục-tiêu-dự-án)
- [Tính năng chính](#tính-năng-chính)
- [Phần A: Thiết lập máy chủ](#phần-a-thiết-lập-máy-chủ)
- [Phần B: Thiết lập client](#phần-b-thiết-lập-client)
- [Phần C: Build từ mã nguồn](#phần-c-build-từ-mã-nguồn)
- [Khắc phục sự cố](#khắc-phục-sự-cố)
- [Khuyến nghị bảo mật](#khuyến-nghị-bảo-mật)
- [Giấy phép và trách nhiệm](#giấy-phép-và-trách-nhiệm)

## Mục tiêu dự án

DNSTT Messenger dành cho các tình huống mà kênh liên lạc thông thường bị DPI hoặc lọc DNS chặn/làm gián đoạn.
Truyền tải qua DNS giúp duy trì kết nối khi HTTPS/VPN bị hạn chế.

## Tính năng chính

- Nhắn tin mã hóa đầu-cuối giữa các client (E2E).
- Truyền tin qua đường hầm DNS bằng MasterDnsVPN.
- Client cho Windows (Go CLI), Electron (desktop) và Android.
- Hỗ trợ giao diện đa ngôn ngữ.
- Federation tùy chọn giữa nhiều máy chủ.

## Phần A: Thiết lập máy chủ

### A1. Yêu cầu

- Linux VPS có IP public.
- Domain có quyền quản lý DNS.
- Truy cập SSH/terminal cơ bản.
- Mở cổng:
  - `53/udp` và `53/tcp` cho DNS tunnel.
  - `9999/tcp` cho máy chủ messenger.
  - `9998/tcp` chỉ khi bật federation.

### A2. Bản ghi DNS cho tunnel

Ví dụ:

- domain gốc: `example.com`
- vùng tunnel: `t.example.com`
- host NS: `ns1.example.com`

Tạo bản ghi:

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

Kiểm tra:

- `dig NS example.com`
- `dig t.example.com`

### A3. Cài đặt MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

Sửa `server_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

Chạy:

```bash
./MasterDnsVPN_Server
```

### A4. Chạy máy chủ messenger

Tạo `config.json` cạnh binary `server`:

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

Chạy:

```bash
./server
```

### A5. Federation (tùy chọn)

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

`s2s_secret` phải giống nhau trên tất cả node federation.

### A6. Thông tin cần gửi cho người dùng

- `server_addr` (ví dụ `1.2.3.4:9999`)
- `DOMAINS` (domain tunnel)
- `ENCRYPTION_KEY` (khóa tunnel)

## Phần B: Thiết lập client

### B1. Cấu hình MasterDnsVPN Client

1. Tải từ [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases).
2. Giải nén.
3. Sửa `client_config.toml`:

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. Khởi chạy MasterDnsVPN và giữ nó hoạt động.

### B2. Cấu hình Messenger

Sửa `client_config.json`:

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. Chạy client

- Go CLI: `client.exe` (Windows) hoặc `./client` (Linux/macOS)
- Electron: chạy trong thư mục `electron-client`
- Android: cài APK từ `android-client` hoặc bản release

### B4. Đăng nhập và đăng ký

1. Mở client.
2. Nhập tên người dùng và mật khẩu.
3. Tạo tài khoản ở lần đầu sử dụng.
4. Sau khi đăng nhập, kiểm tra ngôn ngữ/proxy/server trong settings.

### B5. Ghi chú Android

- Dùng cùng giá trị `server_addr`, `DOMAINS`, `ENCRYPTION_KEY` như desktop.
- Đảm bảo cấu hình proxy đúng với cách bạn triển khai trên Android.

## Phần C: Build từ mã nguồn

### C1. Yêu cầu công cụ

- `Go 1.21+`
- `Node.js 18+` và `npm`
- `Android Studio` + `Android SDK`

### C2. Build Go server và Go client

```bash
go build -o server ./server
go build -o client ./client
```

Ví dụ cross-build:

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

Build package:

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

## Khắc phục sự cố

### Client không kết nối được

- Kiểm tra `server_addr`.
- Kiểm tra firewall và cổng đã mở.
- Đảm bảo các tiến trình máy chủ đang chạy.

### Tunnel hoạt động nhưng chat không chạy

- `DOMAINS` và `ENCRYPTION_KEY` phải khớp với server.
- SOCKS5 endpoint phải là `127.0.0.1:18000`.

### Lỗi đăng nhập

- Kiểm tra username/password.
- Xem log server để tìm lỗi xác thực.

### Lỗi hiển thị ngôn ngữ (Mojibake)

- Đảm bảo file dịch dùng mã hóa UTF-8.
- Kiểm tra fallback locale và đủ translation keys.

## Khuyến nghị bảo mật

- Dùng khóa ngẫu nhiên mạnh và xoay khóa định kỳ.
- Không công khai địa chỉ máy chủ production.
- Tách biệt môi trường test và production.
- Hạn chế SSH (key, firewall, fail2ban).
- Giảm tối đa dữ liệu nhạy cảm trong log.

## Giấy phép và trách nhiệm

Trước khi triển khai production, hãy kiểm tra giấy phép hiện tại trong repository này.
Bạn chịu trách nhiệm tuân thủ pháp luật địa phương và chính sách của tổ chức mình.
