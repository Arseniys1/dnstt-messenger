/**
 * MessengerClient — реализует бинарный протокол dnstt-messenger.
 * Поддерживает прямое TCP и SOCKS5 подключение (для dnstt).
 * Шифрование: X25519 ECDH хендшейк + ChaCha20-Poly1305.
 */

const net = require('net');
const crypto = require('crypto');
const { EventEmitter } = require('events');
const { SocksClient } = require('socks');

// @noble/ciphers и @noble/curves — ESM, грузим через dynamic import
let chacha20poly1305, x25519;
async function loadCrypto() {
  if (chacha20poly1305 && x25519) return;
  const ciphers = await import('@noble/ciphers/chacha.js');
  const curves  = await import('@noble/curves/ed25519.js');
  chacha20poly1305 = ciphers.chacha20poly1305;
  x25519           = curves.x25519;
}

const CMD = {
  REGISTER:    0x01,
  LOGIN:       0x02,
  MSG:         0x03,
  INCOMING:    0x04,
  HISTORY:     0x05,
  ACK:         0x06,
  HISTORY_END: 0x07,
  LOGIN_OK:    0x08,
  LOGIN_FAIL:  0x09,
  ONLINE_LIST: 0x0A,
};

class MessengerClient extends EventEmitter {
  constructor(cfg) {
    super();
    this.cfg = cfg;         // { proxy_addr, server_addr, direct_mode }
    this.socket = null;
    this.sharedKey = null;
    this.sessionID = 0;
    this._loginResolve = null;
    this._registerResolve = null;
    this._loginHandled = false;
    this._buf = Buffer.alloc(0);
  }

  // Парсим "host:port" строку
  _parseAddr(addr) {
    const idx = addr.lastIndexOf(':');
    return { host: addr.slice(0, idx), port: parseInt(addr.slice(idx + 1), 10) };
  }

  async connect() {
    await loadCrypto();
    const srv = this._parseAddr(this.cfg.server_addr);
    let socket;

    try {
      if (this.cfg.direct_mode) {
        socket = await this._tcpConnect(srv.host, srv.port);
      } else {
        const proxy = this._parseAddr(this.cfg.proxy_addr);
        socket = await this._socks5Connect(proxy.host, proxy.port, srv.host, srv.port);
      }
    } catch (e) {
      return { ok: false, error: e.message };
    }

    this.socket = socket;

    // Хендшейк ДОЛЖЕН завершиться до того, как мы начнём слушать данные приложения
    try {
      const { sharedKey, leftover } = await this._ecdhHandshake(socket);
      this.sharedKey = sharedKey;
      // Если сервер прислал данные вместе с pubkey — кладём в буфер
      if (leftover.length > 0) this._buf = leftover;
    } catch (e) {
      socket.destroy();
      return { ok: false, error: 'ECDH failed: ' + e.message };
    }

    // Только после хендшейка вешаем основной обработчик данных
    socket.on('data', (d) => this._onData(d));
    socket.on('close', () => { this.emit('disconnected'); });
    socket.on('error', () => { this.emit('disconnected'); });

    return { ok: true };
  }

  _tcpConnect(host, port) {
    return new Promise((resolve, reject) => {
      const s = net.createConnection({ host, port, timeout: 10000 });
      s.once('connect', () => resolve(s));
      s.once('error', reject);
      s.once('timeout', () => reject(new Error('Connection timeout')));
    });
  }

  async _socks5Connect(proxyHost, proxyPort, dstHost, dstPort) {
    const info = await SocksClient.createConnection({
      proxy: { host: proxyHost, port: proxyPort, type: 5 },
      command: 'connect',
      destination: { host: dstHost, port: dstPort },
      timeout: 10000
    });
    return info.socket;
  }

  // X25519 ECDH: сервер шлёт 32 байта pubkey, клиент отвечает своим.
  // Возвращает { sharedKey, leftover }
  _ecdhHandshake(socket) {
    return new Promise((resolve, reject) => {
      let buf = Buffer.alloc(0);

      const onData = (chunk) => {
        buf = Buffer.concat([buf, chunk]);
        if (buf.length < 32) return;

        socket.removeListener('data', onData);
        socket.removeListener('error', onErr);

        const serverPubBytes = new Uint8Array(buf.slice(0, 32));
        const leftover = buf.slice(32);

        try {
          // Генерируем X25519 ключевую пару через @noble/curves
          const privKey = x25519.utils.randomSecretKey();
          const pubKey  = x25519.getPublicKey(privKey);

          // Отправляем наш публичный ключ серверу
          socket.write(Buffer.from(pubKey));

          // Вычисляем общий секрет
          const shared = x25519.getSharedSecret(privKey, serverPubBytes);
          resolve({ sharedKey: Buffer.from(shared), leftover });
        } catch (e) {
          reject(e);
        }
      };

      const onErr = (e) => reject(e);
      socket.on('data', onData);
      socket.once('error', onErr);
    });
  }

  // --- Регистрация ---
  register(login, pass) {
    return new Promise((resolve) => {
      this._registerResolve = resolve;
      const lBuf = Buffer.from(login);
      const pBuf = Buffer.from(pass);
      const pkt = Buffer.alloc(2 + lBuf.length + pBuf.length);
      pkt[0] = CMD.REGISTER;
      pkt[1] = lBuf.length;
      lBuf.copy(pkt, 2);
      pBuf.copy(pkt, 2 + lBuf.length);
      this.socket.write(pkt);
      // таймаут 5 сек
      setTimeout(() => {
        if (this._registerResolve) {
          this._registerResolve = null;
          resolve({ ok: false, error: 'Timeout' });
        }
      }, 5000);
    });
  }

  // --- Вход ---
  login(login, pass) {
    return new Promise((resolve) => {
      this._loginHandled = false;
      this._loginResolve = resolve;
      const lBuf = Buffer.from(login);
      const pBuf = Buffer.from(pass);
      const pkt = Buffer.alloc(2 + lBuf.length + pBuf.length);
      pkt[0] = CMD.LOGIN;
      pkt[1] = lBuf.length;
      lBuf.copy(pkt, 2);
      pBuf.copy(pkt, 2 + lBuf.length);
      this.socket.write(pkt);
      setTimeout(() => {
        if (this._loginResolve) {
          this._loginResolve = null;
          resolve({ ok: false, error: 'Timeout' });
        }
      }, 8000);
    });
  }

  // --- Отправка сообщения (ChaCha20-Poly1305 via @noble/ciphers) ---
  sendMessage(text) {
    if (!this.sharedKey || !this.sessionID) return false;
    const nonce = crypto.randomBytes(12);
    const key   = new Uint8Array(this.sharedKey);
    const msg   = new TextEncoder().encode(text);

    // chacha20poly1305 из @noble возвращает [ciphertext + 16-byte tag]
    const cipher = chacha20poly1305(key, nonce);
    const ct = cipher.encrypt(msg);

    const sid = this.sessionID;
    const pkt = Buffer.alloc(3 + 12 + ct.length);
    pkt[0] = CMD.MSG;
    pkt[1] = (sid >> 8) & 0xff;
    pkt[2] = sid & 0xff;
    nonce.copy(pkt, 3);
    Buffer.from(ct).copy(pkt, 15);
    this.socket.write(pkt);
    return true;
  }

  // --- Расшифровка входящего (ChaCha20-Poly1305 via @noble/ciphers) ---
  _decrypt(ciphertext, nonce) {
    const key    = new Uint8Array(this.sharedKey);
    const nonceU = new Uint8Array(nonce);
    const ctU    = new Uint8Array(ciphertext);
    const cipher = chacha20poly1305(key, nonceU);
    return Buffer.from(cipher.decrypt(ctU));
  }

  // --- Парсинг входящих данных ---
  _onData(chunk) {
    this._buf = Buffer.concat([this._buf, chunk]);
    this._parse();
  }

  _parse() {
    while (this._buf.length > 0) {
      const cmd = this._buf[0];

      if (cmd === CMD.LOGIN_OK) {
        if (this._buf.length < 3) return;
        this.sessionID = (this._buf[1] << 8) | this._buf[2];
        this._buf = this._buf.slice(3);
        if (this._loginResolve && !this._loginHandled) {
          this._loginHandled = true;
          const r = this._loginResolve;
          this._loginResolve = null;
          r({ ok: true });
        }
        continue;
      }

      if (cmd === CMD.LOGIN_FAIL) {
        this._buf = this._buf.slice(1);
        if (this._loginResolve && !this._loginHandled) {
          this._loginHandled = true;
          const r = this._loginResolve;
          this._loginResolve = null;
          r({ ok: false, error: 'Invalid login or password' });
        }
        continue;
      }

      if (cmd === 0x00 || cmd === 0x01) {
        // ответ на регистрацию
        this._buf = this._buf.slice(1);
        if (this._registerResolve) {
          const r = this._registerResolve;
          this._registerResolve = null;
          r({ ok: cmd === 0x01 });
        }
        continue;
      }

      if (cmd === CMD.ACK) {
        this._buf = this._buf.slice(1);
        continue;
      }

      if (cmd === CMD.HISTORY_END) {
        this._buf = this._buf.slice(1);
        this.emit('history-end');
        continue;
      }

      if (cmd === CMD.HISTORY) {
        if (this._buf.length < 5) return;
        const senderLen = this._buf[1];
        let off = 2 + senderLen;
        if (this._buf.length < off + 1) return;
        const sender = this._buf.slice(2, 2 + senderLen).toString();
        const timeLen = this._buf[off]; off++;
        if (this._buf.length < off + timeLen + 2) return;
        const time = this._buf.slice(off, off + timeLen).toString(); off += timeLen;
        const msgLen = (this._buf[off] << 8) | this._buf[off + 1]; off += 2;
        if (this._buf.length < off + msgLen) return;
        const text = this._buf.slice(off, off + msgLen).toString();
        this._buf = this._buf.slice(off + msgLen);
        this.emit('history', { sender, text, time });
        continue;
      }

      if (cmd === CMD.INCOMING) {
        // Формат: [0x04][senderLen(1)][sender][nonce(12)][ctLen(2)][ciphertext]
        if (this._buf.length < 4) return;
        const senderLen = this._buf[1];
        const minLen = 2 + senderLen + 12 + 2 + 1;
        if (this._buf.length < minLen) return;
        const sender = this._buf.slice(2, 2 + senderLen).toString();
        const nonce  = this._buf.slice(2 + senderLen, 2 + senderLen + 12);
        const ctLen  = (this._buf[2 + senderLen + 12] << 8) | this._buf[2 + senderLen + 13];
        const off    = 2 + senderLen + 12 + 2;
        if (this._buf.length < off + ctLen) return;
        const ct = this._buf.slice(off, off + ctLen);
        try {
          const plain = this._decrypt(ct, nonce);
          const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          this.emit('message', sender, plain.toString(), now);
        } catch (_) {}
        this._buf = this._buf.slice(off + ctLen);
        continue;
      }

      if (cmd === CMD.ONLINE_LIST) {
        if (this._buf.length < 2) return;
        const count = this._buf[1];
        let off = 2;
        const names = [];
        let valid = true;
        for (let i = 0; i < count; i++) {
          if (off >= this._buf.length) { valid = false; break; }
          const nLen = this._buf[off]; off++;
          if (off + nLen > this._buf.length) { valid = false; break; }
          names.push(this._buf.slice(off, off + nLen).toString());
          off += nLen;
        }
        if (!valid) return;
        this._buf = this._buf.slice(off);
        this.emit('online-list', names);
        continue;
      }

      // Неизвестный байт — пропускаем
      this._buf = this._buf.slice(1);
    }
  }

  destroy() {
    if (this.socket) {
      this.socket.destroy();
      this.socket = null;
    }
  }
}

module.exports = MessengerClient;
