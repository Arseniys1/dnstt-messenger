'use strict';

const I18N = window.I18N || { en: {}, ru: {} };

let currentLanguage = 'en';
let myUsername = '';
let connected = false;
let currentView = { type: 'global', id: null };

const msgStore = { global: [], dm: new Map(), room: new Map() };
const unread = { dm: new Map(), room: new Map() };
const rooms = new Map();
const dmPartners = new Set();

const screenConnect = document.getElementById('screen-connect');
const screenChat = document.getElementById('screen-chat');
const statusEl = document.getElementById('connect-status');
const onlineListEl = document.getElementById('online-list');
const messagesEl = document.getElementById('messages');
const msgInput = document.getElementById('msg-input');
const inputBar = document.getElementById('input-bar');
const myUsernameEl = document.getElementById('my-username');
const chatTitle = document.getElementById('chat-title');
const chatActions = document.getElementById('chat-header-actions');
const dmListEl = document.getElementById('dm-list');
const roomListEl = document.getElementById('room-list-sidebar');
const navGlobal = document.getElementById('nav-global');
const settingsView = document.getElementById('settings-view');

const chatCfgServer = document.getElementById('chat-cfg-server');
const chatCfgProxy = document.getElementById('chat-cfg-proxy');
const chatCfgDirect = document.getElementById('chat-cfg-direct');
const chatCfgLanguage = document.getElementById('chat-cfg-language');
const chatSettingsStatus = document.getElementById('chat-settings-status');

function t(key, vars = {}) {
  const template = (I18N[currentLanguage] && I18N[currentLanguage][key]) || I18N.en[key] || key;
  return template.replace(/\{(\w+)\}/g, (_, name) => vars[name] ?? '');
}

function applyTranslations() {
  document.documentElement.lang = currentLanguage;
  document.getElementById('tab-btn-login').textContent = t('tabLogin');
  document.getElementById('tab-btn-register').textContent = t('tabRegister');
  document.getElementById('tab-btn-settings').textContent = t('tabSettings');
  document.getElementById('login-user').placeholder = t('login');
  document.getElementById('login-pass').placeholder = t('password');
  document.getElementById('btn-login').textContent = t('signIn');
  document.getElementById('reg-user').placeholder = t('login');
  document.getElementById('reg-pass').placeholder = t('password');
  document.getElementById('btn-register').textContent = t('createAccount');
  document.getElementById('label-server').textContent = t('serverAddress');
  document.getElementById('label-proxy').textContent = t('socksProxy');
  document.getElementById('label-direct').textContent = t('directConnection');
  document.getElementById('label-language').textContent = t('language');
  document.getElementById('btn-save-cfg').textContent = t('save');
  document.getElementById('label-known-servers').textContent = t('knownServers');
  const hint = document.getElementById('server-list-hint');
  if (hint) hint.textContent = t('knownServersHint');

  document.getElementById('btn-logout').title = t('signOut');
  document.getElementById('btn-open-settings').title = t('tabSettings');
  document.getElementById('sidebar-global-label').textContent = t('globalChatSection');
  document.getElementById('nav-global-text').textContent = t('general');
  document.getElementById('sidebar-dm-label').textContent = t('directMessages');
  document.getElementById('btn-new-dm').title = t('modalNewDm');
  document.getElementById('sidebar-rooms-label').textContent = t('rooms');
  document.getElementById('btn-create-room').title = t('modalCreateRoom');
  document.getElementById('sidebar-online-label').textContent = t('online');
  document.getElementById('msg-input').placeholder = t('messagePlaceholder');

  document.getElementById('chat-settings-title').textContent = t('tabSettings');
  document.getElementById('chat-label-server').textContent = t('serverAddress');
  document.getElementById('chat-label-proxy').textContent = t('socksProxy');
  document.getElementById('chat-label-direct').textContent = t('directConnection');
  document.getElementById('chat-label-language').textContent = t('language');
  document.getElementById('btn-chat-save-cfg').textContent = t('save');
  document.getElementById('chat-label-known-servers').textContent = t('knownServers');
  const chatHint = document.getElementById('chat-server-list-hint');
  if (chatHint) chatHint.textContent = t('knownServersHint');

  document.getElementById('modal-dm-title').textContent = t('modalNewDm');
  document.getElementById('dm-target-user').placeholder = t('userLogin');
  document.getElementById('btn-dm-cancel').textContent = t('cancel');
  document.getElementById('btn-dm-open').textContent = t('open');
  document.getElementById('modal-room-title').textContent = t('modalCreateRoom');
  document.getElementById('room-name-input').placeholder = t('roomName');
  document.getElementById('room-desc-input').placeholder = t('descriptionOptional');
  document.getElementById('room-public-label').textContent = t('roomPublic');
  document.getElementById('btn-create-room-cancel').textContent = t('cancel');
  document.getElementById('btn-create-room-ok').textContent = t('create');
  document.getElementById('modal-invite-title').textContent = t('modalInvite');
  document.getElementById('invite-username').placeholder = t('userLogin');
  document.getElementById('btn-invite-cancel').textContent = t('cancel');
  document.getElementById('btn-invite-ok').textContent = t('invite');

  refreshCurrentView();
}

function refreshCurrentView() {
  if (screenChat.classList.contains('active')) switchView(currentView.type, currentView.id);
  else chatTitle.textContent = t('generalChatTitle');
  renderKnownServers(window._knownServersCache || []);
}

function setConfigForms(cfg) {
  const language = cfg.language || 'en';
  document.getElementById('cfg-server').value = cfg.server_addr || '';
  document.getElementById('cfg-proxy').value = cfg.proxy_addr || '';
  document.getElementById('cfg-direct').checked = !!cfg.direct_mode;
  document.getElementById('cfg-language').value = language;
  chatCfgServer.value = cfg.server_addr || '';
  chatCfgProxy.value = cfg.proxy_addr || '';
  chatCfgDirect.checked = !!cfg.direct_mode;
  chatCfgLanguage.value = language;
}

function setChatSettingsStatus(msg, type = '') {
  if (!chatSettingsStatus) return;
  chatSettingsStatus.textContent = msg || '';
  chatSettingsStatus.className = 'status ' + type;
}

async function persistConfig(cfg, options = { chatStatus: false }) {
  currentLanguage = cfg.language || 'en';
  await window.api.saveConfig(cfg);
  setConfigForms(cfg);
  applyTranslations();
  if (options.chatStatus) setChatSettingsStatus(t('settingsSaved'), 'ok');
}

document.querySelectorAll('.tab').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(tab => tab.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(tab => tab.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
  });
});

window.api.getConfig().then(cfg => {
  currentLanguage = cfg.language || 'en';
  setConfigForms(cfg);
  applyTranslations();
});

document.getElementById('cfg-language').addEventListener('change', e => {
  currentLanguage = e.target.value || 'en';
  chatCfgLanguage.value = currentLanguage;
  applyTranslations();
});

chatCfgLanguage.addEventListener('change', e => {
  currentLanguage = e.target.value || 'en';
  document.getElementById('cfg-language').value = currentLanguage;
  applyTranslations();
});

document.getElementById('btn-save-cfg').addEventListener('click', async () => {
  const cfg = {
    server_addr: document.getElementById('cfg-server').value.trim(),
    proxy_addr: document.getElementById('cfg-proxy').value.trim(),
    direct_mode: document.getElementById('cfg-direct').checked,
    language: document.getElementById('cfg-language').value || 'en',
  };
  await persistConfig(cfg);
  setStatus(t('settingsSaved'), 'ok');
});

document.getElementById('btn-chat-save-cfg').addEventListener('click', async () => {
  const cfg = {
    server_addr: chatCfgServer.value.trim(),
    proxy_addr: chatCfgProxy.value.trim(),
    direct_mode: chatCfgDirect.checked,
    language: chatCfgLanguage.value || 'en',
  };
  await persistConfig(cfg, { chatStatus: true });
});

document.getElementById('btn-register').addEventListener('click', async () => {
  const login = document.getElementById('reg-user').value.trim();
  const pass = document.getElementById('reg-pass').value.trim();
  if (!login || !pass) return setStatus(t('fillAllFields'), 'error');
  setStatus(t('connecting'));
  const cfg = await window.api.getConfig();
  currentLanguage = cfg.language || currentLanguage;
  const conn = await window.api.connect(cfg);
  if (!conn.ok) return setStatus(t('genericError', { error: conn.error }), 'error');
  const res = await window.api.register(login, pass);
  await window.api.disconnect();
  setStatus(res.ok ? t('accountCreated') : t('loginTaken'), res.ok ? 'ok' : 'error');
});

document.getElementById('btn-login').addEventListener('click', doLogin);
document.getElementById('login-pass').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });

async function doLogin() {
  const login = document.getElementById('login-user').value.trim();
  const pass = document.getElementById('login-pass').value.trim();
  if (!login || !pass) return setStatus(t('fillAllFields'), 'error');
  setStatus(t('connecting'));
  const cfg = await window.api.getConfig();
  currentLanguage = cfg.language || currentLanguage;
  setConfigForms(cfg);
  applyTranslations();
  const conn = await window.api.connect(cfg);
  if (!conn.ok) return setStatus(t('connectError', { error: conn.error }), 'error');
  setStatus(t('authorizing'));
  registerListeners();
  const res = await window.api.login(login, pass);
  if (!res.ok) {
    setStatus(t('invalidCredentials'), 'error');
    await window.api.disconnect();
    return;
  }
  myUsername = login;
  connected = true;
  showChat();
}

function registerListeners() {
  const channels = ['message', 'online-list', 'history', 'history-end', 'disconnected', 'server-list', 'dm', 'dm-history', 'room-list', 'room-created', 'room-members', 'room-member-add', 'room-member-rem', 'room-message', 'room-history'];
  channels.forEach(ch => window.api.removeAllListeners(ch));
  window.api.onServerList(servers => { window._knownServersCache = servers; renderKnownServers(servers); });
  window.api.onHistory(msg => { msgStore.global.push(msg); if (currentView.type === 'global') appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername); });
  window.api.onHistoryEnd(() => { if (currentView.type === 'global') appendDivider(`- ${t('historyEnd')} -`); scrollBottom(); });
  window.api.onMessage(({ sender, text, time }) => { if (sender !== myUsername) { msgStore.global.push({ sender, text, time }); if (currentView.type === 'global') { appendMessage(sender, text, time, false); scrollBottom(); } } });
  window.api.onOnlineList(users => renderOnlineList(users));
  window.api.onDisconnected(() => { if (connected) { connected = false; showConnect(); setStatus(t('disconnected'), 'error'); } });
  window.api.onDMHistory(({ sender, recipient, text, time }) => {
    const partner = sender === myUsername ? recipient : sender;
    if (!dmPartners.has(partner)) { dmPartners.add(partner); renderDMList(); }
    getOrCreate(msgStore.dm, partner).push({ sender, text, time });
    if (currentView.type === 'dm' && currentView.id === partner) appendMessage(sender, text, time, sender === myUsername);
  });
  window.api.onDM(({ sender, text, time }) => {
    if (!dmPartners.has(sender)) { dmPartners.add(sender); renderDMList(); }
    getOrCreate(msgStore.dm, sender).push({ sender, text, time });
    if (currentView.type === 'dm' && currentView.id === sender) { appendMessage(sender, text, time, false); scrollBottom(); } else bumpUnread('dm', sender);
  });
  window.api.onRoomList(items => { items.forEach(room => { if (!rooms.has(room.id)) rooms.set(room.id, { name: room.name, isPublic: room.isPublic, owner: room.owner, members: new Set() }); }); renderRoomList(); });
  window.api.onRoomCreated(({ id, name, isPublic, owner, inviter }) => { if (!rooms.has(id)) rooms.set(id, { name, isPublic, owner, members: new Set() }); renderRoomList(); if (inviter) appendSystemMsg(t('invitedToRoom', { inviter, name })); });
  window.api.onRoomMembers(({ id, members }) => { const room = rooms.get(id); if (room) room.members = new Set(members.map(m => m.login)); if (currentView.type === 'room' && currentView.id === id) renderRoomHeader(id); });
  window.api.onRoomMemberAdd(({ id, login }) => { const room = rooms.get(id); if (room) room.members.add(login); if (currentView.type === 'room' && currentView.id === id) { renderRoomHeader(id); appendSystemMsg(t('joinedRoom', { login })); } });
  window.api.onRoomMemberRem(({ id, login }) => { const room = rooms.get(id); if (room) room.members.delete(login); if (login === myUsername) { rooms.delete(id); renderRoomList(); if (currentView.type === 'room' && currentView.id === id) switchView('global'); } else if (currentView.type === 'room' && currentView.id === id) { renderRoomHeader(id); appendSystemMsg(t('leftRoom', { login })); } });
  window.api.onRoomHistory(({ roomID, sender, text, time }) => { getOrCreate(msgStore.room, roomID).push({ sender, text, time }); if (currentView.type === 'room' && currentView.id === roomID) appendMessage(sender, text, time, sender === myUsername); });
  window.api.onRoomMessage(({ roomID, sender, text, time }) => { if (sender !== myUsername) { getOrCreate(msgStore.room, roomID).push({ sender, text, time }); if (currentView.type === 'room' && currentView.id === roomID) { appendMessage(sender, text, time, false); scrollBottom(); } else bumpUnread('room', roomID); } });
}

navGlobal.addEventListener('click', () => switchView('global'));
document.getElementById('btn-open-settings').addEventListener('click', () => switchView('settings'));

function switchView(type, id) {
  currentView = { type, id: id ?? null };
  settingsView.classList.toggle('hidden', type !== 'settings');
  messagesEl.classList.toggle('hidden', type === 'settings');
  inputBar.classList.toggle('hidden', type === 'settings');
  navGlobal.classList.toggle('active', type === 'global');
  document.querySelectorAll('.dm-item').forEach(el => el.classList.toggle('active', type === 'dm' && el.dataset.user === id));
  document.querySelectorAll('.room-item').forEach(el => el.classList.toggle('active', type === 'room' && Number(el.dataset.roomid) === id));

  if (type === 'settings') {
    chatTitle.textContent = t('tabSettings');
    chatActions.innerHTML = '';
    setChatSettingsStatus('');
    return;
  }

  messagesEl.innerHTML = '';
  if (type === 'global') {
    chatTitle.textContent = `# ${t('generalChatTitle')}`;
    chatActions.innerHTML = '';
    msgStore.global.forEach(msg => appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername));
  } else if (type === 'dm') {
    chatTitle.textContent = `💬 ${id}`;
    chatActions.innerHTML = '';
    (msgStore.dm.get(id) || []).forEach(msg => appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername));
    unread.dm.delete(id);
    renderDMList();
  } else if (type === 'room') {
    const room = rooms.get(id);
    if (room && room.isPublic && !room.members.has(myUsername)) window.api.joinRoom(id);
    chatTitle.textContent = room ? `# ${room.name}` : `# ${t('roomTitleFallback', { id })}`;
    renderRoomHeader(id);
    (msgStore.room.get(id) || []).forEach(msg => appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername));
    unread.room.delete(id);
    renderRoomList();
  }
  appendDivider(`- ${t('historyEnd')} -`);
  scrollBottom();
}

function renderRoomHeader(roomID) {
  if (!rooms.get(roomID)) return;
  chatActions.innerHTML = '';
  const inviteBtn = document.createElement('button');
  inviteBtn.className = 'btn-header-action';
  inviteBtn.textContent = t('inviteAction');
  inviteBtn.addEventListener('click', () => openInviteModal(roomID));
  chatActions.appendChild(inviteBtn);
  const leaveBtn = document.createElement('button');
  leaveBtn.className = 'btn-header-action danger';
  leaveBtn.textContent = t('leaveAction');
  leaveBtn.addEventListener('click', async () => { await window.api.leaveRoom(roomID); });
  chatActions.appendChild(leaveBtn);
}

function renderDMList() {
  dmListEl.innerHTML = '';
  for (const partner of dmPartners) {
    const li = document.createElement('li');
    li.className = 'dm-item' + (currentView.type === 'dm' && currentView.id === partner ? ' active' : '');
    li.dataset.user = partner;
    li.textContent = partner;
    const cnt = unread.dm.get(partner) || 0;
    if (cnt > 0) { const badge = document.createElement('span'); badge.className = 'unread-badge'; badge.textContent = cnt; li.appendChild(badge); }
    li.addEventListener('click', () => switchView('dm', partner));
    dmListEl.appendChild(li);
  }
}

function renderRoomList() {
  roomListEl.innerHTML = '';
  for (const [id, room] of rooms) {
    const li = document.createElement('li');
    li.className = 'room-item' + (currentView.type === 'room' && currentView.id === id ? ' active' : '');
    li.dataset.roomid = id;
    li.textContent = `${room.isPublic ? '🌐' : '🔒'} ${room.name}`;
    const cnt = unread.room.get(id) || 0;
    if (cnt > 0) { const badge = document.createElement('span'); badge.className = 'unread-badge'; badge.textContent = cnt; li.appendChild(badge); }
    li.addEventListener('click', () => switchView('room', id));
    roomListEl.appendChild(li);
  }
}

function renderOnlineList(users) {
  onlineListEl.innerHTML = '';
  users.forEach(user => { const li = document.createElement('li'); li.textContent = user; onlineListEl.appendChild(li); });
}

document.getElementById('btn-send').addEventListener('click', sendMsg);
msgInput.addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMsg(); } });

function sendMsg() {
  const text = msgInput.value.trim();
  if (!text || !connected || currentView.type === 'settings') return;
  const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (currentView.type === 'global') {
    window.api.sendMessage(text);
    msgStore.global.push({ sender: myUsername, text, time: now });
  } else if (currentView.type === 'dm') {
    window.api.sendDM(currentView.id, text);
    getOrCreate(msgStore.dm, currentView.id).push({ sender: myUsername, text, time: now });
  } else if (currentView.type === 'room') {
    window.api.sendRoomMessage(currentView.id, text);
    getOrCreate(msgStore.room, currentView.id).push({ sender: myUsername, text, time: now });
  }
  appendMessage(myUsername, text, now, true);
  scrollBottom();
  msgInput.value = '';
  msgInput.style.height = 'auto';
}

msgInput.addEventListener('input', () => {
  msgInput.style.height = 'auto';
  msgInput.style.height = Math.min(msgInput.scrollHeight, 120) + 'px';
});

document.getElementById('btn-new-dm').addEventListener('click', () => {
  document.getElementById('dm-target-user').value = '';
  document.getElementById('modal-dm').classList.remove('hidden');
  document.getElementById('dm-target-user').focus();
});
document.getElementById('btn-dm-cancel').addEventListener('click', () => document.getElementById('modal-dm').classList.add('hidden'));
document.getElementById('btn-dm-open').addEventListener('click', openDM);
document.getElementById('dm-target-user').addEventListener('keydown', e => { if (e.key === 'Enter') openDM(); });

function openDM() {
  const user = document.getElementById('dm-target-user').value.trim();
  if (!user) return;
  document.getElementById('modal-dm').classList.add('hidden');
  if (!dmPartners.has(user)) { dmPartners.add(user); renderDMList(); }
  switchView('dm', user);
}

document.getElementById('btn-create-room').addEventListener('click', () => {
  document.getElementById('room-name-input').value = '';
  document.getElementById('room-desc-input').value = '';
  document.getElementById('room-public-check').checked = false;
  document.getElementById('modal-create-room').classList.remove('hidden');
  document.getElementById('room-name-input').focus();
});
document.getElementById('btn-create-room-cancel').addEventListener('click', () => document.getElementById('modal-create-room').classList.add('hidden'));
document.getElementById('btn-create-room-ok').addEventListener('click', doCreateRoom);
document.getElementById('room-name-input').addEventListener('keydown', e => { if (e.key === 'Enter') doCreateRoom(); });

async function doCreateRoom() {
  const name = document.getElementById('room-name-input').value.trim();
  const desc = document.getElementById('room-desc-input').value.trim();
  const isPublic = document.getElementById('room-public-check').checked;
  if (!name) return;
  document.getElementById('modal-create-room').classList.add('hidden');
  await window.api.createRoom(name, isPublic, desc);
}

let inviteRoomID = null;
function openInviteModal(roomID) {
  inviteRoomID = roomID;
  document.getElementById('invite-username').value = '';
  document.getElementById('modal-invite').classList.remove('hidden');
  document.getElementById('invite-username').focus();
}
document.getElementById('btn-invite-cancel').addEventListener('click', () => document.getElementById('modal-invite').classList.add('hidden'));
document.getElementById('btn-invite-ok').addEventListener('click', doInvite);
document.getElementById('invite-username').addEventListener('keydown', e => { if (e.key === 'Enter') doInvite(); });

async function doInvite() {
  const user = document.getElementById('invite-username').value.trim();
  if (!user || inviteRoomID === null) return;
  document.getElementById('modal-invite').classList.add('hidden');
  await window.api.inviteToRoom(inviteRoomID, user);
  inviteRoomID = null;
}

document.getElementById('btn-logout').addEventListener('click', async () => {
  connected = false;
  await window.api.disconnect();
  showConnect();
  setStatus('');
  setChatSettingsStatus('');
  onlineListEl.innerHTML = '';
  dmListEl.innerHTML = '';
  roomListEl.innerHTML = '';
  msgStore.global = [];
  msgStore.dm.clear();
  msgStore.room.clear();
  rooms.clear();
  dmPartners.clear();
  unread.dm.clear();
  unread.room.clear();
  currentView = { type: 'global', id: null };
});

function showChat() {
  screenConnect.classList.remove('active');
  screenChat.classList.add('active');
  myUsernameEl.textContent = myUsername;
  messagesEl.innerHTML = '';
  navGlobal.classList.add('active');
  settingsView.classList.add('hidden');
  messagesEl.classList.remove('hidden');
  inputBar.classList.remove('hidden');
  chatTitle.textContent = `# ${t('generalChatTitle')}`;
}

function showConnect() {
  screenChat.classList.remove('active');
  screenConnect.classList.add('active');
}

function appendMessage(sender, text, time, own) {
  const div = document.createElement('div');
  div.className = 'msg ' + (own ? 'own' : 'other');
  div.innerHTML = `<div class="meta">${escHtml(own ? t('you') : sender)} · ${escHtml(time)}</div>${escHtml(text)}`;
  messagesEl.appendChild(div);
}

function appendDivider(text) {
  const div = document.createElement('div');
  div.className = 'divider';
  div.textContent = text;
  messagesEl.appendChild(div);
}

function appendSystemMsg(text) { appendDivider(text); scrollBottom(); }
function scrollBottom() { messagesEl.scrollTop = messagesEl.scrollHeight; }
function setStatus(msg, type = '') { statusEl.textContent = msg; statusEl.className = 'status ' + type; }
function escHtml(str) { return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }
function getOrCreate(map, key) { if (!map.has(key)) map.set(key, []); return map.get(key); }
function bumpUnread(type, id) { if (type === 'dm') { unread.dm.set(id, (unread.dm.get(id) || 0) + 1); renderDMList(); } else if (type === 'room') { unread.room.set(id, (unread.room.get(id) || 0) + 1); renderRoomList(); } }

function renderKnownServers(servers) {
  window._knownServersCache = servers;
  renderKnownServersFor('server-list-container', servers, async (addr) => {
    document.getElementById('cfg-server').value = addr;
    document.getElementById('cfg-direct').checked = true;
    const cfg = {
      server_addr: addr,
      proxy_addr: document.getElementById('cfg-proxy').value.trim(),
      direct_mode: true,
      language: currentLanguage,
    };
    await persistConfig(cfg);
    document.querySelectorAll('.tab').forEach(tab => tab.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(tab => tab.classList.remove('active'));
    document.querySelector('.tab[data-tab="settings"]').classList.add('active');
    document.getElementById('tab-settings').classList.add('active');
    setStatus(t('serverSelected', { addr }), 'ok');
  });
  renderKnownServersFor('chat-server-list-container', servers, async (addr) => {
    chatCfgServer.value = addr;
    chatCfgDirect.checked = true;
    const cfg = {
      server_addr: addr,
      proxy_addr: chatCfgProxy.value.trim(),
      direct_mode: true,
      language: currentLanguage,
    };
    await persistConfig(cfg, { chatStatus: true });
    setChatSettingsStatus(t('serverSelected', { addr }), 'ok');
  });
}

function renderKnownServersFor(containerId, servers, onPick) {
  const container = document.getElementById(containerId);
  if (!container) return;
  container.innerHTML = '';
  if (!servers || servers.length === 0) {
    const small = document.createElement('small');
    small.textContent = t('knownServersEmpty');
    container.appendChild(small);
    return;
  }
  servers.forEach(addr => {
    const btn = document.createElement('button');
    btn.className = 'btn-server';
    btn.textContent = addr;
    btn.title = t('connectToServer', { addr });
    btn.addEventListener('click', async () => {
      await onPick(addr);
    });
    container.appendChild(btn);
  });
}
