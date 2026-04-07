package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	_ "modernc.org/sqlite"
)

type Config struct {
	ListenAddr    string `json:"listen_addr"`
	DBPath        string `json:"db_path"`
	SharedKey     string `json:"shared_key"`
	MinPacketSize int    `json:"min_packet_size"`
	HistoryLimit  int    `json:"history_limit"`
}

const (
	CmdRegister  = 0x01
	CmdLogin     = 0x02
	CmdMsg       = 0x03
	CmdIncoming  = 0x04
	CmdHistory   = 0x05
	CmdHistoryEnd = 0x07
	CmdLoginOK   = 0x08
	CmdLoginFail = 0x09
)

var (
	cfg        Config
	db         *sql.DB
	sessions   = make(map[uint16]string)   // SID -> Login
	conns      = make(map[uint16]net.Conn) // SID -> Conn
	sessMu     sync.RWMutex
	sidCounter uint16
)

func main() {
	loadConfig("config.json")
	initDB()
	defer db.Close()

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		fmt.Printf("❌ Ошибка запуска: %v\n", err)
		return
	}
	fmt.Printf("🚀 Server started on %s\n", cfg.ListenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	var mySID uint16

	defer func() {
		if mySID != 0 {
			sessMu.Lock()
			login := sessions[mySID]
			delete(sessions, mySID)
			delete(conns, mySID)
			sessMu.Unlock()
			fmt.Printf("👋 Отключился: %s (SID: %d)\n", login, mySID)
		}
	}()

	buf := make([]byte, 512)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if n < cfg.MinPacketSize {
			continue
		}

		cmd := buf[0]
		payload := buf[1:n]

		switch cmd {
		case CmdRegister:
			handleRegister(conn, payload)
		case CmdLogin:
			mySID = handleLogin(conn, payload)
		case CmdMsg:
			handleMessage(conn, mySID, payload)
		}
	}
}

func handleRegister(conn net.Conn, data []byte) {
	lLen := int(data[0])
	login := string(data[1 : 1+lLen])
	pass := string(data[1+lLen:])

	_, err := db.Exec("INSERT INTO users (login, password) VALUES (?, ?)", login, pass)
	if err != nil {
		conn.Write([]byte{0x00})
		return
	}
	fmt.Printf("📝 Новый пользователь: %s\n", login)
	conn.Write([]byte{0x01})
}

func handleLogin(conn net.Conn, data []byte) uint16 {
	lLen := int(data[0])
	login := string(data[1 : 1+lLen])
	pass := string(data[1+lLen:])

	var storedPass string
	err := db.QueryRow("SELECT password FROM users WHERE login = ?", login).Scan(&storedPass)

	if err == nil && storedPass == pass {
		sessMu.Lock()
		sidCounter++
		sid := sidCounter
		sessions[sid] = login
		conns[sid] = conn
		sessMu.Unlock()

		// [CmdLoginOK(1)][SID_hi(1)][SID_lo(1)] затем история
		conn.Write([]byte{CmdLoginOK, byte(sid >> 8), byte(sid & 0xFF)})
		sendHistory(conn)
		fmt.Printf("🔑 Вошел: %s (SID: %d)\n", login, sid)
		return sid
	}
	conn.Write([]byte{CmdLoginFail})
	return 0
}

func handleMessage(conn net.Conn, senderSID uint16, data []byte) {
	// [SID(2)][Nonce(12)][EncData]
	if senderSID == 0 || len(data) < 14 {
		return
	}
	nonce := data[2:14]
	ciphertext := data[14:]

	sessMu.RLock()
	user, ok := sessions[senderSID]
	sessMu.RUnlock()

	if !ok {
		return
	}

	decrypted, err := decrypt(ciphertext, nonce)
	if err != nil {
		return
	}

	saveMessage(user, string(decrypted))
	fmt.Printf("📩 [%s]: %s\n", user, string(decrypted))
	conn.Write([]byte{0x06}) // ACK отправителю

	broadcast(senderSID, user, nonce, ciphertext)
}

// broadcast рассылает сообщение всем клиентам кроме отправителя.
// Формат: [CmdIncoming(1)][SenderLen(1)][Sender][Nonce(12)][Ciphertext]
func broadcast(senderSID uint16, senderName string, nonce, ciphertext []byte) {
	senderBytes := []byte(senderName)
	packet := []byte{CmdIncoming, byte(len(senderBytes))}
	packet = append(packet, senderBytes...)
	packet = append(packet, nonce...)
	packet = append(packet, ciphertext...)

	sessMu.RLock()
	defer sessMu.RUnlock()

	for sid, c := range conns {
		if sid != senderSID {
			c.Write(packet)
		}
	}
}

// sendHistory отправляет последние N сообщений клиенту.
// Формат каждого: [CmdHistory(1)][SenderLen(1)][Sender][TimeLen(1)][Time][MsgLen(2)][PlainText]
// В конце: [CmdHistoryEnd(1)]
func sendHistory(conn net.Conn) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		`SELECT sender, content, created_at FROM messages ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		conn.Write([]byte{CmdHistoryEnd})
		return
	}
	defer rows.Close()

	type msgRow struct{ sender, content, createdAt string }
	var msgs []msgRow
	for rows.Next() {
		var m msgRow
		rows.Scan(&m.sender, &m.content, &m.createdAt)
		msgs = append(msgs, m)
	}

	// Переворачиваем: старые сообщения первыми
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	for _, m := range msgs {
		timeStr := m.createdAt
		if len(timeStr) > 16 {
			timeStr = timeStr[:16] // "2006-01-02 15:04"
		}
		senderBytes := []byte(m.sender)
		timeBytes := []byte(timeStr)
		contentBytes := []byte(m.content)
		msgLen := len(contentBytes)

		packet := []byte{CmdHistory, byte(len(senderBytes))}
		packet = append(packet, senderBytes...)
		packet = append(packet, byte(len(timeBytes)))
		packet = append(packet, timeBytes...)
		packet = append(packet, byte(msgLen>>8), byte(msgLen))
		packet = append(packet, contentBytes...)
		conn.Write(packet)
	}

	conn.Write([]byte{CmdHistoryEnd})
}

func saveMessage(sender, content string) {
	db.Exec("INSERT INTO messages (sender, content) VALUES (?, ?)", sender, content)
}

func loadConfig(path string) {
	// Defaults
	cfg = Config{
		ListenAddr:    "0.0.0.0:9999",
		DBPath:        "./messenger.db",
		SharedKey:     "12345678901234567890123456789012",
		MinPacketSize: 2,
		HistoryLimit:  50,
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
		return
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&cfg)
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		panic(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT UNIQUE,
		password TEXT
	);`)
	db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TEXT DEFAULT (datetime('now'))
	);`)
}

func decrypt(data, nonce []byte) ([]byte, error) {
	aead, _ := chacha20poly1305.New([]byte(cfg.SharedKey))
	return aead.Open(nil, nonce, data, nil)
}
