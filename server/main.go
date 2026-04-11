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

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Config struct {
	ListenAddr         string   `json:"listen_addr"`
	DBPath             string   `json:"db_path"`
	HistoryLimit       int      `json:"history_limit"`
	MaxFrameSize       int      `json:"max_frame_size"`
	S2SAddr            string   `json:"s2s_addr"`            // server-to-server gossip listen addr
	PublicAddr         string   `json:"public_addr"`         // client-facing external addr
	GossipEnabled      bool     `json:"gossip_enabled"`      // default true
	GossipIntervalSec  int      `json:"gossip_interval_sec"` // default 60
	InitialPeers       []string `json:"peers"`               // s2s addrs of seed servers
	S2SSecret          string   `json:"s2s_secret"`          // HMAC secret for S2S relay auth
	FederationSyncDays int      `json:"federation_sync_days"` // how many days back to sync (default 7)
}

const (
	CmdRegister     = 0x01
	CmdLogin        = 0x02
	// 0x03-0x05 retired (plaintext CmdMsg/CmdIncoming/CmdHistory removed)
	CmdAck          = 0x06
	CmdHistoryEnd   = 0x07
	CmdLoginOK      = 0x08
	CmdLoginFail    = 0x09
	CmdOnlineList   = 0x0A
	CmdOnlineAdd    = 0x0B
	CmdOnlineRemove = 0x0C
	CmdFragment     = 0x0D
	CmdServerList   = 0x0E
	// E2E commands
	CmdSetPublicKey     = 0x0F
	CmdPublicKey        = 0x10
	CmdPublicKeyRequest = 0x11
	CmdE2EMsg           = 0x12
	CmdE2EIncoming      = 0x13
	CmdE2EHistory       = 0x14
)

// clientState holds all per-session state.
type clientState struct {
	conn  net.Conn
	key   []byte // session ECDH key (kept; used for login frame protection in future)
	login string
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
func writeFrame(conn net.Conn, cmd byte, payload []byte) {
	total := 1 + len(payload)
	frame := make([]byte, 2+total)
	binary.LittleEndian.PutUint16(frame[0:2], uint16(total))
	frame[2] = cmd
	copy(frame[3:], payload)
	conn.Write(frame) //nolint:errcheck
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
	const maxPending = 65536
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
			case CmdSetPublicKey:
				if mySID != 0 {
					handleSetPublicKey(mySID, payload)
				}
			case CmdPublicKeyRequest:
				if mySID != 0 {
					handlePublicKeyRequest(conn, payload)
				}
			case CmdFragment:
				if mySID != 0 {
					if innerCmd, data := handleFragment(mySID, payload); data != nil {
						if innerCmd == CmdE2EMsg {
							handleE2EMessage(mySID, data)
						}
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
			sidCounter = 1
		}
		sid := sidCounter
		startSID := sid
		for clients[sid] != nil {
			sidCounter++
			if sidCounter == 0 {
				sidCounter = 1
			}
			sid = sidCounter
			if sid == startSID {
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
		// 2. Send full online list so client builds its sidNames map
		sendOnlineListTo(conn)
		// 3. Send E2E message history for this user
		sendE2EHistory(conn, login)
		writeFrame(conn, CmdHistoryEnd, nil)
		// 4. Send list of known federated servers
		if payload := buildServerListPayload(); payload != nil {
			writeFrame(conn, CmdServerList, payload)
		}
		// 5. Notify all existing clients about the new user
		broadcastOnlineAdd(sid, login)

		fmt.Printf("🔑 Вошел: %s (SID: %d)\n", login, sid)
		return sid
	}
	writeFrame(conn, CmdLoginFail, nil)
	return 0
}

// handleSetPublicKey stores the client's long-term X25519 public key.
// Payload: [pubkey(32)]
func handleSetPublicKey(senderSID uint16, payload []byte) {
	if len(payload) != 32 {
		return
	}
	sessMu.RLock()
	login := ""
	if cs, ok := clients[senderSID]; ok {
		login = cs.login
	}
	sessMu.RUnlock()
	if login == "" {
		return
	}
	db.Exec( //nolint:errcheck
		"INSERT OR REPLACE INTO user_keys (login, pubkey, updated_at) VALUES (?, ?, strftime('%s','now'))",
		login, payload,
	)
	fmt.Printf("🔑 [E2E] Публичный ключ получен от %s\n", login)
}

// handlePublicKeyRequest responds with the public key of a requested user.
// Payload: [UsernameLen(1)][Username(N)]
func handlePublicKeyRequest(conn net.Conn, payload []byte) {
	if len(payload) < 1 {
		return
	}
	lLen := int(payload[0])
	if len(payload) < 1+lLen {
		return
	}
	username := string(payload[1 : 1+lLen])

	var pubkey []byte
	err := db.QueryRow("SELECT pubkey FROM user_keys WHERE login = ?", username).Scan(&pubkey)
	if err != nil || len(pubkey) != 32 {
		return
	}

	// CmdPublicKey: [UsernameLen(1)][Username(N)][pubkey(32)]
	resp := make([]byte, 0, 1+len(username)+32)
	resp = append(resp, byte(len(username)))
	resp = append(resp, []byte(username)...)
	resp = append(resp, pubkey...)
	writeFrame(conn, CmdPublicKey, resp)
}

// handleE2EMessage processes a fully reassembled CmdE2EMsg payload.
// Data format (after stripping the 0x12 cmd tag by handleFragment):
//
//	[Nonce(12)][EncContentLen(2 LE)][EncContent(N)][EnvelopeCount(1)]
//	per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
//
// Envelope = ephemeral_pub(32) + ChaCha20-Poly1305(ECDH(eph,recip),msgKey)(48)
func handleE2EMessage(senderSID uint16, data []byte) {
	if senderSID == 0 || len(data) < 12+2+1 {
		return
	}

	nonce := data[:12]
	encContentLen := int(binary.LittleEndian.Uint16(data[12:14]))
	if len(data) < 14+encContentLen+1 {
		return
	}
	encContent := data[14 : 14+encContentLen]

	off := 14 + encContentLen
	envelopeCount := int(data[off])
	off++

	envelopes := make(map[string][]byte) // login → 80-byte envelope
	for i := 0; i < envelopeCount; i++ {
		if off >= len(data) {
			break
		}
		lLen := int(data[off])
		off++
		if off+lLen+80 > len(data) {
			break
		}
		login := string(data[off : off+lLen])
		off += lLen
		env := make([]byte, 80)
		copy(env, data[off:off+80])
		off += 80
		envelopes[login] = env
	}

	sessMu.RLock()
	senderLogin := ""
	senderConn := net.Conn(nil)
	if cs, ok := clients[senderSID]; ok {
		senderLogin = cs.login
		senderConn = cs.conn
	}
	sessMu.RUnlock()
	if senderLogin == "" {
		return
	}

	// storedBlob = nonce(12) + encContentLen(2 LE) + encContent
	// All clients parse storedBlob[12:14] as the ciphertext length.
	storedBlob := make([]byte, 12+2+len(encContent))
	copy(storedBlob[:12], nonce)
	binary.LittleEndian.PutUint16(storedBlob[12:14], uint16(len(encContent)))
	copy(storedBlob[14:], encContent)

	globalID := uuid.New().String()

	// Persist message
	result, err := db.Exec(
		`INSERT INTO messages (sender, content, encrypted_content, global_id, origin_server, created_at)
		 VALUES (?, '', ?, ?, ?, strftime('%s','now'))`,
		senderLogin, storedBlob, globalID, cfg.PublicAddr,
	)
	if err != nil {
		fmt.Printf("❌ Ошибка сохранения E2E сообщения: %v\n", err)
		return
	}
	msgID, _ := result.LastInsertId()

	// Persist per-user envelopes
	for login, env := range envelopes {
		db.Exec( //nolint:errcheck
			"INSERT OR IGNORE INTO msg_keys (msg_id, login, key_envelope) VALUES (?, ?, ?)",
			msgID, login, env,
		)
	}

	fmt.Printf("📩 [E2E] %s: <зашифровано, %d байт>\n", senderLogin, len(encContent))

	// Ack to sender
	if senderConn != nil {
		writeFrame(senderConn, CmdAck, nil)
	}

	// Broadcast to all online clients that have an envelope
	broadcastE2EEncrypted(senderSID, msgID, storedBlob, envelopes)

	// Relay to federated peers
	go relayToAllPeers(globalID, senderLogin, storedBlob, envelopes, cfg.PublicAddr)
}

// broadcastE2EEncrypted sends CmdE2EIncoming to each online client that has an envelope.
// CmdE2EIncoming payload: [SenderSID(2 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
func broadcastE2EEncrypted(senderSID uint16, msgID int64, storedBlob []byte, envelopes map[string][]byte) {
	type target struct {
		conn     net.Conn
		login    string
		envelope []byte
	}

	sessMu.RLock()
	targets := make([]target, 0, len(clients))
	for sid, cs := range clients {
		if sid == senderSID {
			continue
		}
		if env, ok := envelopes[cs.login]; ok {
			targets = append(targets, target{conn: cs.conn, login: cs.login, envelope: env})
		}
	}
	sessMu.RUnlock()

	for _, t := range targets {
		// payload: [SenderSID(2 BE)][MsgID(4 LE)][storedBlob][Envelope(80)]
		payload := make([]byte, 2+4+len(storedBlob)+80)
		payload[0] = byte(senderSID >> 8)
		payload[1] = byte(senderSID & 0xFF)
		binary.LittleEndian.PutUint32(payload[2:6], uint32(msgID))
		copy(payload[6:], storedBlob)
		copy(payload[6+len(storedBlob):], t.envelope)
		writeFrame(t.conn, CmdE2EIncoming, payload)
	}
}

// sendE2EHistory sends stored E2E messages for a given login.
// CmdE2EHistory payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
func sendE2EHistory(conn net.Conn, login string) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`
		SELECT m.id, m.sender, m.encrypted_content, CAST(m.created_at AS INTEGER), mk.key_envelope
		FROM messages m
		JOIN msg_keys mk ON mk.msg_id = m.id
		WHERE mk.login = ? AND m.encrypted_content IS NOT NULL
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT ?
	`, login, limit)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			msgID     int64
			sender    string
			blob      []byte
			createdAt int64
			envelope  []byte
		)
		if err := rows.Scan(&msgID, &sender, &blob, &createdAt, &envelope); err != nil {
			continue
		}
		if len(blob) < 14 || len(envelope) != 80 {
			continue
		}

		ts := uint32(createdAt)
		senderBytes := []byte(sender)

		payload := make([]byte, 0, 1+len(senderBytes)+4+4+len(blob)+80)
		payload = append(payload, byte(len(senderBytes)))
		payload = append(payload, senderBytes...)
		payload = append(payload, byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts))
		// MsgID (4 LE)
		idBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(idBuf, uint32(msgID))
		payload = append(payload, idBuf...)
		payload = append(payload, blob...)
		payload = append(payload, envelope...)
		writeFrame(conn, CmdE2EHistory, payload)
	}
}

// handleFragment stores a fragment and returns (cmd, data) when all arrive.
// The assembled bytes start with the cmd tag (byte 0), stripped before returning.
// Returns (0, nil) if more fragments needed or on error.
func handleFragment(senderSID uint16, data []byte) (byte, []byte) {
	if len(data) < 4 {
		return 0, nil
	}
	msgID := data[0]
	fragIdx := data[1]
	fragCount := data[2]
	chunk := data[3:]

	if fragCount == 0 || fragIdx >= fragCount {
		return 0, nil
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
		return 0, nil
	}

	// All fragments received — reassemble in order
	var assembled []byte
	for i := uint8(0); i < fb.total; i++ {
		assembled = append(assembled, fb.frags[i]...)
	}
	delete(fragMap, key)

	if len(assembled) < 1 {
		return 0, nil
	}
	// assembled[0] = command tag; assembled[1:] = actual payload
	return assembled[0], assembled[1:]
}

// buildOnlinePacket builds [Count(1)][SID(2 BE)][NameLen(1)][Name]...
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
		ListenAddr:         "0.0.0.0:9999",
		DBPath:             "./messenger.db",
		HistoryLimit:       50,
		MaxFrameSize:       180,
		S2SAddr:            "0.0.0.0:9998",
		GossipEnabled:      true,
		GossipIntervalSec:  60,
		FederationSyncDays: 7,
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
		return
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&cfg) //nolint:errcheck
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
	);`) //nolint:errcheck

	db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`) //nolint:errcheck

	// E2E: per-user long-term public keys
	db.Exec(`CREATE TABLE IF NOT EXISTS user_keys (
		login      TEXT PRIMARY KEY,
		pubkey     BLOB NOT NULL,
		updated_at INTEGER DEFAULT (strftime('%s','now'))
	);`) //nolint:errcheck

	// E2E: per-message per-user key envelopes
	db.Exec(`CREATE TABLE IF NOT EXISTS msg_keys (
		msg_id       INTEGER NOT NULL,
		login        TEXT NOT NULL,
		key_envelope BLOB NOT NULL,
		PRIMARY KEY (msg_id, login)
	);`) //nolint:errcheck

	// Migrate: add columns if missing (SQLite has no IF NOT EXISTS on ALTER TABLE)
	migrateColumn("messages", "encrypted_content", "BLOB")
	migrateColumn("messages", "global_id", "TEXT")
	migrateColumn("messages", "origin_server", "TEXT")

	// Index for deduplication in federation
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_global_id
		ON messages(global_id) WHERE global_id IS NOT NULL AND global_id != ''`) //nolint:errcheck

	// Migrate old TEXT timestamps to INTEGER
	db.Exec(`UPDATE messages SET created_at = CAST(strftime('%s', created_at) AS INTEGER)
		WHERE typeof(created_at) = 'text'`) //nolint:errcheck
}

// migrateColumn adds a column to a table if it does not already exist.
func migrateColumn(table, column, colType string) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype, notnull, dfltVal, pk string
		rows.Scan(&cid, &name, &ctype, &notnull, &dfltVal, &pk) //nolint:errcheck
		if name == column {
			return
		}
	}
	db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)) //nolint:errcheck
}
