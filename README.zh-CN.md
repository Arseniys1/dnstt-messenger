# DNSTT Messenger

一款支持端到端加密（E2E）并可通过 DNS 隧道通信的安全消息系统，适用于网络审查或强过滤环境。

## 文档语言

- English: [README.en.md](./README.en.md)
- Русский: [README.ru.md](./README.ru.md)
- 简体中文: [README.zh-CN.md](./README.zh-CN.md)
- فارسی: [README.fa.md](./README.fa.md)
- Türkçe: [README.tr.md](./README.tr.md)
- العربية: [README.ar.md](./README.ar.md)
- Tiếng Việt: [README.vi.md](./README.vi.md)

## 目录

- [项目目标](#项目目标)
- [核心功能](#核心功能)
- [A 部分：服务器部署](#a-部分服务器部署)
- [B 部分：客户端配置](#b-部分客户端配置)
- [C 部分：源码构建](#c-部分源码构建)
- [故障排查](#故障排查)
- [安全建议](#安全建议)
- [许可与责任](#许可与责任)

## 项目目标

DNSTT Messenger 面向常规通信渠道被 DPI 或 DNS 过滤影响的场景。
通过 DNS 传输可以在 HTTPS/VPN 受限时保持可用连接。

## 核心功能

- 客户端之间端到端加密通信（E2E）。
- 基于 MasterDnsVPN 的 DNS 隧道传输。
- 提供 Windows（Go CLI）、Electron（桌面）和 Android 客户端。
- 多语言界面支持。
- 可选多服务器联邦（federation）。

## A 部分：服务器部署

### A1. 前置条件

- 具有公网 IP 的 Linux VPS。
- 可管理 DNS 的域名。
- 基础 SSH/终端访问能力。
- 开放端口：
  - `53/udp` 和 `53/tcp`（DNS 隧道）。
  - `9999/tcp`（消息服务器）。
  - `9998/tcp`（仅 federation 模式）。

### A2. DNS 记录配置

示例：

- 主域名：`example.com`
- 隧道域：`t.example.com`
- NS 主机：`ns1.example.com`

创建记录：

1. `A`: `ns1.example.com -> <VPS_IP>`
2. `NS`: `t.example.com -> ns1.example.com`

检查：

- `dig NS example.com`
- `dig t.example.com`

### A3. 安装 MasterDnsVPN Server

```bash
wget https://github.com/masterking32/MasterDnsVPN/releases/latest/download/MasterDnsVPN_Server_Linux_AMD64.zip
unzip MasterDnsVPN_Server_Linux_AMD64.zip
cd MasterDnsVPN_Server_Linux_AMD64
```

编辑 `server_config.toml`：

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "replace_with_strong_random_key"
```

运行：

```bash
./MasterDnsVPN_Server
```

### A4. 启动消息服务器

在 `server` 二进制旁创建 `config.json`：

```json
{
  "listen_addr": "0.0.0.0:9999",
  "db_path": "./messenger.db",
  "history_limit": 50
}
```

运行：

```bash
./server
```

### A5. Federation（可选）

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

`s2s_secret` 必须在所有联邦节点一致。

### A6. 需要提供给客户端的信息

- `server_addr`（例如 `1.2.3.4:9999`）
- `DOMAINS`（隧道域名）
- `ENCRYPTION_KEY`（隧道密钥）

## B 部分：客户端配置

### B1. 配置 MasterDnsVPN Client

1. 从 [MasterDnsVPN Releases](https://github.com/masterking32/MasterDnsVPN/releases) 下载。
2. 解压文件。
3. 编辑 `client_config.toml`：

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. 启动并保持 MasterDnsVPN 运行。

### B2. 配置 Messenger

编辑 `client_config.json`：

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. 启动客户端

- Go CLI：Windows 用 `client.exe`，Linux/macOS 用 `./client`
- Electron：在 `electron-client` 启动
- Android：安装 `android-client` 构建出的 APK 或发布包

### B4. 登录与注册

1. 启动客户端。
2. 输入用户名和密码。
3. 首次使用时创建账号。
4. 登录后确认语言/代理/服务器设置。

### B5. Android 说明

- Android 端使用与桌面端相同的 `server_addr`、`DOMAINS`、`ENCRYPTION_KEY`。
- 确保代理配置与实际启动方式一致。

## C 部分：源码构建

### C1. 依赖

- `Go 1.21+`
- `Node.js 18+` 与 `npm`
- `Android Studio` + `Android SDK`

### C2. 构建 Go 服务端与客户端

```bash
go build -o server ./server
go build -o client ./client
```

跨平台示例：

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

打包：

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

Windows：

```cmd
gradlew.bat assembleRelease
```

## 故障排查

### 客户端无法连接

- 检查 `server_addr`。
- 检查防火墙和端口是否放行。
- 确认服务进程已运行。

### 隧道已连通但聊天不可用

- `DOMAINS` 与 `ENCRYPTION_KEY` 必须和服务器一致。
- SOCKS5 地址应为 `127.0.0.1:18000`。

### 登录失败

- 检查用户名与密码。
- 查看服务器认证日志。

### 本地化乱码（Mojibake）

- 确认翻译文件为 UTF-8 编码。
- 检查 fallback 语言与翻译键完整性。

## 安全建议

- 使用高强度随机密钥并定期轮换。
- 不要公开生产服务器地址。
- 测试环境与生产环境分离。
- 限制 SSH 访问（密钥、防火墙、fail2ban）。
- 日志中避免记录敏感内容。

## 许可与责任

上线前请检查本仓库中的最新许可证。
你需要自行确保部署与使用符合所在地法律及组织政策。
