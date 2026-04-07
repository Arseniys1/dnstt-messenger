const { app, BrowserWindow, ipcMain, Notification } = require('electron');
const path = require('path');
const fs = require('fs');
const MessengerClient = require('./net/client');

let win;
let client = null;

const CONFIG_PATH = path.join(app.getPath('userData'), 'config.json');

function loadConfig() {
  const defaults = {
    proxy_addr: '127.0.0.1:18000',
    server_addr: '127.0.0.1:9999',
    direct_mode: false
  };
  try {
    return { ...defaults, ...JSON.parse(fs.readFileSync(CONFIG_PATH, 'utf8')) };
  } catch {
    return defaults;
  }
}

function saveConfig(cfg) {
  fs.writeFileSync(CONFIG_PATH, JSON.stringify(cfg, null, 2));
}

function createWindow() {
  win = new BrowserWindow({
    width: 900,
    height: 650,
    minWidth: 600,
    minHeight: 450,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    },
    title: 'DNSTT Messenger',
    backgroundColor: '#1a1a2e'
  });
  win.loadFile(path.join(__dirname, 'renderer', 'index.html'));
  win.setMenuBarVisibility(false);
}

app.whenReady().then(createWindow);
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit(); });
app.on('activate', () => { if (BrowserWindow.getAllWindows().length === 0) createWindow(); });

// --- IPC handlers ---

ipcMain.handle('get-config', () => loadConfig());

ipcMain.handle('save-config', (_, cfg) => {
  saveConfig(cfg);
  return true;
});

ipcMain.handle('connect', async (_, cfg) => {
  if (client) { client.destroy(); client = null; }
  client = new MessengerClient(cfg);

  client.on('message', (sender, text, time) => {
    win.webContents.send('message', { sender, text, time });
    if (win.isMinimized() || !win.isVisible()) {
      new Notification({
        title: sender,
        body: text.length > 100 ? text.slice(0, 100) + '…' : text
      }).show();
    }
  });
  client.on('online-list', (users) => {
    win.webContents.send('online-list', users);
  });
  client.on('history', (msg) => {
    win.webContents.send('history', msg);
  });
  client.on('history-end', () => {
    win.webContents.send('history-end');
  });
  client.on('disconnected', () => {
    win.webContents.send('disconnected');
    client = null;
  });

  return client.connect();
});

ipcMain.handle('register', async (_, login, pass) => {
  if (!client) return { ok: false, error: 'Not connected' };
  return client.register(login, pass);
});

ipcMain.handle('login', async (_, login, pass) => {
  if (!client) return { ok: false, error: 'Not connected' };
  return client.login(login, pass);
});

ipcMain.handle('send-message', async (_, text) => {
  if (!client) return false;
  return client.sendMessage(text);
});

ipcMain.handle('disconnect', async () => {
  if (client) { client.destroy(); client = null; }
  return true;
});
