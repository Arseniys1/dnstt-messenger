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
  ['message','online-list','history','history-end','disconnected','server-list'].forEach(ch =>
    window.api.removeAllListeners(ch)
  );

  window.api.onServerList(servers => renderKnownServers(servers));

  window.api.onHistory(msg => {
    appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername);
  });

  window.api.onHistoryEnd(() => {
    appendDivider('— конец истории —');
    scrollBottom();
  });

  window.api.onMessage(({ sender, text, time }) => {
    if (sender === myUsername) return; // уже показали при отправке
    appendMessage(sender, text, time, false);
    scrollBottom();
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

function appendMessage(sender, text, time, own) {
  const div = document.createElement('div');
  div.className = 'msg ' + (own ? 'own' : 'other');
  div.innerHTML = `<div class="meta">${escHtml(own ? 'Вы' : sender)} · ${escHtml(time)}</div>${escHtml(text)}`;
  messages.appendChild(div);
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
  window.api.sendMessage(text);
  const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  appendMessage(myUsername, text, now, true);
  scrollBottom();
  msgInput.value = '';
  msgInput.style.height = 'auto';
}

// Auto-resize textarea
msgInput.addEventListener('input', () => {
  msgInput.style.height = 'auto';
  msgInput.style.height = Math.min(msgInput.scrollHeight, 120) + 'px';
});

// --- Known servers list (federation) ---
function renderKnownServers(servers) {
  const container = document.getElementById('server-list-container');
  if (!container) return;
  container.innerHTML = '';
  if (!servers || servers.length === 0) {
    container.innerHTML = '<small>Список пуст</small>';
    return;
  }
  servers.forEach(addr => {
    const btn = document.createElement('button');
    btn.className = 'btn-server';
    btn.textContent = addr;
    btn.title = 'Подключиться к ' + addr;
    btn.addEventListener('click', async () => {
      document.getElementById('cfg-server').value = addr;
      document.getElementById('cfg-direct').checked = true;
      const cfg = {
        server_addr: addr,
        proxy_addr:  document.getElementById('cfg-proxy').value.trim(),
        direct_mode: true
      };
      await window.api.saveConfig(cfg);
      // Switch to settings tab so user sees the update
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
      document.querySelector('.tab[data-tab="settings"]').classList.add('active');
      document.getElementById('tab-settings').classList.add('active');
      setStatus('Сервер выбран: ' + addr, 'ok');
    });
    container.appendChild(btn);
  });
}

// --- Logout ---
document.getElementById('btn-logout').addEventListener('click', async () => {
  connected = false;
  await window.api.disconnect();
  showConnect();
  setStatus('');
  onlineList.innerHTML = '';
});
