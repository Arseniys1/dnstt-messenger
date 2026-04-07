package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/chacha20poly1305"
	_ "modernc.org/sqlite"
)

type Config struct {
	ListenAddr    string `json:"listen_addr"`
	DBPath        string `json:"db_path"`
	MinPacketSize int    `json:"min_packet_size"`
	HistoryLimit  int    `json:"history_limit"`
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
)

var (
	cfg        Config
	db         *sql.DB
	sessions   = make(map[uint16]string)
	conns      = make(map[uint16]net.Conn)
	keys       = make(map[uint16][]byte)
	nonces     = make(map[uint16]*atomic.Uint64) // SID -> исходящий счётчик nonce
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

func ecdhHandshake(conn net.Conn) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if _, err = conn.Write(privKey.PublicKey().Bytes()); err != nil {
		return nil, err
	}
	clientPubBytes := make([]byte, 32)
	if _, err = readFull(conn, clientPubBytes); err != nil {
		return nil, err
	}
	clientPub, err := curve.NewPublicKey(clientPubBytes)
	if err != nil {
		return nil, err
	}
	return privKey.ECDH(clientPub)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	sharedKey, err := ecdhHandshake(conn)
	if err != nil {
		fmt.Printf("❌ ECDH failed (%s): %v\n", conn.RemoteAddr(), err)
		return
	}
	fmt.Printf("🔐 ECDH ok (%s)\n", conn.RemoteAddr())

	var mySID uint16
	var recvCounter uint64 // входящий счётчик для replay-защиты

	defer func() {
		if mySID != 0 {
			sessMu.Lock()
			login := sessions[mySID]
			delete(sessions, mySID)
			delete(conns, mySID)
			delete(keys, mySID)
			delete(nonces, mySID)
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
			mySID = handleLogin(conn, payload, sharedKey)
		case CmdMsg:
			// payload: [nonce(8)][ciphertext]
			handleMessage(mySID, payload, sharedKey, &recvCounter)
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
	if err != nil || storedPass != pass {
		conn.Write([]byte{CmdLoginFail})
		return 0
	}

	sessMu.Lock()
	sidCounter++
	sid := sidCounter
	sessions[sid] = login
	conns[sid] = conn
	keys[sid] = sharedKey
	nc := &atomic.Uint64{}
	nonces[sid] = nc
	sessMu.Unlock()

	conn.Write([]byte{CmdLoginOK, byte(sid >> 8), byte(sid & 0xFF)})
	sendHistory(conn, sharedKey, nc)
	fmt.Printf("🔑 Вошел: %s (SID: %d)\n", login, sid)
	return sid
}

func handleMessage(senderSID uint16, data []byte, sharedKey []byte, recvCounter *uint64) {
	// [nonce(8)][ciphertext]
	if senderSID == 0 || len(data) < 9 {
		return
	}

	cnt := binary.LittleEndian.Uint64(data[:8])
	// replay-защита: счётчик должен строго возрастать
	if cnt <= *recvCounter {
		return
	}
	*recvCounter = cnt

	nonce := make([]byte, 12)
	copy(nonce[:8], data[:8])
	ciphertext := data[8:]

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

	saveMessage(user, string(decrypted))
	fmt.Printf("📩 [%s]: %s\n", user, string(decrypted))

	broadcastEncrypted(senderSID, user, decrypted)
}

func broadcastEncrypted(senderSID uint16, senderName string, plaintext []byte) {
	senderBytes := []byte(senderName)

	sessMu.RLock()
	defer sessMu.RUnlock()

	for sid, c := range conns {
		if sid == senderSID {
			continue
		}
		key := keys[sid]
		nc := nonces[sid]

		aead, err := chacha20poly1305.New(key)
		if err != nil {
			continue
		}

		cntVal := nc.Add(1)
		nonce := make([]byte, 12)
		binary.LittleEndian.PutUint64(nonce[:8], cntVal)

		ct := aead.Seal(nil, nonce, plaintext, nil)

		// [CmdIncoming(1)][senderLen(1)][sender][nonce(8)][ciphertext]
		packet := []byte{CmdIncoming, byte(len(senderBytes))}
		packet = append(packet, senderBytes...)
		packet = append(packet, nonce[:8]...)
		packet = append(packet, ct...)
		c.Write(packet)
	}
}

// sendHistory: [CmdHistory(1)][senderLen(1)][sender][ts uint32 BE(4)][nonce(8)][ciphertext]
func sendHistory(conn net.Conn, key []byte, nc *atomic.Uint64) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		`SELECT sender, content, strftime('%s', created_at) FROM messages ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		conn.Write([]byte{CmdHistoryEnd})
		return
	}
	defer rows.Close()

	type msgRow struct {
		sender, content string
		ts              uint32
	}
	var msgs []msgRow
	for rows.Next() {
		var m msgRow
		var tsStr string
		rows.Scan(&m.sender, &m.content, &tsStr)
		var tsVal uint64
		fmt.Sscanf(tsStr, "%d", &tsVal)
		m.ts = uint32(tsVal)
		msgs = append(msgs, m)
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		conn.Write([]byte{CmdHistoryEnd})
		return
	}

	for _, m := range msgs {
		senderBytes := []byte(m.sender)
		contentBytes := []byte(m.content)
		if len(contentBytes) > 255 {
			contentBytes = contentBytes[:255]
		}

		cntVal := nc.Add(1)
		nonce := make([]byte, 12)
		binary.LittleEndian.PutUint64(nonce[:8], cntVal)
		ct := aead.Seal(nil, nonce, contentBytes, nil)

		// [CmdHistory(1)][senderLen(1)][sender][ts uint32 BE(4)][nonce(8)][ciphertext]
		packet := []byte{CmdHistory, byte(len(senderBytes))}
		packet = append(packet, senderBytes...)
		packet = binary.BigEndian.AppendUint32(packet, m.ts)
		packet = append(packet, nonce[:8]...)
		packet = append(packet, ct...)
		conn.Write(packet)
	}

	conn.Write([]byte{CmdHistoryEnd})
}

func saveMessage(sender, content string) {
	db.Exec("INSERT INTO messages (sender, content) VALUES (?, ?)", sender, content)
}

func decryptWith(key, data, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, data, nil)
}

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
