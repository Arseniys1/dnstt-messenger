/**
 * MessengerClient — реализует бинарный протокол dnstt-messenger.
 * Поддерживает прямое TCP и SOCKS5 подключение (для dnstt).
 * Шифрование: X25519 ECDH хендшейк + ChaCha20-Poly1305.
 *
 * Protocol frame format: [TotalLen(2 LE)][Cmd(1)][Payload...]
 * TotalLen = 1 + len(Payload).
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
  REGISTER:      0x01,
  LOGIN:         0x02,
  MSG:           0x03,
  INCOMING:      0x04,
  HISTORY:       0x05,
  ACK:           0x06,
  HISTORY_END:   0x07,
  LOGIN_OK:      0x08,
  LOGIN_FAIL:    0x09,
  ONLINE_LIST:   0x0A,
  ONLINE_ADD:    0x0B,
  ONLINE_REMOVE: 0x0C,
  FRAGMENT:      0x0D,
};

const MAX_FRAME_SIZE = 180; // conservative limit for DNS tunnel MTU

class MessengerClient extends EventEmitter {
  constructor(cfg) {
    super();
    this.cfg = cfg;              // { proxy_addr, server_addr, direct_mode }
    this.socket = null;
    this.sharedKey = null;
    this.sessionID = 0;
    this._loginResolve = null;
    this._registerResolve = null;
    this._loginHandled = false;
    this._buf = Buffer.alloc(0);
    this._sendCounter = 0;       // monotonic nonce counter (client→server)
    this._fragCounter = 0;       // fragment message ID counter
    this._sidNames = new Map();  // SID (number) → username string
  }

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

    try {
      const { sharedKey, leftover } = await this._ecdhHandshake(socket);
      this.sharedKey = sharedKey;
      if (leftover.length > 0) this._buf = leftover;
    } catch (e) {
      socket.destroy();
      return { ok: false, error: 'ECDH failed: ' + e.message };
    }

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
          const privKey = x25519.utils.randomSecretKey();
          const pubKey  = x25519.getPublicKey(privKey);
          socket.write(Buffer.from(pubKey));
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

  // --- Build a length-prefixed frame: [TotalLen(2 LE)][cmd(1)][payload...] ---
  _buildFrame(cmd, payloadBuf) {
    const total = 1 + payloadBuf.length;
    const frame = Buffer.alloc(2 + total);
    frame.writeUInt16LE(total, 0);
    frame[2] = cmd;
    payloadBuf.copy(frame, 3);
    return frame;
  }

  // --- Регистрация ---
  register(login, pass) {
    return new Promise((resolve) => {
      this._registerResolve = resolve;
      const lBuf = Buffer.from(login);
      const pBuf = Buffer.from(pass);
      const payload = Buffer.alloc(1 + lBuf.length + pBuf.length);
      payload[0] = lBuf.length;
      lBuf.copy(payload, 1);
      pBuf.copy(payload, 1 + lBuf.length);
      this.socket.write(this._buildFrame(CMD.REGISTER, payload));
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
      const payload = Buffer.alloc(1 + lBuf.length + pBuf.length);
      payload[0] = lBuf.length;
      lBuf.copy(payload, 1);
      pBuf.copy(payload, 1 + lBuf.length);
      this.socket.write(this._buildFrame(CMD.LOGIN, payload));
      setTimeout(() => {
        if (this._loginResolve) {
          this._loginResolve = null;
          resolve({ ok: false, error: 'Timeout' });
        }
      }, 8000);
    });
  }

  // --- Отправка сообщения ---
  sendMessage(text) {
    if (!this.sharedKey) return false;
    // Counter-based nonce: 8-byte LE counter + 4 zero bytes = 12 bytes total
    this._sendCounter++;
    const nonce = Buffer.alloc(12); // all zeros initially
    nonce.writeUInt32LE(this._sendCounter & 0xFFFFFFFF, 0);
    nonce.writeUInt32LE(Math.floor(this._sendCounter / 0x100000000), 4);

    const key    = new Uint8Array(this.sharedKey);
    const msg    = new TextEncoder().encode(text);
    const cipher = chacha20poly1305(key, new Uint8Array(nonce));
    const ct     = Buffer.from(cipher.encrypt(msg));

    // msgPayload = [Counter(8 LE)][Ciphertext(N+16)]
    const msgPayload = Buffer.concat([nonce.slice(0, 8), ct]);

    // Frame = 2 (len prefix) + 1 (cmd) + msgPayload.length
    if (3 + msgPayload.length <= MAX_FRAME_SIZE) {
      this.socket.write(this._buildFrame(CMD.MSG, msgPayload));
      return true;
    }

    // Fragment: each fragment frame = 2 + 1 (cmd) + 3 (msgID, fragIdx, fragCount) + chunk
    this._fragCounter = (this._fragCounter + 1) & 0xFF;
    const msgID    = this._fragCounter;
    const maxChunk = MAX_FRAME_SIZE - 6;
    const chunks   = [];
    let   offset   = 0;
    while (offset < msgPayload.length) {
      chunks.push(msgPayload.slice(offset, offset + maxChunk));
      offset += maxChunk;
    }
    if (chunks.length > 255) return false; // fragCount must fit in one byte
    const fragCount = chunks.length;
    chunks.forEach((chunk, idx) => {
      const fp = Buffer.alloc(3 + chunk.length);
      fp[0] = msgID;
      fp[1] = idx;
      fp[2] = fragCount;
      chunk.copy(fp, 3);
      this.socket.write(this._buildFrame(CMD.FRAGMENT, fp));
    });
    return true;
  }

  // --- Расшифровка входящего ---
  _decrypt(ciphertext, nonce) {
    const key    = new Uint8Array(this.sharedKey);
    const nonceU = new Uint8Array(nonce);
    const ctU    = new Uint8Array(ciphertext);
    const cipher = chacha20poly1305(key, nonceU);
    return Buffer.from(cipher.decrypt(ctU));
  }

  // --- Входящие данные ---
  _onData(chunk) {
    this._buf = Buffer.concat([this._buf, chunk]);
    this._parse();
  }

  // --- Parse loop: process all complete frames in the buffer ---
  _parse() {
    while (this._buf.length >= 2) {
      const frameLen = this._buf.readUInt16LE(0);
      if (frameLen < 1 || this._buf.length < 2 + frameLen) return;
      const cmd     = this._buf[2];
      const payload = this._buf.slice(3, 2 + frameLen);
      this._buf = this._buf.slice(2 + frameLen);
      this._dispatchFrame(cmd, payload);
    }
  }

  _dispatchFrame(cmd, payload) {
    switch (cmd) {

      case CMD.LOGIN_OK:
        if (payload.length < 2) break;
        this.sessionID = (payload[0] << 8) | payload[1];
        if (this._loginResolve && !this._loginHandled) {
          this._loginHandled = true;
          const r = this._loginResolve;
          this._loginResolve = null;
          r({ ok: true });
        }
        break;

      case CMD.LOGIN_FAIL:
        if (this._loginResolve && !this._loginHandled) {
          this._loginHandled = true;
          const r = this._loginResolve;
          this._loginResolve = null;
          r({ ok: false, error: 'Invalid login or password' });
        }
        break;

      case 0x00: // register failure
        if (this._registerResolve) {
          const r = this._registerResolve;
          this._registerResolve = null;
          r({ ok: false });
        }
        break;

      // 0x01 = CMD.REGISTER is repurposed as register success response from server
      case CMD.REGISTER:
        if (this._registerResolve) {
          const r = this._registerResolve;
          this._registerResolve = null;
          r({ ok: true });
        }
        break;

      case CMD.ACK:
        break;

      case CMD.HISTORY_END:
        this.emit('history-end');
        break;

      case CMD.HISTORY: {
        // Payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][Content...]
        // Minimum: 1 (SenderLen) + 4 (Timestamp) = 5 bytes.
        if (payload.length < 5) break;
        const senderLen = payload[0];
        if (payload.length < 1 + senderLen + 4) break;
        const sender = payload.slice(1, 1 + senderLen).toString();
        const tsOff  = 1 + senderLen;
        const ts     = payload.readUInt32BE(tsOff);
        const text   = payload.slice(tsOff + 4).toString();
        const time   = new Date(ts * 1000).toLocaleString([], {
          year: 'numeric', month: '2-digit', day: '2-digit',
          hour: '2-digit', minute: '2-digit'
        });
        this.emit('history', { sender, text, time });
        break;
      }

      case CMD.INCOMING: {
        // Payload: [SenderSID(2 BE)][Counter(8 LE)][Ciphertext(N+16)]
        if (payload.length < 2 + 8 + 16) break;
        const senderSID = (payload[0] << 8) | payload[1];
        const nonce     = Buffer.alloc(12);
        payload.copy(nonce, 0, 2, 10); // bytes 2-9 = counter (8 bytes)
        // bytes 8-11 stay zero
        const ct        = payload.slice(10);
        try {
          const plain      = this._decrypt(ct, nonce);
          const senderName = this._sidNames.get(senderSID) || `SID:${senderSID}`;
          const now        = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          this.emit('message', senderName, plain.toString(), now);
        } catch (_) {}
        break;
      }

      case CMD.ONLINE_LIST: {
        // Payload: [Count(1)][SID(2 BE)][NameLen(1)][Name]...
        if (payload.length < 1) break;
        const count   = payload[0];
        let   off     = 1;
        const newMap  = new Map();
        const names   = [];
        let   valid   = true;
        for (let i = 0; i < count; i++) {
          if (off + 3 > payload.length) { valid = false; break; }
          const sid  = (payload[off] << 8) | payload[off + 1]; off += 2;
          const nLen = payload[off]; off++;
          if (off + nLen > payload.length) { valid = false; break; }
          const name = payload.slice(off, off + nLen).toString(); off += nLen;
          newMap.set(sid, name);
          names.push(name);
        }
        if (valid) {
          this._sidNames = newMap;
          this.emit('online-list', names);
        }
        break;
      }

      case CMD.ONLINE_ADD: {
        // Payload: [SID(2 BE)][NameLen(1)][Name]
        if (payload.length < 4) break;
        const sid  = (payload[0] << 8) | payload[1];
        const nLen = payload[2];
        if (3 + nLen > payload.length) break;
        const name = payload.slice(3, 3 + nLen).toString();
        this._sidNames.set(sid, name);
        this.emit('online-list', [...this._sidNames.values()]);
        break;
      }

      case CMD.ONLINE_REMOVE: {
        // Payload: [SID(2 BE)]
        if (payload.length < 2) break;
        const sid = (payload[0] << 8) | payload[1];
        this._sidNames.delete(sid);
        this.emit('online-list', [...this._sidNames.values()]);
        break;
      }

      // Unknown command — ignore (already consumed from buffer by _parse)
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
