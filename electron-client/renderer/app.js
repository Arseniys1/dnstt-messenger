'use strict';

// --- State ---
let myUsername = '';
let connected = false;

// --- DOM refs ---
const screenConnect = document.getElementById('screen-connect');
const screenChat    = document.getElementById('screen-chat');
const status        = document.getElementById('connect-status');
const onlineList    = document.getElementById('online-list');
const messages      = document.getElementById('messages');
const msgInput      = document.getElementById('msg-input');
const myUsernameEl  = document.getElementById('my-username');

// --- Tabs ---
document.querySelectorAll('.tab').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
  });
});

// --- Load config ---
window.api.getConfig().then(cfg => {
  document.getElementById('cfg-server').value  = cfg.server_addr  || '';
  document.getElementById('cfg-proxy').value   = cfg.proxy_addr   || '';
  document.getElementById('cfg-direct').checked = !!cfg.direct_mode;
});

// --- Save config ---
document.getElementById('btn-save-cfg').addEventListener('click', async () => {
  const cfg = {
    server_addr: document.getElementById('cfg-server').value.trim(),
    proxy_addr:  document.getElementById('cfg-proxy').value.trim(),
    direct_mode: document.getElementById('cfg-direct').checked
  };
  await window.api.saveConfig(cfg);
  setStatus('Настройки сохранены', 'ok');
});

// --- Register ---
document.getElementById('btn-register').addEventListener('click', async () => {
  const login = document.getElementById('reg-user').value.trim();
  const pass  = document.getElementById('reg-pass').value.trim();
  if (!login || !pass) { setStatus('Заполните все поля', 'error'); return; }

  setStatus('Подключение...');
  const cfg = await window.api.getConfig();
  const conn = await window.api.connect(cfg);
  if (!conn.ok) { setStatus('Ошибка: ' + conn.error, 'error'); return; }

  const res = await window.api.register(login, pass);
  await window.api.disconnect();
  if (res.ok) {
    setStatus('Аккаунт создан! Теперь войдите.', 'ok');
  } else {
    setStatus('Логин уже занят', 'error');
  }
});

// --- Login ---
document.getElementById('btn-login').addEventListener('click', doLogin);
document.getElementById('login-pass').addEventListener('keydown', e => {
  if (e.key === 'Enter') doLogin();
});

async function doLogin() {
  const login = document.getElementById('login-user').value.trim();
  const pass  = document.getElementById('login-pass').value.trim();
  if (!login || !pass) { setStatus('Заполните все поля', 'error'); return; }

  setStatus('Подключение...');
  const cfg = await window.api.getConfig();
  const conn = await window.api.connect(cfg);
  if (!conn.ok) { setStatus('Ошибка подключения: ' + conn.error, 'error'); return; }

  setStatus('Авторизация...');
  registerListeners();

  const res = await window.api.login(login, pass);
  if (!res.ok) {
    setStatus('Неверный логин или пароль', 'error');
    await window.api.disconnect();
    return;
  }

  myUsername = login;
  connected = true;
  showChat();
}

// --- IPC listeners ---
function registerListeners() {
  ['message','online-list','history','history-end','disconnected','ack','delivered'].forEach(ch =>
    window.api.removeAllListeners(ch)
  );

  window.api.onHistory(msg => {
    appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername);
  });

  window.api.onHistoryEnd(() => {
    appendDivider('— конец истории —');
    scrollBottom();
  });

  window.api.onMessage(({ sender, text, time, msgId }) => {
    if (sender === myUsername) return;
    appendMessage(sender, text, time, false, msgId);
    scrollBottom();
  });

  window.api.onAck((msgId) => {
    const el = pendingAckQueue.shift();
    if (el) el.dataset.msgId = msgId;
  });

  window.api.onDelivered((msgId) => {
    const el = messages.querySelector(`.msg.own[data-msg-id="${msgId}"]`);
    if (el) {
      const ticks = el.querySelector('.ticks');
      if (ticks) { ticks.textContent = '✓✓'; ticks.classList.add('read'); }
    }
  });

  window.api.onOnlineList(users => {
    onlineList.innerHTML = '';
    users.forEach(u => {
      const li = document.createElement('li');
      li.textContent = u;
      onlineList.appendChild(li);
    });
  });

  window.api.onDisconnected(() => {
    if (connected) {
      connected = false;
      showConnect();
      setStatus('Соединение разорвано', 'error');
    }
  });
}

// --- Chat UI ---
function showChat() {
  screenConnect.classList.remove('active');
  screenChat.classList.add('active');
  myUsernameEl.textContent = myUsername;
  messages.innerHTML = '';
}

function showConnect() {
  screenChat.classList.remove('active');
  screenConnect.classList.add('active');
}

// Очередь собственных сообщений ожидающих ACK (FIFO)
const pendingAckQueue = [];

function appendMessage(sender, text, time, own, msgId = null) {
  const div = document.createElement('div');
  div.className = 'msg ' + (own ? 'own' : 'other');
  if (msgId) div.dataset.msgId = msgId;

  const ticks = own ? '<span class="ticks">✓</span>' : '';
  div.innerHTML = `<div class="meta">${escHtml(own ? 'Вы' : sender)} · ${escHtml(time)}${ticks}</div>${escHtml(text)}`;
  messages.appendChild(div);

  if (own && !msgId) pendingAckQueue.push(div);
  return div;
}

function appendDivider(text) {
  const div = document.createElement('div');
  div.className = 'divider';
  div.textContent = text;
  messages.appendChild(div);
}

function scrollBottom() {
  messages.scrollTop = messages.scrollHeight;
}

function setStatus(msg, type = '') {
  status.textContent = msg;
  status.className = 'status ' + type;
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// --- Send message ---
document.getElementById('btn-send').addEventListener('click', sendMsg);
msgInput.addEventListener('keydown', e => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMsg();
  }
});

function sendMsg() {
  const text = msgInput.value.trim();
  if (!text || !connected) return;
  const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  // Добавляем сообщение синхронно, до IPC вызова
  const el = appendMessage(myUsername, text, now, true);
  scrollBottom();
  // IPC вызов — когда придёт ACK с msgId, привяжем его к этому элементу
  window.api.sendMessage(text).then(localId => {
    if (localId && el) el.dataset.localId = localId;
  });
  msgInput.value = '';
  msgInput.style.height = 'auto';
}

// Auto-resize textarea
msgInput.addEventListener('input', () => {
  msgInput.style.height = 'auto';
  msgInput.style.height = Math.min(msgInput.scrollHeight, 120) + 'px';
});

// --- Logout ---
document.getElementById('btn-logout').addEventListener('click', async () => {
  connected = false;
  pendingAckQueue.length = 0;
  await window.api.disconnect();
  showConnect();
  setStatus('');
  onlineList.innerHTML = '';
});
