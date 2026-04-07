package main

import (
	"crypto/ecdh"
	"crypto/rand"
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
	ListenAddr   string `json:"listen_addr"`
	DBPath       string `json:"db_path"`
	MinPacketSize int   `json:"min_packet_size"`
	HistoryLimit  int   `json:"history_limit"`
}

const (
	CmdRegister   = 0x01
	CmdLogin      = 0x02
	CmdMsg        = 0x03
	CmdIncoming   = 0x04
	CmdHistory    = 0x05
	CmdHistoryEnd = 0x07
	CmdLoginOK    = 0x08
	CmdLoginFail  = 0x09
	CmdOnlineList = 0x0A
	CmdRead       = 0x0B // клиент -> сервер: прочитал сообщение msgID
	CmdDelivered  = 0x0C // сервер -> клиент: сообщение msgID прочитано
)

var (
	cfg        Config
	db         *sql.DB
	sessions   = make(map[uint16]string)
	conns      = make(map[uint16]net.Conn)
	keys       = make(map[uint16][]byte) // SID -> sharedKey
	msgSenders = make(map[int64]uint16)  // msgID -> senderSID
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

// ecdhHandshake выполняет X25519 хендшейк и возвращает общий 32-байтный ключ.
// Сервер первым отправляет свой публичный ключ, затем читает клиентский.
func ecdhHandshake(conn net.Conn) ([]byte, error) {
	curve := ecdh.X25519()

	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("генерация ключа: %w", err)
	}

	// Отправляем публичный ключ сервера (32 байта)
	_, err = conn.Write(privKey.PublicKey().Bytes())
	if err != nil {
		return nil, fmt.Errorf("отправка публичного ключа: %w", err)
	}

	// Читаем публичный ключ клиента (32 байта)
	clientPubBytes := make([]byte, 32)
	if _, err = readFull(conn, clientPubBytes); err != nil {
		return nil, fmt.Errorf("чтение публичного ключа клиента: %w", err)
	}

	clientPub, err := curve.NewPublicKey(clientPubBytes)
	if err != nil {
		return nil, fmt.Errorf("парсинг публичного ключа клиента: %w", err)
	}

	shared, err := privKey.ECDH(clientPub)
	if err != nil {
		return nil, fmt.Errorf("вычисление общего секрета: %w", err)
	}

	return shared, nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	sharedKey, err := ecdhHandshake(conn)
	if err != nil {
		fmt.Printf("❌ ECDH хендшейк не удался (%s): %v\n", conn.RemoteAddr(), err)
		return
	}
	fmt.Printf("🔐 ECDH хендшейк выполнен (%s)\n", conn.RemoteAddr())

	var mySID uint16
	defer func() {
		if mySID != 0 {
			sessMu.Lock()
			login := sessions[mySID]
			delete(sessions, mySID)
			delete(conns, mySID)
			delete(keys, mySID)
			sessMu.Unlock()
			fmt.Printf("👋 Отключился: %s (SID: %d)\n", login, mySID)
			broadcastOnlineList()
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
			mySID = handleLogin(conn, payload, sharedKey)
		case CmdMsg:
			handleMessage(conn, mySID, payload, sharedKey)
		case CmdRead:
			handleRead(mySID, payload)
		}
	}
}

func handleRegister(conn net.Conn, data []byte) {
	if len(data) < 2 {
		conn.Write([]byte{0x00})
		return
	}
	lLen := int(data[0])
	if len(data) < 1+lLen {
		conn.Write([]byte{0x00})
		return
	}
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

func handleLogin(conn net.Conn, data []byte, sharedKey []byte) uint16 {
	if len(data) < 2 {
		conn.Write([]byte{CmdLoginFail})
		return 0
	}
	lLen := int(data[0])
	if len(data) < 1+lLen {
		conn.Write([]byte{CmdLoginFail})
		return 0
	}
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
		keys[sid] = sharedKey
		sessMu.Unlock()

		conn.Write([]byte{CmdLoginOK, byte(sid >> 8), byte(sid & 0xFF)})
		sendHistory(conn)
		sendOnlineListTo(conn)
		fmt.Printf("🔑 Вошел: %s (SID: %d)\n", login, sid)
		broadcastOnlineList()
		return sid
	}
	conn.Write([]byte{CmdLoginFail})
	return 0
}

func handleMessage(conn net.Conn, senderSID uint16, data []byte, sharedKey []byte) {
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

	decrypted, err := decryptWith(sharedKey, ciphertext, nonce)
	if err != nil {
		return
	}

	msgID := saveMessage(user, string(decrypted))
	fmt.Printf("📩 [%s]: %s\n", user, string(decrypted))

	// ACK с msgID: [0x06][msgID(4)]
	conn.Write([]byte{0x06,
		byte(msgID >> 24), byte(msgID >> 16), byte(msgID >> 8), byte(msgID),
	})

	sessMu.Lock()
	msgSenders[msgID] = senderSID
	sessMu.Unlock()

	broadcastEncrypted(senderSID, user, decrypted, msgID)
}

// handleRead: клиент прочитал сообщение msgID → уведомляем отправителя
// Формат: [msgID(4)]
func handleRead(readerSID uint16, data []byte) {
	if len(data) < 4 {
		return
	}
	msgID := int64(data[0])<<24 | int64(data[1])<<16 | int64(data[2])<<8 | int64(data[3])

	sessMu.RLock()
	senderSID, ok := msgSenders[msgID]
	senderConn := conns[senderSID]
	sessMu.RUnlock()

	if !ok || senderConn == nil || senderSID == readerSID {
		return
	}

	// CmdDelivered: [0x0C][msgID(4)]
	pkt := []byte{CmdDelivered,
		byte(msgID >> 24), byte(msgID >> 16), byte(msgID >> 8), byte(msgID),
	}
	senderConn.Write(pkt)
}

func broadcastEncrypted(senderSID uint16, senderName string, plaintext []byte, msgID int64) {
	senderBytes := []byte(senderName)

	sessMu.RLock()
	defer sessMu.RUnlock()

	for sid, c := range conns {
		if sid == senderSID {
			continue
		}
		key := keys[sid]
		aead, err := chacha20poly1305.New(key)
		if err != nil {
			continue
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			continue
		}
		ct := aead.Seal(nil, nonce, plaintext, nil)

		// CmdIncoming: [0x04][senderLen][sender][msgID(4)][nonce(12)][ct]
		packet := []byte{CmdIncoming, byte(len(senderBytes))}
		packet = append(packet, senderBytes...)
		packet = append(packet,
			byte(msgID>>24), byte(msgID>>16), byte(msgID>>8), byte(msgID),
		)
		packet = append(packet, nonce...)
		packet = append(packet, ct...)
		c.Write(packet)
	}
}

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

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	for _, m := range msgs {
		timeStr := m.createdAt
		if len(timeStr) > 16 {
			timeStr = timeStr[:16]
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

// buildOnlinePacket строит компактный пакет со списком онлайн пользователей.
// Формат: [0x0A][count][len1][name1]...[lenN][nameN]
func buildOnlinePacket() []byte {
	sessMu.RLock()
	defer sessMu.RUnlock()
	packet := []byte{CmdOnlineList, byte(len(sessions))}
	for _, name := range sessions {
		b := []byte(name)
		packet = append(packet, byte(len(b)))
		packet = append(packet, b...)
	}
	return packet
}

func sendOnlineListTo(conn net.Conn) {
	conn.Write(buildOnlinePacket())
}

func broadcastOnlineList() {
	packet := buildOnlinePacket()
	sessMu.RLock()
	defer sessMu.RUnlock()
	for _, c := range conns {
		c.Write(packet)
	}
}

func saveMessage(sender, content string) int64 {
	res, err := db.Exec("INSERT INTO messages (sender, content) VALUES (?, ?)", sender, content)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

func decryptWith(key, data, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, data, nil)
}

// readFull читает ровно len(buf) байт из conn.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func loadConfig(path string) {
	cfg = Config{
		ListenAddr:    "0.0.0.0:9999",
		DBPath:        "./messenger.db",
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
