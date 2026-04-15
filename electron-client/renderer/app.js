'use strict';

// ─── I18n Setup ──────────────────────────────────────────────────────────────
// I18n is exposed via preload.js through window.i18n

// Initialize i18n on load
document.addEventListener('DOMContentLoaded', async () => {
  // Load saved language preference from config
  const cfg = await window.api.getConfig();
  await window.i18n.initialize(cfg.language);
  populateLanguageSelector();
  updateUILanguage();
});

// Helper function to translate
function t(key, params = {}) {
  return window.i18n.translate(key, params);
}

/**
 * Set text direction based on language
 * @param {string} languageCode - Language code to determine direction
 */
function setTextDirection(languageCode) {
  const direction = window.i18n.isRTL(languageCode) ? 'rtl' : 'ltr';
  document.documentElement.setAttribute('dir', direction);
}

/**
 * Populate the language selector dropdown with supported languages
 */
function populateLanguageSelector() {
  const languageSelect = document.getElementById('cfg-language');
  if (!languageSelect) {
    console.error('Language selector element not found');
    return;
  }
  
  const supportedLanguages = window.i18n.getSupportedLanguages();
  const currentLanguage = window.i18n.getCurrentLanguage();
  
  console.log('Populating language selector with', supportedLanguages.length, 'languages');
  console.log('Current language:', currentLanguage);
  
  // Clear existing options
  languageSelect.innerHTML = '';
  
  // Add options for each supported language
  supportedLanguages.forEach(lang => {
    const option = document.createElement('option');
    option.value = lang.code;
    option.textContent = `${lang.nativeName} (${lang.name})`;
    if (lang.code === currentLanguage) {
      option.selected = true;
    }
    languageSelect.appendChild(option);
  });
  
  console.log('Language selector populated with', languageSelect.options.length, 'options');
  
  // Add change event listener
  languageSelect.addEventListener('change', handleLanguageChange);
}

/**
 * Handle language change from the dropdown
 */
async function handleLanguageChange(event) {
  const newLanguage = event.target.value;
  
  // Set the new language
  await window.i18n.setLanguage(newLanguage);
  
  // Update all UI text
  updateUILanguage();
  
  // Save language preference to config
  const cfg = await window.api.getConfig();
  cfg.language = newLanguage;
  await window.api.saveConfig(cfg);
  
  // Show confirmation message
  setStatus(t('settings.saved'), 'ok');
}

// Update UI elements with translated text
function updateUILanguage() {
  // Update document title
  document.title = t('app.title');
  
  // Update HTML lang attribute
  const currentLang = window.i18n.getCurrentLanguage();
  document.documentElement.lang = currentLang;
  
  // Update text direction for RTL languages
  setTextDirection(currentLang);
  
  // Update static text elements in index.html
  const logo = document.querySelector('.logo');
  if (logo) logo.textContent = '🔐 ' + t('app.name');
  
  // Update tabs
  const tabs = document.querySelectorAll('.tab');
  if (tabs[0]) tabs[0].textContent = t('login.tab_login');
  if (tabs[1]) tabs[1].textContent = t('login.tab_register');
  if (tabs[2]) tabs[2].textContent = t('login.tab_settings');
  
  // Update login tab
  const loginUser = document.getElementById('login-user');
  if (loginUser) loginUser.placeholder = t('login.username');
  const loginPass = document.getElementById('login-pass');
  if (loginPass) loginPass.placeholder = t('login.password');
  const btnLogin = document.getElementById('btn-login');
  if (btnLogin) btnLogin.textContent = t('login.button_login');
  
  // Update register tab
  const regUser = document.getElementById('reg-user');
  if (regUser) regUser.placeholder = t('login.username');
  const regPass = document.getElementById('reg-pass');
  if (regPass) regPass.placeholder = t('login.password');
  const btnRegister = document.getElementById('btn-register');
  if (btnRegister) btnRegister.textContent = t('login.button_register');
  
  // Update settings tab
  const settingsLabels = document.querySelectorAll('#tab-settings label');
  if (settingsLabels[0]) settingsLabels[0].textContent = t('settings.language');
  if (settingsLabels[1]) settingsLabels[1].textContent = t('settings.server_address');
  const cfgServer = document.getElementById('cfg-server');
  if (cfgServer) cfgServer.placeholder = 'host:port';
  if (settingsLabels[2]) settingsLabels[2].textContent = t('settings.proxy_address');
  const cfgProxy = document.getElementById('cfg-proxy');
  if (cfgProxy) cfgProxy.placeholder = '127.0.0.1:18000';
  const directLabel = document.querySelector('label.row-label');
  if (directLabel) {
    const checkbox = directLabel.querySelector('input');
    directLabel.textContent = '';
    directLabel.appendChild(checkbox);
    directLabel.appendChild(document.createTextNode(' ' + t('settings.direct_mode')));
  }
  const btnSaveCfg = document.getElementById('btn-save-cfg');
  if (btnSaveCfg) btnSaveCfg.textContent = t('settings.button_save');
  const serverListLabel = document.querySelectorAll('#tab-settings label')[4];
  if (serverListLabel) serverListLabel.textContent = t('settings.known_servers');
  const serverListContainer = document.getElementById('server-list-container');
  if (serverListContainer && serverListContainer.querySelector('small')) {
    serverListContainer.innerHTML = `<small>${t('settings.known_servers_hint')}</small>`;
  }
  
  // Update chat screen sidebar labels
  const sidebarLabels = document.querySelectorAll('.sidebar-section-label');
  if (sidebarLabels[0]) sidebarLabels[0].textContent = t('chat.title_global');
  if (sidebarLabels[1]) {
    const dmLabel = sidebarLabels[1];
    const btnNewDm = dmLabel.querySelector('#btn-new-dm');
    dmLabel.textContent = t('sidebar.section_dms');
    if (btnNewDm) dmLabel.appendChild(btnNewDm);
  }
  if (sidebarLabels[2]) {
    const roomLabel = sidebarLabels[2];
    const btnCreateRoom = roomLabel.querySelector('#btn-create-room');
    roomLabel.textContent = t('sidebar.section_rooms');
    if (btnCreateRoom) roomLabel.appendChild(btnCreateRoom);
  }
  if (sidebarLabels[3]) sidebarLabels[3].textContent = t('sidebar.section_online');
  
  // Update nav items
  const navGlobalSpan = navGlobal.querySelector('span.nav-icon');
  if (navGlobalSpan) {
    navGlobal.textContent = '';
    navGlobal.appendChild(navGlobalSpan);
    navGlobal.appendChild(document.createTextNode(' ' + t('chat.title_general').substring(2)));
  }
  
  // Update logout button
  const btnLogout = document.getElementById('btn-logout');
  if (btnLogout) btnLogout.title = t('sidebar.button_logout');
  
  // Update message input
  const msgInput = document.getElementById('msg-input');
  if (msgInput) msgInput.placeholder = t('chat.input_placeholder');
  
  // Update modal: New DM
  const modalDmTitle = document.querySelector('#modal-dm h3');
  if (modalDmTitle) modalDmTitle.textContent = t('dm.new_conversation');
  const dmTargetUser = document.getElementById('dm-target-user');
  if (dmTargetUser) dmTargetUser.placeholder = t('dm.target_user');
  const btnDmCancel = document.getElementById('btn-dm-cancel');
  if (btnDmCancel) btnDmCancel.textContent = t('dm.button_cancel');
  const btnDmOpen = document.getElementById('btn-dm-open');
  if (btnDmOpen) btnDmOpen.textContent = t('dm.button_open');
  
  // Update modal: Create room
  const modalCreateRoomTitle = document.querySelector('#modal-create-room h3');
  if (modalCreateRoomTitle) modalCreateRoomTitle.textContent = t('room.create_title');
  const roomNameInput = document.getElementById('room-name-input');
  if (roomNameInput) roomNameInput.placeholder = t('room.name');
  const roomDescInput = document.getElementById('room-desc-input');
  if (roomDescInput) roomDescInput.placeholder = t('room.description');
  const roomPublicLabel = document.querySelector('#modal-create-room label.row-label');
  if (roomPublicLabel) {
    const checkbox = roomPublicLabel.querySelector('input');
    roomPublicLabel.textContent = '';
    roomPublicLabel.appendChild(checkbox);
    roomPublicLabel.appendChild(document.createTextNode(' ' + t('room.public')));
  }
  const btnCreateRoomCancel = document.getElementById('btn-create-room-cancel');
  if (btnCreateRoomCancel) btnCreateRoomCancel.textContent = t('room.button_cancel');
  const btnCreateRoomOk = document.getElementById('btn-create-room-ok');
  if (btnCreateRoomOk) btnCreateRoomOk.textContent = t('room.button_create');
  
  // Update modal: Invite to room
  const modalInviteTitle = document.querySelector('#modal-invite h3');
  if (modalInviteTitle) modalInviteTitle.textContent = t('room.invite_title');
  const inviteUsername = document.getElementById('invite-username');
  if (inviteUsername) inviteUsername.placeholder = t('room.invite_username');
  const btnInviteCancel = document.getElementById('btn-invite-cancel');
  if (btnInviteCancel) btnInviteCancel.textContent = t('room.button_cancel');
  const btnInviteOk = document.getElementById('btn-invite-ok');
  if (btnInviteOk) btnInviteOk.textContent = t('room.button_invite_ok');
}

// ─── State ────────────────────────────────────────────────────────────────────
let myUsername = '';
let connected  = false;

// currentView: { type: 'global' | 'dm' | 'room', id: null | username | roomID }
let currentView = { type: 'global', id: null };

// Message stores: key → [{sender,text,time}]
const msgStore = {
  global: [],
  dm: new Map(),     // username → []
  room: new Map(),   // roomID   → []
};

// Unread counts
const unread = {
  dm:   new Map(), // username → count
  room: new Map(), // roomID   → count
};

// Known rooms: id → { name, isPublic, owner, members: Set }
const rooms = new Map();

// Known DM partners (to show in sidebar even before history loads)
const dmPartners = new Set();

// ─── DOM ─────────────────────────────────────────────────────────────────────
const screenConnect = document.getElementById('screen-connect');
const screenChat    = document.getElementById('screen-chat');
const statusEl      = document.getElementById('connect-status');
const onlineListEl  = document.getElementById('online-list');
const messagesEl    = document.getElementById('messages');
const msgInput      = document.getElementById('msg-input');
const myUsernameEl  = document.getElementById('my-username');
const chatTitle     = document.getElementById('chat-title');
const chatActions   = document.getElementById('chat-header-actions');
const dmListEl      = document.getElementById('dm-list');
const roomListEl    = document.getElementById('room-list-sidebar');
const navGlobal     = document.getElementById('nav-global');

// ─── Connect-screen tabs ─────────────────────────────────────────────────────
document.querySelectorAll('.tab').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
  });
});

// ─── Load / save config ──────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  window.api.getConfig().then(cfg => {
    document.getElementById('cfg-server').value   = cfg.server_addr  || '';
    document.getElementById('cfg-proxy').value    = cfg.proxy_addr   || '';
    document.getElementById('cfg-direct').checked = !!cfg.direct_mode;
    
    // Language selector will be populated by the i18n initialization above
    // No need to set it here as it's handled in populateLanguageSelector
  });
});

document.getElementById('btn-save-cfg').addEventListener('click', async () => {
  const cfg = {
    server_addr: document.getElementById('cfg-server').value.trim(),
    proxy_addr:  document.getElementById('cfg-proxy').value.trim(),
    direct_mode: document.getElementById('cfg-direct').checked,
  };
  await window.api.saveConfig(cfg);
  setStatus(t('settings.saved'), 'ok');
});

// ─── Register ────────────────────────────────────────────────────────────────
document.getElementById('btn-register').addEventListener('click', async () => {
  const login = document.getElementById('reg-user').value.trim();
  const pass  = document.getElementById('reg-pass').value.trim();
  if (!login || !pass) { setStatus(t('error.fill_all_fields'), 'error'); return; }
  setStatus(t('status.connecting'));
  const cfg  = await window.api.getConfig();
  const conn = await window.api.connect(cfg);
  if (!conn.ok) { setStatus(t('error.connection_failed', { error: conn.error }), 'error'); return; }
  const res = await window.api.register(login, pass);
  await window.api.disconnect();
  setStatus(res.ok ? t('success.account_created') : t('error.username_taken'), res.ok ? 'ok' : 'error');
});

// ─── Login ───────────────────────────────────────────────────────────────────
document.getElementById('btn-login').addEventListener('click', doLogin);
document.getElementById('login-pass').addEventListener('keydown', e => {
  if (e.key === 'Enter') doLogin();
});

async function doLogin() {
  const login = document.getElementById('login-user').value.trim();
  const pass  = document.getElementById('login-pass').value.trim();
  if (!login || !pass) { setStatus(t('error.fill_all_fields'), 'error'); return; }
  setStatus(t('status.connecting'));
  const cfg  = await window.api.getConfig();
  const conn = await window.api.connect(cfg);
  if (!conn.ok) { setStatus(t('error.connection_failed', { error: conn.error }), 'error'); return; }
  setStatus(t('status.authorizing'));
  registerListeners();
  const res = await window.api.login(login, pass);
  if (!res.ok) {
    setStatus(t('error.invalid_credentials'), 'error');
    await window.api.disconnect();
    return;
  }
  myUsername = login;
  connected  = true;
  showChat();
}

// ─── IPC listeners ───────────────────────────────────────────────────────────
function registerListeners() {
  const channels = [
    'message','online-list','history','history-end','disconnected','server-list',
    'dm','dm-history','room-list','room-created','room-members',
    'room-member-add','room-member-rem','room-message','room-history',
  ];
  channels.forEach(ch => window.api.removeAllListeners(ch));

  window.api.onServerList(servers => renderKnownServers(servers));

  // ── Global chat ──
  window.api.onHistory(msg => {
    msgStore.global.push(msg);
    if (currentView.type === 'global') appendMessage(msg.sender, msg.text, msg.time, msg.sender === myUsername);
  });

  window.api.onHistoryEnd(() => {
    if (currentView.type === 'global') appendDivider(t('chat.history_divider'));
    scrollBottom();
  });

  window.api.onMessage(({ sender, text, time }) => {
    if (sender === myUsername) return;
    const msg = { sender, text, time };
    msgStore.global.push(msg);
    if (currentView.type === 'global') {
      appendMessage(sender, text, time, false);
      scrollBottom();
    } else {
      bumpUnread('global');
    }
  });

  window.api.onOnlineList(users => renderOnlineList(users));

  window.api.onDisconnected(() => {
    if (connected) {
      connected = false;
      showConnect();
      setStatus(t('status.connection_lost'), 'error');
    }
  });

  // ── Direct messages ──
  window.api.onDMHistory(({ sender, recipient, text, time }) => {
    const partner = sender === myUsername ? recipient : sender;
    if (!dmPartners.has(partner)) {
      dmPartners.add(partner);
      renderDMList();
    }
    const arr = getOrCreate(msgStore.dm, partner);
    arr.push({ sender, text, time });
    if (currentView.type === 'dm' && currentView.id === partner) {
      appendMessage(sender, text, time, sender === myUsername);
    }
  });

  window.api.onDM(({ sender, text, time }) => {
    if (!dmPartners.has(sender)) {
      dmPartners.add(sender);
      renderDMList();
    }
    const arr = getOrCreate(msgStore.dm, sender);
    arr.push({ sender, text, time });
    if (currentView.type === 'dm' && currentView.id === sender) {
      appendMessage(sender, text, time, false);
      scrollBottom();
    } else {
      bumpUnread('dm', sender);
    }
  });

  // ── Rooms ──
  window.api.onRoomList(roomsArr => {
    roomsArr.forEach(r => {
      if (!rooms.has(r.id)) rooms.set(r.id, { name: r.name, isPublic: r.isPublic, owner: r.owner, members: new Set() });
    });
    renderRoomList();
  });

  window.api.onRoomCreated(({ id, name, isPublic, owner, inviter }) => {
    if (!rooms.has(id)) rooms.set(id, { name, isPublic, owner, members: new Set() });
    renderRoomList();
    if (inviter) appendSystemMsg(t('room.invited_by', { name, inviter }));
  });

  window.api.onRoomMembers(({ id, members }) => {
    const room = rooms.get(id);
    if (room) room.members = new Set(members.map(m => m.login));
    if (currentView.type === 'room' && currentView.id === id) renderRoomHeader(id);
  });

  window.api.onRoomMemberAdd(({ id, login }) => {
    const room = rooms.get(id);
    if (room) room.members.add(login);
    if (currentView.type === 'room' && currentView.id === id) {
      renderRoomHeader(id);
      appendSystemMsg(t('room.member_joined', { user: login }));
    }
  });

  window.api.onRoomMemberRem(({ id, login }) => {
    const room = rooms.get(id);
    if (room) room.members.delete(login);
    if (login === myUsername) {
      // we were removed / left
      rooms.delete(id);
      renderRoomList();
      if (currentView.type === 'room' && currentView.id === id) switchView('global');
    } else if (currentView.type === 'room' && currentView.id === id) {
      renderRoomHeader(id);
      appendSystemMsg(t('room.member_left', { user: login }));
    }
  });

  window.api.onRoomHistory(({ roomID, sender, text, time }) => {
    const arr = getOrCreate(msgStore.room, roomID);
    arr.push({ sender, text, time });
    if (currentView.type === 'room' && currentView.id === roomID) {
      appendMessage(sender, text, time, sender === myUsername);
    }
  });

  window.api.onRoomMessage(({ roomID, sender, text, time }) => {
    if (sender === myUsername) return;
    const arr = getOrCreate(msgStore.room, roomID);
    arr.push({ sender, text, time });
    if (currentView.type === 'room' && currentView.id === roomID) {
      appendMessage(sender, text, time, false);
      scrollBottom();
    } else {
      bumpUnread('room', roomID);
    }
  });
}

// ─── Navigation ───────────────────────────────────────────────────────────────
navGlobal.addEventListener('click', () => switchView('global'));

function switchView(type, id) {
  currentView = { type, id: id ?? null };
  messagesEl.innerHTML = '';

  // Sidebar active states
  navGlobal.classList.toggle('active', type === 'global');
  document.querySelectorAll('.dm-item').forEach(el => {
    el.classList.toggle('active', type === 'dm' && el.dataset.user === id);
  });
  document.querySelectorAll('.room-item').forEach(el => {
    el.classList.toggle('active', type === 'room' && Number(el.dataset.roomid) === id);
  });

  if (type === 'global') {
    chatTitle.textContent = t('chat.title_global');
    chatActions.innerHTML = '';
    msgStore.global.forEach(m => appendMessage(m.sender, m.text, m.time, m.sender === myUsername));
    unread.global = 0;

  } else if (type === 'dm') {
    chatTitle.textContent = `💬 ${id}`;
    chatActions.innerHTML = '';
    (msgStore.dm.get(id) || []).forEach(m => appendMessage(m.sender, m.text, m.time, m.sender === myUsername));
    unread.dm.delete(id);
    renderDMList();

  } else if (type === 'room') {
    const room = rooms.get(id);
    // Auto-join public rooms when the user clicks on them
    if (room && room.isPublic && !room.members.has(myUsername)) {
      window.api.joinRoom(id);
    }
    chatTitle.textContent = room ? `# ${room.name}` : `# ${t('room.title')} ${id}`;
    renderRoomHeader(id);
    (msgStore.room.get(id) || []).forEach(m => appendMessage(m.sender, m.text, m.time, m.sender === myUsername));
    unread.room.delete(id);
    renderRoomList();
  }

  appendDivider(t('chat.history_divider'));
  scrollBottom();
}

function renderRoomHeader(roomID) {
  const room = rooms.get(roomID);
  if (!room) return;
  chatActions.innerHTML = '';

  const inviteBtn = document.createElement('button');
  inviteBtn.className = 'btn-header-action';
  inviteBtn.textContent = t('room.button_invite');
  inviteBtn.addEventListener('click', () => openInviteModal(roomID));
  chatActions.appendChild(inviteBtn);

  const leaveBtn = document.createElement('button');
  leaveBtn.className = 'btn-header-action danger';
  leaveBtn.textContent = t('room.button_leave');
  leaveBtn.addEventListener('click', async () => {
    await window.api.leaveRoom(roomID);
  });
  chatActions.appendChild(leaveBtn);
}

// ─── Sidebar renders ──────────────────────────────────────────────────────────
function renderDMList() {
  dmListEl.innerHTML = '';
  for (const partner of dmPartners) {
    const li = document.createElement('li');
    li.className = 'dm-item' + (currentView.type === 'dm' && currentView.id === partner ? ' active' : '');
    li.dataset.user = partner;
    li.textContent = partner;
    const cnt = unread.dm.get(partner) || 0;
    if (cnt > 0) {
      const badge = document.createElement('span');
      badge.className = 'unread-badge';
      badge.textContent = cnt;
      li.appendChild(badge);
    }
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
    const prefix = room.isPublic ? '🌐' : '🔒';
    li.textContent = `${prefix} ${room.name}`;
    const cnt = unread.room.get(id) || 0;
    if (cnt > 0) {
      const badge = document.createElement('span');
      badge.className = 'unread-badge';
      badge.textContent = cnt;
      li.appendChild(badge);
    }
    li.addEventListener('click', () => switchView('room', id));
    roomListEl.appendChild(li);
  }
}

function renderOnlineList(users) {
  onlineListEl.innerHTML = '';
  users.forEach(u => {
    const li = document.createElement('li');
    li.textContent = u;
    onlineListEl.appendChild(li);
  });
}

// ─── Send message (context-aware) ────────────────────────────────────────────
document.getElementById('btn-send').addEventListener('click', sendMsg);
msgInput.addEventListener('keydown', e => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMsg(); }
});

function sendMsg() {
  const text = msgInput.value.trim();
  if (!text || !connected) return;
  const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  if (currentView.type === 'global') {
    window.api.sendMessage(text);
    const msg = { sender: myUsername, text, time: now };
    msgStore.global.push(msg);
    appendMessage(myUsername, text, now, true);

  } else if (currentView.type === 'dm') {
    window.api.sendDM(currentView.id, text);
    const arr = getOrCreate(msgStore.dm, currentView.id);
    const msg = { sender: myUsername, text, time: now };
    arr.push(msg);
    appendMessage(myUsername, text, now, true);

  } else if (currentView.type === 'room') {
    window.api.sendRoomMessage(currentView.id, text);
    const arr = getOrCreate(msgStore.room, currentView.id);
    const msg = { sender: myUsername, text, time: now };
    arr.push(msg);
    appendMessage(myUsername, text, now, true);
  }

  scrollBottom();
  msgInput.value = '';
  msgInput.style.height = 'auto';
}

msgInput.addEventListener('input', () => {
  msgInput.style.height = 'auto';
  msgInput.style.height = Math.min(msgInput.scrollHeight, 120) + 'px';
});

// ─── Modal: New DM ───────────────────────────────────────────────────────────
document.getElementById('btn-new-dm').addEventListener('click', () => {
  document.getElementById('dm-target-user').value = '';
  document.getElementById('modal-dm').classList.remove('hidden');
  document.getElementById('dm-target-user').focus();
});
document.getElementById('btn-dm-cancel').addEventListener('click', () => {
  document.getElementById('modal-dm').classList.add('hidden');
});
document.getElementById('btn-dm-open').addEventListener('click', openDM);
document.getElementById('dm-target-user').addEventListener('keydown', e => {
  if (e.key === 'Enter') openDM();
});

function openDM() {
  const user = document.getElementById('dm-target-user').value.trim();
  if (!user) return;
  document.getElementById('modal-dm').classList.add('hidden');
  if (!dmPartners.has(user)) {
    dmPartners.add(user);
    renderDMList();
  }
  switchView('dm', user);
}

// ─── Modal: Create room ───────────────────────────────────────────────────────
document.getElementById('btn-create-room').addEventListener('click', () => {
  document.getElementById('room-name-input').value = '';
  document.getElementById('room-desc-input').value = '';
  document.getElementById('room-public-check').checked = false;
  document.getElementById('modal-create-room').classList.remove('hidden');
  document.getElementById('room-name-input').focus();
});
document.getElementById('btn-create-room-cancel').addEventListener('click', () => {
  document.getElementById('modal-create-room').classList.add('hidden');
});
document.getElementById('btn-create-room-ok').addEventListener('click', doCreateRoom);
document.getElementById('room-name-input').addEventListener('keydown', e => {
  if (e.key === 'Enter') doCreateRoom();
});

async function doCreateRoom() {
  const name  = document.getElementById('room-name-input').value.trim();
  const desc  = document.getElementById('room-desc-input').value.trim();
  const pub   = document.getElementById('room-public-check').checked;
  if (!name) return;
  document.getElementById('modal-create-room').classList.add('hidden');
  await window.api.createRoom(name, pub, desc);
}

// ─── Modal: Invite to room ────────────────────────────────────────────────────
let _inviteRoomID = null;
function openInviteModal(roomID) {
  _inviteRoomID = roomID;
  document.getElementById('invite-username').value = '';
  document.getElementById('modal-invite').classList.remove('hidden');
  document.getElementById('invite-username').focus();
}
document.getElementById('btn-invite-cancel').addEventListener('click', () => {
  document.getElementById('modal-invite').classList.add('hidden');
});
document.getElementById('btn-invite-ok').addEventListener('click', doInvite);
document.getElementById('invite-username').addEventListener('keydown', e => {
  if (e.key === 'Enter') doInvite();
});

async function doInvite() {
  const user = document.getElementById('invite-username').value.trim();
  if (!user || _inviteRoomID === null) return;
  document.getElementById('modal-invite').classList.add('hidden');
  await window.api.inviteToRoom(_inviteRoomID, user);
  _inviteRoomID = null;
}

// ─── Logout ───────────────────────────────────────────────────────────────────
document.getElementById('btn-logout').addEventListener('click', async () => {
  connected = false;
  await window.api.disconnect();
  showConnect();
  setStatus('');
  onlineListEl.innerHTML = '';
  dmListEl.innerHTML     = '';
  roomListEl.innerHTML   = '';
  msgStore.global = [];
  msgStore.dm.clear();
  msgStore.room.clear();
  rooms.clear();
  dmPartners.clear();
  unread.dm.clear();
  unread.room.clear();
  currentView = { type: 'global', id: null };
});

// ─── UI helpers ──────────────────────────────────────────────────────────────
function showChat() {
  screenConnect.classList.remove('active');
  screenChat.classList.add('active');
  myUsernameEl.textContent = myUsername;
  messagesEl.innerHTML = '';
  navGlobal.classList.add('active');
  chatTitle.textContent = t('chat.title_global');
}

function showConnect() {
  screenChat.classList.remove('active');
  screenConnect.classList.add('active');
}

function appendMessage(sender, text, time, own) {
  const div = document.createElement('div');
  div.className = 'msg ' + (own ? 'own' : 'other');
  div.innerHTML = `<div class="meta">${escHtml(own ? t('chat.you') : sender)} · ${escHtml(time)}</div>${escHtml(text)}`;
  messagesEl.appendChild(div);
}

function appendDivider(text) {
  const div = document.createElement('div');
  div.className = 'divider';
  div.textContent = text;
  messagesEl.appendChild(div);
}

function appendSystemMsg(text) {
  const div = document.createElement('div');
  div.className = 'divider';
  div.textContent = text;
  messagesEl.appendChild(div);
  scrollBottom();
}

function scrollBottom() {
  messagesEl.scrollTop = messagesEl.scrollHeight;
}

function setStatus(msg, type = '') {
  statusEl.textContent = msg;
  statusEl.className   = 'status ' + type;
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function getOrCreate(map, key) {
  if (!map.has(key)) map.set(key, []);
  return map.get(key);
}

function bumpUnread(type, id) {
  if (type === 'global') {
    // show badge on global nav item
  } else if (type === 'dm') {
    unread.dm.set(id, (unread.dm.get(id) || 0) + 1);
    renderDMList();
  } else if (type === 'room') {
    unread.room.set(id, (unread.room.get(id) || 0) + 1);
    renderRoomList();
  }
}

// ─── Known servers ────────────────────────────────────────────────────────────
function renderKnownServers(servers) {
  const container = document.getElementById('server-list-container');
  if (!container) return;
  container.innerHTML = '';
  if (!servers || servers.length === 0) {
    container.innerHTML = `<small>${t('settings.server_list_empty')}</small>`;
    return;
  }
  servers.forEach(addr => {
    const btn = document.createElement('button');
    btn.className   = 'btn-server';
    btn.textContent = addr;
    btn.title       = t('status.connecting') + ' ' + addr;
    btn.addEventListener('click', async () => {
      document.getElementById('cfg-server').value  = addr;
      document.getElementById('cfg-direct').checked = true;
      const cfg = {
        server_addr: addr,
        proxy_addr:  document.getElementById('cfg-proxy').value.trim(),
        direct_mode: true,
      };
      await window.api.saveConfig(cfg);
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
      document.querySelector('.tab[data-tab="settings"]').classList.add('active');
      document.getElementById('tab-settings').classList.add('active');
      setStatus(t('settings.server_selected', { server: addr }), 'ok');
    });
    container.appendChild(btn);
  });
}
