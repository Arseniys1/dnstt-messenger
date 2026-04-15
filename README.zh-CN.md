# DNSTT Messenger（简体中文）

DNSTT Messenger 是一个端到端加密（E2E）消息应用，可通过 DNS 隧道（MasterDnsVPN）在受限网络环境中通信。

## 快速开始（客户端）

1. 下载并运行 MasterDnsVPN 客户端。
2. 配置 `client_config.toml`（域名与加密密钥由服务器管理员提供）。
3. 启动并保持 MasterDnsVPN 运行。
4. 启动 DNSTT Messenger 客户端。
5. 在应用内打开 **Settings**，设置服务器/代理/语言并保存。
6. 注册或登录账号。

## 自建服务器

1. 为隧道域名配置 DNS 记录。
2. 启动 MasterDnsVPN 服务端。
3. 使用 `config.json` 启动消息服务器（`server`）。
4. 向用户分发连接参数：
   - `server_addr`
   - 隧道域名
   - 隧道加密密钥

## 构建

- Go 客户端/服务端：`go build`
- Electron 客户端：`npm install && npm start`（或 `npm run build:*`）
- Android 客户端：`./gradlew assembleRelease`（或 Android Studio）

## 安全建议

- 妥善保管 MasterDnsVPN 加密密钥。
- 使用高强度账号密码。
- 使用可信服务器基础设施。
- 如怀疑泄露，请立即轮换密钥。
