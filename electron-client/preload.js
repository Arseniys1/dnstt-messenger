const { contextBridge, ipcRenderer } = require('electron');

// Load I18n Manager for renderer
const I18nManager = require('./i18n/manager.js');
const i18nInstance = new I18nManager();

contextBridge.exposeInMainWorld('api', {
  getConfig: () => ipcRenderer.invoke('get-config'),
  saveConfig: (cfg) => ipcRenderer.invoke('save-config', cfg),
  connect: (cfg) => ipcRenderer.invoke('connect', cfg),
  register: (login, pass) => ipcRenderer.invoke('register', login, pass),
  login: (login, pass) => ipcRenderer.invoke('login', login, pass),
  sendMessage: (text) => ipcRenderer.invoke('send-message', text),
  disconnect: () => ipcRenderer.invoke('disconnect'),

  onMessage: (cb) => ipcRenderer.on('message', (_, d) => cb(d)),
  onOnlineList: (cb) => ipcRenderer.on('online-list', (_, d) => cb(d)),
  onHistory: (cb) => ipcRenderer.on('history', (_, d) => cb(d)),
  onHistoryEnd: (cb) => ipcRenderer.on('history-end', () => cb()),
  onDisconnected: (cb) => ipcRenderer.on('disconnected', () => cb()),
  onServerList: (cb) => ipcRenderer.on('server-list', (_, d) => cb(d)),
  getServerList: () => ipcRenderer.invoke('get-server-list'),

  removeAllListeners: (ch) => ipcRenderer.removeAllListeners(ch),

  // Direct messages
  sendDM: (recipientLogin, text) => ipcRenderer.invoke('send-dm', recipientLogin, text),

  // Rooms
  createRoom:      (name, isPublic, description) => ipcRenderer.invoke('create-room', name, isPublic, description),
  joinRoom:        (roomID)            => ipcRenderer.invoke('join-room', roomID),
  leaveRoom:       (roomID)            => ipcRenderer.invoke('leave-room', roomID),
  sendRoomMessage: (roomID, text)      => ipcRenderer.invoke('send-room-message', roomID, text),
  inviteToRoom:    (roomID, username)  => ipcRenderer.invoke('invite-to-room', roomID, username),
  getRooms:        ()                  => ipcRenderer.invoke('get-rooms'),

  // Room events
  onDM:            (cb) => ipcRenderer.on('dm',            (_, d) => cb(d)),
  onDMHistory:     (cb) => ipcRenderer.on('dm-history',    (_, d) => cb(d)),
  onRoomList:      (cb) => ipcRenderer.on('room-list',     (_, d) => cb(d)),
  onRoomCreated:   (cb) => ipcRenderer.on('room-created',  (_, d) => cb(d)),
  onRoomMembers:   (cb) => ipcRenderer.on('room-members',  (_, d) => cb(d)),
  onRoomMemberAdd: (cb) => ipcRenderer.on('room-member-add', (_, d) => cb(d)),
  onRoomMemberRem: (cb) => ipcRenderer.on('room-member-rem', (_, d) => cb(d)),
  onRoomMessage:   (cb) => ipcRenderer.on('room-message',  (_, d) => cb(d)),
  onRoomHistory:   (cb) => ipcRenderer.on('room-history',  (_, d) => cb(d)),
});

// Expose I18n functionality to renderer
contextBridge.exposeInMainWorld('i18n', {
  initialize: async (savedLanguage = null) => {
    return await i18nInstance.initialize(savedLanguage);
  },
  translate: (key, params = {}) => {
    return i18nInstance.translate(key, params);
  },
  setLanguage: async (languageCode) => {
    return await i18nInstance.setLanguage(languageCode);
  },
  getCurrentLanguage: () => {
    return i18nInstance.getCurrentLanguage();
  },
  getSupportedLanguages: () => {
    return i18nInstance.getSupportedLanguages();
  },
  isRTL: (languageCode = null) => {
    return i18nInstance.isRTL(languageCode);
  },
  detectSystemLanguage: () => {
    return i18nInstance.detectSystemLanguage();
  }
});

