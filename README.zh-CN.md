# DNSTT Messenger - 完整指南（简体中文）

DNSTT Messenger 是一个支持端到端加密（E2E）的消息系统，可通过 MasterDnsVPN 使用 DNS 隧道在受限网络中通信。

## 目录

- A 部分：服务器部署（MasterDnsVPN + 消息服务器）
- B 部分：客户端配置（Windows/Electron/Android）
- C 部分：源码构建
- 故障排查与安全建议

---

## A 部分：服务器部署

### A1. 前置条件

- 一台 Linux VPS（公网 IP）
- 你可管理的域名
- 基本终端操作能力
- 防火墙端口：
  - `53/udp`、`53/tcp`（DNS）
  - `9999/tcp`（消息服务）
  - `9998/tcp`（仅 federation 使用）

### A2. 隧道域名 DNS 配置

示例：

- 主域名：`example.com`
- 隧道域：`t.example.com`
- NS 主机：`ns1.example.com`

配置记录：

1. `A`：`ns1.example.com -> <VPS_IP>`
2. `NS`：`t.example.com -> ns1.example.com`

传播后，`*.t.example.com` 应解析到你的 VPS DNS 服务。

### A3. 安装并运行 MasterDnsVPN Server

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

启动：

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

多节点配置示例：

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

### A6. 提供给客户端的信息

- `server_addr`
- 隧道域名（`DOMAINS`）
- 隧道加密密钥（`ENCRYPTION_KEY`）

---

## B 部分：客户端配置

### B1. 配置 MasterDnsVPN 客户端

1. 下载发布包：  
   [https://github.com/masterking32/MasterDnsVPN/releases](https://github.com/masterking32/MasterDnsVPN/releases)
2. 解压。
3. 编辑 `client_config.toml`：

```toml
DOMAINS = ["t.example.com"]
ENCRYPTION_KEY = "same_key_as_server"
LISTEN_IP = "127.0.0.1"
LISTEN_PORT = 18000
PROTOCOL_TYPE = "SOCKS5"
```

4. 启动并保持 MasterDnsVPN 运行。

### B2. 配置 Messenger 客户端

```json
{
  "proxy_addr": "127.0.0.1:18000",
  "server_addr": "SERVER_IP_OR_HOST:9999",
  "direct_mode": false
}
```

### B3. 登录与应用内设置

- 在应用内打开 Settings（无需退出）
- 修改语言、服务器、代理后保存
- 注册或登录账号

### B4. Android

- 可直接安装 APK，或在 `android-client` 中构建
- 使用与桌面端一致的连接参数

---

## C 部分：源码构建

### C1. 依赖

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

---

## 故障排查

- 无法连接：
  - 检查 `server_addr`
  - 检查防火墙端口
  - 检查服务是否运行
- 隧道正常但无消息：
  - 检查 `DOMAINS` 与 `ENCRYPTION_KEY` 一致
  - 检查 SOCKS5 地址 `127.0.0.1:18000`
- 登录失败：
  - 校验账号密码
  - 查看服务器日志

---

## 安全建议

- 妥善保管隧道密钥。
- 使用强密码。
- 定期更新服务端依赖。
- 发现泄露风险时立即轮换密钥。
