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

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/chacha20poly1305"
	_ "modernc.org/sqlite"
)

type Config struct {
	ListenAddr        string   `json:"listen_addr"`
	DBPath            string   `json:"db_path"`
	HistoryLimit      int      `json:"history_limit"`
	MaxFrameSize      int      `json:"max_frame_size"`
	S2SAddr           string   `json:"s2s_addr"`            // server-to-server gossip listen addr
	PublicAddr        string   `json:"public_addr"`         // client-facing external addr (empty = don't advertise)
	GossipEnabled     bool     `json:"gossip_enabled"`      // default true
	GossipIntervalSec int      `json:"gossip_interval_sec"` // default 60
	InitialPeers      []string `json:"peers"`               // s2s addrs of seed servers
}

const (
	CmdRegister     = 0x01
	CmdLogin        = 0x02
	CmdMsg          = 0x03
	CmdIncoming     = 0x04
	CmdHistory      = 0x05
	CmdAck          = 0x06
	CmdHistoryEnd   = 0x07
	CmdLoginOK      = 0x08
	CmdLoginFail    = 0x09
	CmdOnlineList   = 0x0A
	CmdOnlineAdd    = 0x0B
	CmdOnlineRemove = 0x0C
	CmdFragment     = 0x0D
	CmdServerList   = 0x0E
)

// clientState holds all per-session state (replaces separate sessions/conns/keys maps).
type clientState struct {
	conn      net.Conn
	key       []byte
	login     string
	sendNonce atomic.Uint64 // monotonic counter for server→client nonces
}

// fragKey identifies a reassembly buffer by session + message ID.
type fragKey struct {
	sid   uint16
	msgID uint8
}

// fragBuf holds received fragments for a single fragmented message.
type fragBuf struct {
	frags    [256][]byte
	total    uint8
	received uint8
}

var (
	cfg        Config
	db         *sql.DB
	clients    = make(map[uint16]*clientState)
	sessMu     sync.RWMutex
	sidCounter uint16

	fragMu  sync.Mutex
	fragMap = make(map[fragKey]*fragBuf)
)

func main() {
	loadConfig("config.json")
	initDB()
	defer db.Close()

	// Federation: load saved peers, seed from config, start S2S listener and gossip loop
	loadPeers()
	for _, addr := range cfg.InitialPeers {
		peerStore.AddSeed(addr)
	}
	go startS2SListener()
	go startGossipLoop()

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

// writeFrame sends [TotalLen(2 LE)][cmd(1)][payload...] to conn.
// TotalLen = 1 + len(payload).
func writeFrame(conn net.Conn, cmd byte, payload []byte) {
	total := 1 + len(payload)
	frame := make([]byte, 2+total)
	binary.LittleEndian.PutUint16(frame[0:2], uint16(total))
	frame[2] = cmd
	copy(frame[3:], payload)
	conn.Write(frame)
}

func ecdhHandshake(conn net.Conn) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("генерация ключа: %w", err)
	}
	_, err = conn.Write(privKey.PublicKey().Bytes())
	if err != nil {
		return nil, fmt.Errorf("отправка публичного ключа: %w", err)
	}
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
	var recvCounter uint64

	defer func() {
		if mySID != 0 {
			sessMu.Lock()
			login := clients[mySID].login
			delete(clients, mySID)
			sessMu.Unlock()
			fmt.Printf("👋 Отключился: %s (SID: %d)\n", login, mySID)
			broadcastOnlineRemove(mySID)

			fragMu.Lock()
			for k := range fragMap {
				if k.sid == mySID {
					delete(fragMap, k)
				}
			}
			fragMu.Unlock()
		}
	}()

	// Accumulating buffer: parse complete frames even when TCP delivers partial data.
	const maxPending = 65536 // 64 KB cap; disconnect client if exceeded
	var pending []byte
	tmp := make([]byte, 4096)

	for {
		n, err := conn.Read(tmp)
		if err != nil {
			return
		}
		pending = append(pending, tmp[:n]...)
		if len(pending) > maxPending {
			fmt.Printf("⚠️ Превышен буфер (%d байт) от %s — разрываем соединение\n", len(pending), conn.RemoteAddr())
			return
		}

		for {
			if len(pending) < 2 {
				break
			}
			frameLen := int(binary.LittleEndian.Uint16(pending[0:2]))
			if frameLen < 1 || len(pending) < 2+frameLen {
				break
			}
			cmd := pending[2]
			payload := make([]byte, frameLen-1)
			copy(payload, pending[3:2+frameLen])
			pending = pending[2+frameLen:]

			switch cmd {
			case CmdRegister:
				handleRegister(conn, payload)
			case CmdLogin:
				mySID = handleLogin(conn, payload, sharedKey)
			case CmdMsg:
				handleMessage(mySID, payload, sharedKey, &recvCounter)
			case CmdFragment:
				if mySID != 0 {
					if reassembled := handleFragment(mySID, payload); reassembled != nil {
						handleMessage(mySID, reassembled, sharedKey, &recvCounter)
					}
				}
			}
		}
	}
}

func handleRegister(conn net.Conn, data []byte) {
	if len(data) < 2 {
		writeFrame(conn, 0x00, nil)
		return
	}
	lLen := int(data[0])
	if len(data) < 1+lLen {
		writeFrame(conn, 0x00, nil)
		return
	}
	login := string(data[1 : 1+lLen])
	pass := string(data[1+lLen:])

	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		writeFrame(conn, 0x00, nil)
		return
	}
	_, err = db.Exec("INSERT INTO users (login, password) VALUES (?, ?)", login, string(hashed))
	if err != nil {
		writeFrame(conn, 0x00, nil)
		return
	}
	fmt.Printf("📝 Новый пользователь: %s\n", login)
	writeFrame(conn, 0x01, nil)
}

func handleLogin(conn net.Conn, data []byte, sharedKey []byte) uint16 {
	if len(data) < 2 {
		writeFrame(conn, CmdLoginFail, nil)
		return 0
	}
	lLen := int(data[0])
	if len(data) < 1+lLen {
		writeFrame(conn, CmdLoginFail, nil)
		return 0
	}
	login := string(data[1 : 1+lLen])
	pass := string(data[1+lLen:])

	var storedHash string
	err := db.QueryRow("SELECT password FROM users WHERE login = ?", login).Scan(&storedHash)

	if err == nil && bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pass)) == nil {
		sessMu.Lock()
		sidCounter++
		if sidCounter == 0 {
			sidCounter = 1 // skip 0 (reserved as "no session")
		}
		sid := sidCounter
		startSID := sid
		// Resolve collision: if all 65535 slots are taken this will loop, but
		// that is an extreme edge case we handle gracefully by failing login.
		for clients[sid] != nil {
			sidCounter++
			if sidCounter == 0 {
				sidCounter = 1
			}
			sid = sidCounter
			if sid == startSID { // full wrap — no free slot
				sessMu.Unlock()
				writeFrame(conn, CmdLoginFail, nil)
				return 0
			}
		}
		clients[sid] = &clientState{
			conn:  conn,
			key:   sharedKey,
			login: login,
		}
		sessMu.Unlock()

		// 1. Notify client of its SID
		writeFrame(conn, CmdLoginOK, []byte{byte(sid >> 8), byte(sid & 0xFF)})
		// 2. Send full online list (with SIDs) so client builds its sidNames map
		sendOnlineListTo(conn)
		// 3. Send message history
		sendHistory(conn)
		writeFrame(conn, CmdHistoryEnd, nil)
		// 4. Send list of known federated servers (if any)
		if payload := buildServerListPayload(); payload != nil {
			writeFrame(conn, CmdServerList, payload)
		}
		// 5. Notify all existing clients about the new user (delta update)
		broadcastOnlineAdd(sid, login)

		fmt.Printf("🔑 Вошел: %s (SID: %d)\n", login, sid)
		return sid
	}
	writeFrame(conn, CmdLoginFail, nil)
	return 0
}

func handleMessage(senderSID uint16, data []byte, sharedKey []byte, recvCounter *uint64) {
	// Payload: [Counter(8 LE)][Ciphertext(N+16)]
	// Minimum: 8 bytes counter + 16 bytes auth tag = 24 bytes
	if senderSID == 0 || len(data) < 24 {
		return
	}

	cntVal := binary.LittleEndian.Uint64(data[0:8])
	if cntVal <= *recvCounter {
		fmt.Printf("⚠️ Повторное сообщение от SID %d (счётчик %d <= %d)\n", senderSID, cntVal, *recvCounter)
		return
	}
	*recvCounter = cntVal

	// Nonce: 8-byte counter (LE) + 4 zero bytes = 12 bytes total
	nonce := make([]byte, 12)
	copy(nonce[:8], data[0:8])
	ciphertext := data[8:]

	sessMu.RLock()
	cs, ok := clients[senderSID]
	sessMu.RUnlock()
	if !ok {
		return
	}

	decrypted, err := decryptWith(sharedKey, ciphertext, nonce)
	if err != nil {
		return
	}

	saveMessage(cs.login, string(decrypted))
	fmt.Printf("📩 [%s]: %s\n", cs.login, string(decrypted))
	writeFrame(cs.conn, CmdAck, nil)

	broadcastEncrypted(senderSID, decrypted)
}

// handleFragment stores a fragment and returns the reassembled CmdMsg payload when all arrive.
func handleFragment(senderSID uint16, data []byte) []byte {
	if len(data) < 4 {
		return nil
	}
	msgID := data[0]
	fragIdx := data[1]
	fragCount := data[2]
	chunk := data[3:]

	if fragCount == 0 || fragIdx >= fragCount {
		return nil
	}

	key := fragKey{senderSID, msgID}
	fragMu.Lock()
	defer fragMu.Unlock()

	fb, ok := fragMap[key]
	if !ok {
		fb = &fragBuf{total: fragCount}
		fragMap[key] = fb
	}
	if fb.frags[fragIdx] == nil {
		fb.frags[fragIdx] = make([]byte, len(chunk))
		copy(fb.frags[fragIdx], chunk)
		fb.received++
	}
	if fb.received < fb.total {
		return nil
	}

	// All fragments received — reassemble in order
	var assembled []byte
	for i := uint8(0); i < fb.total; i++ {
		assembled = append(assembled, fb.frags[i]...)
	}
	delete(fragMap, key)
	return assembled
}

func broadcastEncrypted(senderSID uint16, plaintext []byte) {
	// Collect targets inside the lock (increment nonce atomically), then write
	// outside the lock so a slow/blocked socket can't stall new logins.
	type target struct {
		conn  net.Conn
		key   []byte
		cnt   uint64
	}

	sessMu.RLock()
	targets := make([]target, 0, len(clients))
	for sid, cs := range clients {
		if sid == senderSID {
			continue
		}
		targets = append(targets, target{conn: cs.conn, key: cs.key, cnt: cs.sendNonce.Add(1)})
	}
	sessMu.RUnlock()

	for _, t := range targets {
		aead, err := chacha20poly1305.New(t.key)
		if err != nil {
			continue
		}
		// Use per-client monotonic counter as nonce (unique per key, no random needed)
		nonce := make([]byte, 12)
		binary.LittleEndian.PutUint64(nonce[:8], t.cnt)
		ct := aead.Seal(nil, nonce, plaintext, nil)

		// CmdIncoming payload: [SenderSID(2 BE)][Counter(8 LE)][Ciphertext]
		payload := make([]byte, 2+8+len(ct))
		payload[0] = byte(senderSID >> 8)
		payload[1] = byte(senderSID & 0xFF)
		copy(payload[2:10], nonce[:8])
		copy(payload[10:], ct)

		writeFrame(t.conn, CmdIncoming, payload)
	}
}

func sendHistory(conn net.Conn) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		`SELECT sender, content, CAST(created_at AS INTEGER) FROM messages ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	type msgRow struct {
		sender, content string
		createdAt       int64
	}
	var msgs []msgRow
	for rows.Next() {
		var m msgRow
		if err := rows.Scan(&m.sender, &m.content, &m.createdAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("⚠️ Ошибка чтения истории: %v\n", err)
		return
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	for _, m := range msgs {
		ts := uint32(m.createdAt)
		senderBytes := []byte(m.sender)
		contentBytes := []byte(m.content)

		// CmdHistory payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][Content...]
		// Content length is implicit from frame length (saves 2 bytes vs explicit MsgLen).
		payload := make([]byte, 0, 1+len(senderBytes)+4+len(contentBytes))
		payload = append(payload, byte(len(senderBytes)))
		payload = append(payload, senderBytes...)
		payload = append(payload, byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts))
		payload = append(payload, contentBytes...)
		writeFrame(conn, CmdHistory, payload)
	}
}

// buildOnlinePacket builds [Count(1)][SID(2 BE)][NameLen(1)][Name]...
// Includes SIDs so clients can build a SID→name map for CmdIncoming resolution.
func buildOnlinePacket() []byte {
	sessMu.RLock()
	defer sessMu.RUnlock()
	pkt := []byte{byte(len(clients))}
	for sid, cs := range clients {
		b := []byte(cs.login)
		pkt = append(pkt, byte(sid>>8), byte(sid&0xFF))
		pkt = append(pkt, byte(len(b)))
		pkt = append(pkt, b...)
	}
	return pkt
}

func sendOnlineListTo(conn net.Conn) {
	writeFrame(conn, CmdOnlineList, buildOnlinePacket())
}

// broadcastOnlineAdd sends CmdOnlineAdd to all existing sessions except newSID.
// Called after the new client is already in the clients map.
func broadcastOnlineAdd(newSID uint16, name string) {
	nameBytes := []byte(name)
	payload := make([]byte, 0, 3+len(nameBytes))
	payload = append(payload, byte(newSID>>8), byte(newSID&0xFF))
	payload = append(payload, byte(len(nameBytes)))
	payload = append(payload, nameBytes...)

	sessMu.RLock()
	conns := make([]net.Conn, 0, len(clients))
	for sid, cs := range clients {
		if sid != newSID {
			conns = append(conns, cs.conn)
		}
	}
	sessMu.RUnlock()

	for _, c := range conns {
		writeFrame(c, CmdOnlineAdd, payload)
	}
}

// broadcastOnlineRemove sends CmdOnlineRemove to all remaining sessions.
// Called after the disconnected client is already removed from the clients map.
func broadcastOnlineRemove(removedSID uint16) {
	payload := []byte{byte(removedSID >> 8), byte(removedSID & 0xFF)}

	sessMu.RLock()
	conns := make([]net.Conn, 0, len(clients))
	for _, cs := range clients {
		conns = append(conns, cs.conn)
	}
	sessMu.RUnlock()

	for _, c := range conns {
		writeFrame(c, CmdOnlineRemove, payload)
	}
}

const maxMessageLen = 65536 // 64 KB

func saveMessage(sender, content string) {
	if len(content) > maxMessageLen {
		fmt.Printf("⚠️ Сообщение от %s превышает лимит (%d байт)\n", sender, len(content))
		return
	}
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
		ListenAddr:        "0.0.0.0:9999",
		DBPath:            "./messenger.db",
		HistoryLimit:      50,
		MaxFrameSize:      180,
		S2SAddr:           "0.0.0.0:9998",
		GossipEnabled:     true,
		GossipIntervalSec: 60,
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
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`)
	// Migrate old TEXT timestamps ("YYYY-MM-DD HH:MM:SS") to INTEGER unix timestamps.
	db.Exec(`UPDATE messages SET created_at = CAST(strftime('%s', created_at) AS INTEGER) WHERE typeof(created_at) = 'text'`)
}
