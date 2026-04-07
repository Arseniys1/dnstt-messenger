const { contextBridge, ipcRenderer } = require('electron');

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
  onAck: (cb) => ipcRenderer.on('ack', (_, msgId) => cb(msgId)),
  onDelivered: (cb) => ipcRenderer.on('delivered', (_, msgId) => cb(msgId)),

  removeAllListeners: (ch) => ipcRenderer.removeAllListeners(ch)
});
