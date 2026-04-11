/**
 * MessengerClient — реализует бинарный протокол dnstt-messenger.
 * Поддерживает прямое TCP и SOCKS5 подключение (для dnstt).
 * Шифрование: X25519 ECDH хендшейк (сессия) + E2E X25519/ChaCha20-Poly1305.
 *
 * Protocol frame format: [TotalLen(2 LE)][Cmd(1)][Payload...]
 * TotalLen = 1 + len(Payload).
 */

const net    = require('net');
const crypto = require('crypto');
const fs     = require('fs');
const path   = require('path');
const { EventEmitter } = require('events');
const { SocksClient }  = require('socks');

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
  REGISTER:        0x01,
  LOGIN:           0x02,
  // 0x03-0x05 retired (plaintext msg/incoming/history removed)
  ACK:             0x06,
  HISTORY_END:     0x07,
  LOGIN_OK:        0x08,
  LOGIN_FAIL:      0x09,
  ONLINE_LIST:     0x0A,
  ONLINE_ADD:      0x0B,
  ONLINE_REMOVE:   0x0C,
  FRAGMENT:        0x0D,
  SERVER_LIST:     0x0E,
  // E2E commands
  SET_PUBLIC_KEY:     0x0F,
  PUBLIC_KEY:         0x10,
  PUBLIC_KEY_REQUEST: 0x11,
  E2E_MSG:            0x12,
  E2E_INCOMING:       0x13,
  E2E_HISTORY:        0x14,
};

const MAX_FRAME_SIZE = 180;

// ---- HKDF helper (works on any Node.js >= 12) ----
function hkdf(ikm, salt, info, length) {
  // Extract
  const prk = crypto.createHmac('sha256', salt).update(ikm).digest();
  // Expand
  const infoBytes = Buffer.isBuffer(info) ? info : Buffer.from(info);
  let output = Buffer.alloc(0);
  let T = Buffer.alloc(0);
  let counter = 1;
  while (output.length < length) {
    T = crypto.createHmac('sha256', prk)
        .update(T).update(infoBytes).update(Buffer.from([counter++]))
        .digest();
    output = Buffer.concat([output, T]);
  }
  return output.slice(0, length);
}

class MessengerClient extends EventEmitter {
  constructor(cfg) {
    super();
    this.cfg = cfg;
    this.socket = null;
    this.sharedKey = null;    // session ECDH key
    this.sessionID = 0;
    this._loginResolve = null;
    this._registerResolve = null;
    this._loginHandled = false;
    this._buf = Buffer.alloc(0);
    this._fragCounter = 0;
    this._sidNames = new Map();   // SID → username
    this._knownServers = [];

    // E2E state
    this._e2ePrivKey = null;     // Uint8Array (32 bytes)
    this._e2ePubKey  = null;     // Uint8Array (32 bytes)
    this._knownPubkeys = new Map(); // username → Buffer(32)
    this._pendingMessages = [];   // messages waiting for pubkeys
    this._myLogin = '';
  }

  // ---- Key file path ----
  _keyPath() {
    // Store next to the config / in userData if available
    try {
      const { app } = require('electron');
      return path.join(app.getPath('userData'), 'e2e_key.json');
    } catch (_) {
      return path.join(__dirname, '..', 'e2e_key.json');
    }
  }

  _loadOrGenerateE2EKey() {
    const kp = this._keyPath();
    try {
      const data = JSON.parse(fs.readFileSync(kp, 'utf8'));
      if (data.priv_key && data.pub_key) {
        this._e2ePrivKey = Buffer.from(data.priv_key, 'base64');
        this._e2ePubKey  = Buffer.from(data.pub_key,  'base64');
        console.log('[E2E] Ключ загружен из', kp);
        return;
      }
    } catch (_) {}

    // Generate new key
    this._e2ePrivKey = x25519.utils.randomSecretKey();
    this._e2ePubKey  = x25519.getPublicKey(this._e2ePrivKey);
    try {
      const dir = path.dirname(kp);
      if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
      fs.writeFileSync(kp, JSON.stringify({
        priv_key: Buffer.from(this._e2ePrivKey).toString('base64'),
        pub_key:  Buffer.from(this._e2ePubKey).toString('base64'),
      }), { mode: 0o600 });
      console.log('[E2E] Новый ключ сохранён в', kp);
    } catch (e) {
      console.warn('[E2E] Не удалось сохранить ключ:', e.message);
    }
  }

  // ---- sealEnvelope: 80-byte envelope = ephPub(32) + ChaCha20Poly1305(msgKey)(48) ----
  _sealEnvelope(recipientPub, msgKey) {
    const ephPriv = x25519.utils.randomSecretKey();
    const ephPub  = x25519.getPublicKey(ephPriv);
    const shared  = x25519.getSharedSecret(ephPriv, new Uint8Array(recipientPub));

    // HKDF(shared, salt=ephPub, info="dnstt-e2e-v1") → 44 bytes
    const km = hkdf(Buffer.from(shared), Buffer.from(ephPub), 'dnstt-e2e-v1', 44);

    const key   = new Uint8Array(km.slice(0, 32));
    const nonce = new Uint8Array(km.slice(32, 44));
    const cipher = chacha20poly1305(key, nonce);
    const ct = Buffer.from(cipher.encrypt(new Uint8Array(msgKey))); // 48 bytes

    return Buffer.concat([Buffer.from(ephPub), ct]); // 80 bytes
  }

  // ---- openEnvelope: decrypt 80-byte envelope → 32-byte msgKey ----
  _openEnvelope(envelope) {
    const ephPub = envelope.slice(0, 32);
    const ct     = envelope.slice(32); // 48 bytes
    const shared = x25519.getSharedSecret(this._e2ePrivKey, new Uint8Array(ephPub));
    const km = hkdf(Buffer.from(shared), Buffer.from(ephPub), 'dnstt-e2e-v1', 44);

    const key   = new Uint8Array(km.slice(0, 32));
    const nonce = new Uint8Array(km.slice(32, 44));
    const cipher = chacha20poly1305(key, nonce);
    return Buffer.from(cipher.decrypt(new Uint8Array(ct))); // 32 bytes
  }

  _parseAddr(addr) {
    const idx = addr.lastIndexOf(':');
    return { host: addr.slice(0, idx), port: parseInt(addr.slice(idx + 1), 10) };
  }

  async connect() {
    await loadCrypto();
    this._loadOrGenerateE2EKey();

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

    socket.on('data',  (d) => this._onData(d));
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
      timeout: 10000,
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
        } catch (e) { reject(e); }
      };
      const onErr = (e) => reject(e);
      socket.on('data', onData);
      socket.once('error', onErr);
    });
  }

  _buildFrame(cmd, payloadBuf) {
    const total = 1 + payloadBuf.length;
    const frame = Buffer.alloc(2 + total);
    frame.writeUInt16LE(total, 0);
    frame[2] = cmd;
    payloadBuf.copy(frame, 3);
    return frame;
  }

  // ---- Register ----
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
        if (this._registerResolve) { this._registerResolve = null; resolve({ ok: false, error: 'Timeout' }); }
      }, 5000);
    });
  }

  // ---- Login ----
  login(login, pass) {
    return new Promise((resolve) => {
      this._loginHandled = false;
      this._loginResolve = resolve;
      this._myLogin = login;
      const lBuf = Buffer.from(login);
      const pBuf = Buffer.from(pass);
      const payload = Buffer.alloc(1 + lBuf.length + pBuf.length);
      payload[0] = lBuf.length;
      lBuf.copy(payload, 1);
      pBuf.copy(payload, 1 + lBuf.length);
      this.socket.write(this._buildFrame(CMD.LOGIN, payload));
      setTimeout(() => {
        if (this._loginResolve) { this._loginResolve = null; resolve({ ok: false, error: 'Timeout' }); }
      }, 8000);
    });
  }

  // ---- Upload our E2E public key ----
  _sendSetPublicKey() {
    if (!this._e2ePubKey) return;
    this.socket.write(this._buildFrame(CMD.SET_PUBLIC_KEY, Buffer.from(this._e2ePubKey)));
  }

  // ---- Request public key for a user ----
  _sendPublicKeyRequest(username) {
    const uBuf = Buffer.from(username);
    const payload = Buffer.alloc(1 + uBuf.length);
    payload[0] = uBuf.length;
    uBuf.copy(payload, 1);
    this.socket.write(this._buildFrame(CMD.PUBLIC_KEY_REQUEST, payload));
  }

  // ---- Send E2E message ----
  sendMessage(text) {
    if (!this._e2ePrivKey) return false;

    // Collect recipients + check for missing pubkeys
    const recipients = new Map();
    const missing = [];
    for (const [, name] of this._sidNames) {
      if (this._knownPubkeys.has(name)) {
        recipients.set(name, this._knownPubkeys.get(name));
      } else {
        missing.push(name);
      }
    }
    // Add self
    if (this._myLogin) {
      if (!this._knownPubkeys.has(this._myLogin)) {
        this._knownPubkeys.set(this._myLogin, Buffer.from(this._e2ePubKey));
      }
      recipients.set(this._myLogin, this._knownPubkeys.get(this._myLogin));
    }

    if (missing.length > 0) {
      this._pendingMessages.push(text);
      missing.forEach(n => this._sendPublicKeyRequest(n));
      return true;
    }

    this._doSendE2EMessage(text, recipients);
    return true;
  }

  _doSendE2EMessage(text, recipients) {
    // 1. Random msgKey + nonce
    const msgKey = crypto.randomBytes(32);
    const nonce  = crypto.randomBytes(12);

    // 2. Encrypt content
    const msgU   = new TextEncoder().encode(text);
    const cipher = chacha20poly1305(new Uint8Array(msgKey), new Uint8Array(nonce));
    const encContent = Buffer.from(cipher.encrypt(msgU));

    // 3. Build envelopes
    const envelopes = [];
    for (const [login, pubkey] of recipients) {
      try {
        const env = this._sealEnvelope(pubkey, msgKey);
        envelopes.push({ login, env });
      } catch (e) {
        console.warn('[E2E] envelope error for', login, e);
      }
    }

    // 4. Assemble payload
    // [Nonce(12)][EncContentLen(2 LE)][EncContent(N)][EnvelopeCount(1)]
    // per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
    const ecLenBuf = Buffer.alloc(2);
    ecLenBuf.writeUInt16LE(encContent.length, 0);
    const envParts = envelopes.map(({ login, env }) => {
      const lb = Buffer.from(login);
      return Buffer.concat([Buffer.from([lb.length]), lb, env]);
    });
    const assembled = Buffer.concat([
      nonce, ecLenBuf, encContent,
      Buffer.from([envelopes.length]),
      ...envParts,
    ]);

    // 5. Fragment with CmdE2EMsg prefix
    this._sendFragmented(CMD.E2E_MSG, assembled);
  }

  // ---- Fragment and send with cmd prefix ----
  _sendFragmented(cmd, payloadBuf) {
    const full = Buffer.concat([Buffer.from([cmd]), payloadBuf]);
    this._fragCounter = (this._fragCounter + 1) & 0xFF;
    const msgID    = this._fragCounter;
    const maxChunk = MAX_FRAME_SIZE - 6;
    const chunks   = [];
    let offset = 0;
    while (offset < full.length) {
      chunks.push(full.slice(offset, offset + maxChunk));
      offset += maxChunk;
    }
    if (chunks.length > 255) { console.error('[E2E] Message too large'); return; }
    const fragCount = chunks.length;
    chunks.forEach((chunk, idx) => {
      const fp = Buffer.alloc(3 + chunk.length);
      fp[0] = msgID; fp[1] = idx; fp[2] = fragCount;
      chunk.copy(fp, 3);
      this.socket.write(this._buildFrame(CMD.FRAGMENT, fp));
    });
  }

  // ---- Flush pending messages once pubkeys arrive ----
  _flushPending() {
    const msgs = this._pendingMessages.splice(0);
    for (const text of msgs) {
      const recipients = new Map();
      const missing = [];
      for (const [, name] of this._sidNames) {
        if (this._knownPubkeys.has(name)) recipients.set(name, this._knownPubkeys.get(name));
        else missing.push(name);
      }
      if (this._myLogin) recipients.set(this._myLogin, Buffer.from(this._e2ePubKey));
      if (missing.length > 0) {
        this._pendingMessages.push(text); // re-queue
      } else {
        this._doSendE2EMessage(text, recipients);
      }
    }
  }

  // ---- Incoming data ----
  _onData(chunk) {
    this._buf = Buffer.concat([this._buf, chunk]);
    this._parse();
  }

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
          // Upload E2E pubkey after successful login
          this._sendSetPublicKey();
          const r = this._loginResolve; this._loginResolve = null;
          r({ ok: true });
        }
        break;

      case CMD.LOGIN_FAIL:
        if (this._loginResolve && !this._loginHandled) {
          this._loginHandled = true;
          const r = this._loginResolve; this._loginResolve = null;
          r({ ok: false, error: 'Invalid login or password' });
        }
        break;

      case 0x00:
        if (this._registerResolve) { const r = this._registerResolve; this._registerResolve = null; r({ ok: false }); }
        break;

      case CMD.REGISTER:
        if (this._registerResolve) { const r = this._registerResolve; this._registerResolve = null; r({ ok: true }); }
        break;

      case CMD.ACK:
        break;

      case CMD.HISTORY_END:
        this.emit('history-end');
        break;

      case CMD.E2E_HISTORY: {
        // [SenderLen(1)][Sender(N)][Timestamp(4 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
        if (payload.length < 103) break;
        const senderLen = payload[0];
        if (1 + senderLen + 4 + 4 + 12 + 2 + 80 > payload.length) break;
        const sender   = payload.slice(1, 1 + senderLen).toString();
        const tsOff    = 1 + senderLen;
        const ts       = payload.readUInt32BE(tsOff);
        // MsgID at tsOff+4 (4 LE) — skip
        const blobOff  = tsOff + 4 + 4;
        const storedBlob = payload.slice(blobOff, payload.length - 80);
        const envelope   = payload.slice(payload.length - 80);
        if (storedBlob.length < 14) break;
        const nonce         = storedBlob.slice(0, 12);
        const encContentLen = storedBlob.readUInt16LE(12);
        if (storedBlob.length < 14 + encContentLen) break;
        const ciphertext = storedBlob.slice(14, 14 + encContentLen);
        try {
          const msgKey = this._openEnvelope(envelope);
          const cipher = chacha20poly1305(new Uint8Array(msgKey), new Uint8Array(nonce));
          const plain  = Buffer.from(cipher.decrypt(new Uint8Array(ciphertext))).toString();
          const time   = new Date(ts * 1000).toLocaleString([], {
            year: 'numeric', month: '2-digit', day: '2-digit',
            hour: '2-digit', minute: '2-digit',
          });
          this.emit('history', { sender, text: plain, time });
        } catch (_) {}
        break;
      }

      case CMD.E2E_INCOMING: {
        // [SenderSID(2 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
        if (payload.length < 100) break;
        const senderSID  = (payload[0] << 8) | payload[1];
        const storedBlob = payload.slice(6, payload.length - 80);
        const envelope   = payload.slice(payload.length - 80);
        if (storedBlob.length < 14) break;
        const nonce         = storedBlob.slice(0, 12);
        const encContentLen = storedBlob.readUInt16LE(12);
        if (storedBlob.length < 14 + encContentLen) break;
        const ciphertext = storedBlob.slice(14, 14 + encContentLen);
        try {
          const msgKey     = this._openEnvelope(envelope);
          const cipher     = chacha20poly1305(new Uint8Array(msgKey), new Uint8Array(nonce));
          const plain      = Buffer.from(cipher.decrypt(new Uint8Array(ciphertext))).toString();
          const senderName = this._sidNames.get(senderSID) || `SID:${senderSID}`;
          const now        = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          this.emit('message', senderName, plain, now);
        } catch (_) {}
        break;
      }

      case CMD.PUBLIC_KEY: {
        // [UsernameLen(1)][Username(N)][pubkey(32)]
        if (payload.length < 1 + 32) break;
        const uLen     = payload[0];
        if (payload.length < 1 + uLen + 32) break;
        const username = payload.slice(1, 1 + uLen).toString();
        const pubkey   = payload.slice(1 + uLen, 1 + uLen + 32);
        this._knownPubkeys.set(username, pubkey);
        this._flushPending();
        break;
      }

      case CMD.ONLINE_LIST: {
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
          // Request pubkeys for all online users
          names.forEach(n => { if (!this._knownPubkeys.has(n)) this._sendPublicKeyRequest(n); });
          this.emit('online-list', names);
        }
        break;
      }

      case CMD.ONLINE_ADD: {
        if (payload.length < 4) break;
        const sid  = (payload[0] << 8) | payload[1];
        const nLen = payload[2];
        if (3 + nLen > payload.length) break;
        const name = payload.slice(3, 3 + nLen).toString();
        this._sidNames.set(sid, name);
        if (!this._knownPubkeys.has(name)) this._sendPublicKeyRequest(name);
        this.emit('online-list', [...this._sidNames.values()]);
        break;
      }

      case CMD.ONLINE_REMOVE: {
        if (payload.length < 2) break;
        const sid = (payload[0] << 8) | payload[1];
        this._sidNames.delete(sid);
        this.emit('online-list', [...this._sidNames.values()]);
        break;
      }

      case CMD.SERVER_LIST: {
        if (payload.length < 1) break;
        const count = payload[0];
        let off = 1;
        const servers = [];
        for (let i = 0; i < count; i++) {
          if (off >= payload.length) break;
          const aLen = payload[off++];
          if (off + aLen > payload.length) break;
          servers.push(payload.slice(off, off + aLen).toString());
          off += aLen;
        }
        this._knownServers = servers;
        this.emit('server-list', servers);
        break;
      }

      // Unknown command — ignore
    }
  }

  destroy() {
    if (this.socket) { this.socket.destroy(); this.socket = null; }
  }
}

// Expose CMD for the main process
MessengerClient.CMD = CMD;
module.exports = MessengerClient;
