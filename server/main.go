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
	// Direct messages
	CmdDM         = 0x15
	CmdDMIncoming = 0x16
	CmdDMHistory  = 0x17
	// Rooms / group chats
	CmdCreateRoom      = 0x18
	CmdRoomCreated     = 0x19
	CmdRoomList        = 0x1A
	CmdJoinRoom        = 0x1B
	CmdLeaveRoom       = 0x1C
	CmdRoomMsg         = 0x1D
	CmdRoomMsgIncoming = 0x1E
	CmdRoomHistory     = 0x1F
	CmdRoomInvite      = 0x20
	CmdRoomMembers     = 0x21
	CmdRoomMemberAdd   = 0x22
	CmdRoomMemberRem   = 0x23
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
			cs := clients[mySID]
			delete(clients, mySID)
			sessMu.Unlock()
			login := ""
			if cs != nil {
				login = cs.login
			}
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
				if mySID == 0 {
					mySID = handleLogin(conn, payload, sharedKey)
				}
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
						switch innerCmd {
						case CmdE2EMsg:
							handleE2EMessage(mySID, data)
						case CmdDM:
							handleDM(mySID, data)
						case CmdRoomMsg:
							handleRoomMsg(mySID, data)
						}
					}
				}
			case CmdCreateRoom:
				if mySID != 0 {
					handleCreateRoom(mySID, payload)
				}
			case CmdJoinRoom:
				if mySID != 0 {
					handleJoinRoom(mySID, payload)
				}
			case CmdLeaveRoom:
				if mySID != 0 {
					handleLeaveRoom(mySID, payload)
				}
			case CmdRoomInvite:
				if mySID != 0 {
					handleRoomInvite(mySID, payload)
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
		// 3b. Send all known user public keys (including federated users)
		sendAllUserKeys(conn)
		// 3c. Send DM history
		sendDMHistory(conn, login)
		// 3d. Send room list + per-room members & history
		sendRoomList(conn, login)
		sendRoomDataForUser(conn, login)
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

	// Relay pubkey to all federated peers so cross-server messaging works
	go relayUserKeyToAllPeers(login, payload)

	// Broadcast the new pubkey to all other online clients so they can
	// immediately flush any queued messages waiting for this user's key.
	resp := make([]byte, 0, 1+len(login)+32)
	resp = append(resp, byte(len(login)))
	resp = append(resp, []byte(login)...)
	resp = append(resp, payload...)

	sessMu.RLock()
	conns := make([]net.Conn, 0, len(clients))
	for sid, cs := range clients {
		if sid != senderSID {
			conns = append(conns, cs.conn)
		}
	}
	sessMu.RUnlock()
	for _, c := range conns {
		writeFrame(c, CmdPublicKey, resp)
	}
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
	broadcastE2EEncrypted(senderSID, senderLogin, msgID, storedBlob, envelopes)

	// Relay to federated peers
	go relayToAllPeers(globalID, senderLogin, storedBlob, envelopes, cfg.PublicAddr)
}

// broadcastE2EEncrypted sends CmdE2EIncoming to each online client that has an envelope.
// CmdE2EIncoming payload: [SenderLen(1)][Sender(N)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
func broadcastE2EEncrypted(senderSID uint16, senderLogin string, msgID int64, storedBlob []byte, envelopes map[string][]byte) {
	type target struct {
		conn     net.Conn
		envelope []byte
	}

	sessMu.RLock()
	targets := make([]target, 0, len(clients))
	for sid, cs := range clients {
		if sid == senderSID {
			continue // skip sender (senderSID=0 for federated — no one skipped)
		}
		if env, ok := envelopes[cs.login]; ok {
			targets = append(targets, target{conn: cs.conn, envelope: env})
		}
	}
	sessMu.RUnlock()

	senderBytes := []byte(senderLogin)
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, uint32(msgID))
	for _, t := range targets {
		// payload: [SenderLen(1)][Sender(N)][MsgID(4 LE)][storedBlob][Envelope(80)]
		payload := make([]byte, 0, 1+len(senderBytes)+4+len(storedBlob)+80)
		payload = append(payload, byte(len(senderBytes)))
		payload = append(payload, senderBytes...)
		payload = append(payload, idBuf...)
		payload = append(payload, storedBlob...)
		payload = append(payload, t.envelope...)
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

// sendAllUserKeys sends CmdPublicKey frames for every known user key to the given connection.
// This allows clients to encrypt for federated users they've never seen online.
func sendAllUserKeys(conn net.Conn) {
	rows, err := db.Query("SELECT login, pubkey FROM user_keys")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var login string
		var pubkey []byte
		if err := rows.Scan(&login, &pubkey); err != nil || len(pubkey) != 32 {
			continue
		}
		resp := make([]byte, 0, 1+len(login)+32)
		resp = append(resp, byte(len(login)))
		resp = append(resp, []byte(login)...)
		resp = append(resp, pubkey...)
		writeFrame(conn, CmdPublicKey, resp)
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

	// Direct messages
	db.Exec(`CREATE TABLE IF NOT EXISTS dm_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT NOT NULL,
		recipient TEXT NOT NULL,
		encrypted_content BLOB NOT NULL,
		global_id TEXT,
		origin_server TEXT,
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`) //nolint:errcheck

	db.Exec(`CREATE TABLE IF NOT EXISTS dm_keys (
		msg_id INTEGER NOT NULL,
		login TEXT NOT NULL,
		key_envelope BLOB NOT NULL,
		PRIMARY KEY (msg_id, login)
	);`) //nolint:errcheck

	// Rooms / group chats
	db.Exec(`CREATE TABLE IF NOT EXISTS rooms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT DEFAULT '',
		is_public INTEGER DEFAULT 0,
		owner TEXT NOT NULL,
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`) //nolint:errcheck

	db.Exec(`CREATE TABLE IF NOT EXISTS room_members (
		room_id INTEGER NOT NULL,
		login TEXT NOT NULL,
		is_admin INTEGER DEFAULT 0,
		joined_at INTEGER DEFAULT (strftime('%s','now')),
		PRIMARY KEY (room_id, login)
	);`) //nolint:errcheck

	db.Exec(`CREATE TABLE IF NOT EXISTS room_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id INTEGER NOT NULL,
		sender TEXT NOT NULL,
		encrypted_content BLOB NOT NULL,
		global_id TEXT,
		origin_server TEXT,
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`) //nolint:errcheck

	db.Exec(`CREATE TABLE IF NOT EXISTS room_msg_keys (
		msg_id INTEGER NOT NULL,
		login TEXT NOT NULL,
		key_envelope BLOB NOT NULL,
		PRIMARY KEY (msg_id, login)
	);`) //nolint:errcheck
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

// ─── Direct Messages ────────────────────────────────────────────────────────

// handleDM processes a fully reassembled CmdDM payload.
// Assembled format: [RecipientLen(1)][Recipient(N)][Nonce(12)][EncContentLen(2LE)][EncContent(N)]
//
//	[EnvelopeCount(1)]  per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
func handleDM(senderSID uint16, data []byte) {
	if senderSID == 0 || len(data) < 1 {
		return
	}
	rLen := int(data[0])
	if len(data) < 1+rLen+12+2+1 {
		return
	}
	recipient := string(data[1 : 1+rLen])
	off := 1 + rLen

	nonce := data[off : off+12]
	off += 12
	encContentLen := int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2
	if len(data) < off+encContentLen+1 {
		return
	}
	encContent := data[off : off+encContentLen]
	off += encContentLen

	envelopeCount := int(data[off])
	off++
	envelopes := make(map[string][]byte)
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

	storedBlob := make([]byte, 14+len(encContent))
	copy(storedBlob[:12], nonce)
	binary.LittleEndian.PutUint16(storedBlob[12:14], uint16(len(encContent)))
	copy(storedBlob[14:], encContent)

	globalID := uuid.New().String()
	result, err := db.Exec(
		`INSERT INTO dm_messages (sender, recipient, encrypted_content, global_id, origin_server, created_at)
		 VALUES (?, ?, ?, ?, ?, strftime('%s','now'))`,
		senderLogin, recipient, storedBlob, globalID, cfg.PublicAddr,
	)
	if err != nil {
		return
	}
	msgID, _ := result.LastInsertId()
	for login, env := range envelopes {
		db.Exec( //nolint:errcheck
			"INSERT OR IGNORE INTO dm_keys (msg_id, login, key_envelope) VALUES (?, ?, ?)",
			msgID, login, env,
		)
	}
	fmt.Printf("💬 [DM] %s → %s\n", senderLogin, recipient)

	if senderConn != nil {
		writeFrame(senderConn, CmdAck, nil)
	}

	// Deliver to recipient if online
	senderBytes := []byte(senderLogin)
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, uint32(msgID))

	sessMu.RLock()
	var recipConn net.Conn
	for _, cs := range clients {
		if cs.login == recipient {
			recipConn = cs.conn
			break
		}
	}
	sessMu.RUnlock()

	if recipConn != nil {
		if env, ok := envelopes[recipient]; ok {
			payload := make([]byte, 0, 1+len(senderBytes)+4+len(storedBlob)+80)
			payload = append(payload, byte(len(senderBytes)))
			payload = append(payload, senderBytes...)
			payload = append(payload, idBuf...)
			payload = append(payload, storedBlob...)
			payload = append(payload, env...)
			writeFrame(recipConn, CmdDMIncoming, payload)
		}
	}
}

// sendDMHistory sends stored DMs for login (as sender or recipient).
// CmdDMHistory: [SenderLen(1)][Sender(N)][RecipientLen(1)][Recipient(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
func sendDMHistory(conn net.Conn, login string) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`
		SELECT m.id, m.sender, m.recipient, m.encrypted_content, CAST(m.created_at AS INTEGER), dk.key_envelope
		FROM dm_messages m
		JOIN dm_keys dk ON dk.msg_id = m.id
		WHERE dk.login = ? AND m.encrypted_content IS NOT NULL
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
			recip     string
			blob      []byte
			createdAt int64
			envelope  []byte
		)
		if err := rows.Scan(&msgID, &sender, &recip, &blob, &createdAt, &envelope); err != nil {
			continue
		}
		if len(blob) < 14 || len(envelope) != 80 {
			continue
		}
		ts := uint32(createdAt)
		sb := []byte(sender)
		rb := []byte(recip)
		idBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(idBuf, uint32(msgID))

		payload := make([]byte, 0, 1+len(sb)+1+len(rb)+4+4+len(blob)+80)
		payload = append(payload, byte(len(sb)))
		payload = append(payload, sb...)
		payload = append(payload, byte(len(rb)))
		payload = append(payload, rb...)
		payload = append(payload, byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts))
		payload = append(payload, idBuf...)
		payload = append(payload, blob...)
		payload = append(payload, envelope...)
		writeFrame(conn, CmdDMHistory, payload)
	}
}

// ─── Rooms ───────────────────────────────────────────────────────────────────

// handleCreateRoom: [NameLen(1)][Name(N)][IsPublic(1)][DescLen(2LE)][Desc(N)]
func handleCreateRoom(senderSID uint16, payload []byte) {
	if len(payload) < 4 {
		return
	}
	nLen := int(payload[0])
	if len(payload) < 1+nLen+1+2 {
		return
	}
	name := string(payload[1 : 1+nLen])
	isPublic := payload[1+nLen]
	descLen := int(binary.LittleEndian.Uint16(payload[1+nLen+1 : 1+nLen+3]))
	off := 1 + nLen + 3
	desc := ""
	if len(payload) >= off+descLen {
		desc = string(payload[off : off+descLen])
	}

	sessMu.RLock()
	login := ""
	conn := net.Conn(nil)
	if cs, ok := clients[senderSID]; ok {
		login = cs.login
		conn = cs.conn
	}
	sessMu.RUnlock()
	if login == "" {
		return
	}

	result, err := db.Exec(
		`INSERT INTO rooms (name, description, is_public, owner, created_at)
		 VALUES (?, ?, ?, ?, strftime('%s','now'))`,
		name, desc, int(isPublic), login,
	)
	if err != nil {
		fmt.Printf("❌ Ошибка создания комнаты '%s': %v\n", name, err)
		return
	}
	roomID, _ := result.LastInsertId()
	db.Exec( //nolint:errcheck
		"INSERT OR IGNORE INTO room_members (room_id, login, is_admin) VALUES (?, ?, 1)",
		roomID, login,
	)
	fmt.Printf("🏠 Комната создана: %s (ID: %d, public: %v)\n", name, roomID, isPublic != 0)

	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, uint32(roomID))
	nb := []byte(name)
	ob := []byte(login)
	resp := make([]byte, 0, 4+1+len(nb)+1+1+len(ob))
	resp = append(resp, idBuf...)
	resp = append(resp, byte(len(nb)))
	resp = append(resp, nb...)
	resp = append(resp, isPublic)
	resp = append(resp, byte(len(ob)))
	resp = append(resp, ob...)
	writeFrame(conn, CmdRoomCreated, resp)
	sendRoomMembersTo(conn, uint32(roomID))
}

// sendRoomList sends CmdRoomList with all public rooms + private rooms the user belongs to.
// Format: [RoomCount(2LE)] per room: [RoomID(4LE)][NameLen(1)][Name(N)][IsPublic(1)][OwnerLen(1)][Owner(N)][MemberCount(2LE)]
func sendRoomList(conn net.Conn, login string) {
	rows, err := db.Query(`
		SELECT DISTINCT r.id, r.name, r.is_public, r.owner,
		       (SELECT COUNT(*) FROM room_members rm2 WHERE rm2.room_id = r.id) as mc
		FROM rooms r
		LEFT JOIN room_members rm ON rm.room_id = r.id AND rm.login = ?
		WHERE r.is_public = 1 OR rm.login IS NOT NULL
		ORDER BY r.id ASC
	`, login)
	if err != nil {
		return
	}
	defer rows.Close()

	var entries [][]byte
	for rows.Next() {
		var (
			roomID  int64
			name    string
			isPubl  int
			owner   string
			memCount int
		)
		if err := rows.Scan(&roomID, &name, &isPubl, &owner, &memCount); err != nil {
			continue
		}
		idBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(idBuf, uint32(roomID))
		mcBuf := make([]byte, 2)
		binary.LittleEndian.PutUint16(mcBuf, uint16(memCount))
		nb := []byte(name)
		ob := []byte(owner)
		entry := make([]byte, 0, 4+1+len(nb)+1+1+len(ob)+2)
		entry = append(entry, idBuf...)
		entry = append(entry, byte(len(nb)))
		entry = append(entry, nb...)
		entry = append(entry, byte(isPubl))
		entry = append(entry, byte(len(ob)))
		entry = append(entry, ob...)
		entry = append(entry, mcBuf...)
		entries = append(entries, entry)
	}

	countBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(countBuf, uint16(len(entries)))
	payload := countBuf
	for _, e := range entries {
		payload = append(payload, e...)
	}
	writeFrame(conn, CmdRoomList, payload)
}

// sendRoomMembersTo sends CmdRoomMembers for a given room to conn.
// Format: [RoomID(4LE)][MemberCount(2LE)] per member: [LoginLen(1)][Login(N)][IsAdmin(1)]
func sendRoomMembersTo(conn net.Conn, roomID uint32) {
	rows, err := db.Query("SELECT login, is_admin FROM room_members WHERE room_id = ?", roomID)
	if err != nil {
		return
	}
	defer rows.Close()
	var members [][]byte
	for rows.Next() {
		var login string
		var isAdmin int
		if err := rows.Scan(&login, &isAdmin); err != nil {
			continue
		}
		lb := []byte(login)
		entry := make([]byte, 0, 1+len(lb)+1)
		entry = append(entry, byte(len(lb)))
		entry = append(entry, lb...)
		entry = append(entry, byte(isAdmin))
		members = append(members, entry)
	}
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	mcBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(mcBuf, uint16(len(members)))
	payload := append(idBuf, mcBuf...)
	for _, m := range members {
		payload = append(payload, m...)
	}
	writeFrame(conn, CmdRoomMembers, payload)
}

// sendRoomDataForUser sends members + history for every room the user belongs to.
func sendRoomDataForUser(conn net.Conn, login string) {
	rows, err := db.Query(
		"SELECT room_id FROM room_members WHERE login = ?", login,
	)
	if err != nil {
		return
	}
	var ids []uint32
	for rows.Next() {
		var id uint32
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		sendRoomMembersTo(conn, id)
		sendRoomHistory(conn, id, login)
	}
}

// sendRoomHistory sends CmdRoomHistory entries for a user in a room.
// Format: [RoomID(4LE)][SenderLen(1)][Sender(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
func sendRoomHistory(conn net.Conn, roomID uint32, login string) {
	limit := cfg.HistoryLimit
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`
		SELECT m.id, m.sender, m.encrypted_content, CAST(m.created_at AS INTEGER), rmk.key_envelope
		FROM room_messages m
		JOIN room_msg_keys rmk ON rmk.msg_id = m.id
		WHERE m.room_id = ? AND rmk.login = ? AND m.encrypted_content IS NOT NULL
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT ?
	`, roomID, login, limit)
	if err != nil {
		return
	}
	defer rows.Close()

	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)

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
		sb := []byte(sender)
		midBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(midBuf, uint32(msgID))

		payload := make([]byte, 0, 4+1+len(sb)+4+4+len(blob)+80)
		payload = append(payload, idBuf...)
		payload = append(payload, byte(len(sb)))
		payload = append(payload, sb...)
		payload = append(payload, byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts))
		payload = append(payload, midBuf...)
		payload = append(payload, blob...)
		payload = append(payload, envelope...)
		writeFrame(conn, CmdRoomHistory, payload)
	}
}

// handleJoinRoom: [RoomID(4LE)]
func handleJoinRoom(senderSID uint16, payload []byte) {
	if len(payload) < 4 {
		return
	}
	roomID := binary.LittleEndian.Uint32(payload[:4])

	sessMu.RLock()
	login := ""
	conn := net.Conn(nil)
	if cs, ok := clients[senderSID]; ok {
		login = cs.login
		conn = cs.conn
	}
	sessMu.RUnlock()
	if login == "" {
		return
	}

	var isPublic int
	var roomName string
	if err := db.QueryRow("SELECT is_public, name FROM rooms WHERE id = ?", roomID).Scan(&isPublic, &roomName); err != nil {
		return
	}

	var exists int
	if err := db.QueryRow("SELECT COUNT(*) FROM room_members WHERE room_id = ? AND login = ?", roomID, login).Scan(&exists); err != nil {
		return
	}
	if exists > 0 {
		sendRoomMembersTo(conn, roomID)
		sendRoomHistory(conn, roomID, login)
		return
	}
	if isPublic == 0 {
		return // private room — invite only
	}
	db.Exec("INSERT OR IGNORE INTO room_members (room_id, login, is_admin) VALUES (?, ?, 0)", roomID, login) //nolint:errcheck
	fmt.Printf("🚪 %s вошел в комнату %s (ID: %d)\n", login, roomName, roomID)

	// Inform joiner
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	nb := []byte(roomName)
	var owner string
	if err := db.QueryRow("SELECT owner FROM rooms WHERE id = ?", roomID).Scan(&owner); err != nil {
		return
	}
	ob := []byte(owner)
	resp := make([]byte, 0, 4+1+len(nb)+1+1+len(ob))
	resp = append(resp, idBuf...)
	resp = append(resp, byte(len(nb)))
	resp = append(resp, nb...)
	resp = append(resp, byte(isPublic))
	resp = append(resp, byte(len(ob)))
	resp = append(resp, ob...)
	writeFrame(conn, CmdRoomCreated, resp)

	sendRoomMembersTo(conn, roomID)
	sendRoomHistory(conn, roomID, login)
	broadcastRoomMemberAdd(roomID, login, senderSID)
}

// handleLeaveRoom: [RoomID(4LE)]
func handleLeaveRoom(senderSID uint16, payload []byte) {
	if len(payload) < 4 {
		return
	}
	roomID := binary.LittleEndian.Uint32(payload[:4])

	sessMu.RLock()
	login := ""
	if cs, ok := clients[senderSID]; ok {
		login = cs.login
	}
	sessMu.RUnlock()
	if login == "" {
		return
	}
	db.Exec("DELETE FROM room_members WHERE room_id = ? AND login = ?", roomID, login) //nolint:errcheck
	fmt.Printf("🚪 %s покинул комнату ID: %d\n", login, roomID)
	broadcastRoomMemberRem(roomID, login, senderSID)
}

// handleRoomMsg processes assembled CmdRoomMsg payload.
// Format: [RoomID(4LE)][Nonce(12)][EncContentLen(2LE)][EncContent(N)][EnvelopeCount(1)]
//
//	per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
func handleRoomMsg(senderSID uint16, data []byte) {
	if senderSID == 0 || len(data) < 4+12+2+1 {
		return
	}
	roomID := binary.LittleEndian.Uint32(data[:4])
	off := 4
	nonce := data[off : off+12]
	off += 12
	encContentLen := int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2
	if len(data) < off+encContentLen+1 {
		return
	}
	encContent := data[off : off+encContentLen]
	off += encContentLen
	envelopeCount := int(data[off])
	off++
	envelopes := make(map[string][]byte)
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

	var isMember int
	if err := db.QueryRow("SELECT COUNT(*) FROM room_members WHERE room_id = ? AND login = ?", roomID, senderLogin).Scan(&isMember); err != nil {
		return
	}
	if isMember == 0 {
		return
	}

	storedBlob := make([]byte, 14+len(encContent))
	copy(storedBlob[:12], nonce)
	binary.LittleEndian.PutUint16(storedBlob[12:14], uint16(len(encContent)))
	copy(storedBlob[14:], encContent)

	globalID := uuid.New().String()
	result, err := db.Exec(
		`INSERT INTO room_messages (room_id, sender, encrypted_content, global_id, origin_server, created_at)
		 VALUES (?, ?, ?, ?, ?, strftime('%s','now'))`,
		roomID, senderLogin, storedBlob, globalID, cfg.PublicAddr,
	)
	if err != nil {
		return
	}
	msgID, _ := result.LastInsertId()
	for login, env := range envelopes {
		db.Exec( //nolint:errcheck
			"INSERT OR IGNORE INTO room_msg_keys (msg_id, login, key_envelope) VALUES (?, ?, ?)",
			msgID, login, env,
		)
	}
	fmt.Printf("💬 [Room %d] %s: <зашифровано, %d байт>\n", roomID, senderLogin, len(encContent))

	if senderConn != nil {
		writeFrame(senderConn, CmdAck, nil)
	}
	broadcastRoomMsg(senderSID, senderLogin, roomID, msgID, storedBlob, envelopes)
}

// broadcastRoomMsg sends CmdRoomMsgIncoming to online room members (excluding sender).
// Format: [RoomID(4LE)][SenderLen(1)][Sender(N)][MsgID(4LE)][storedBlob][Envelope(80)]
func broadcastRoomMsg(senderSID uint16, senderLogin string, roomID uint32, msgID int64, storedBlob []byte, envelopes map[string][]byte) {
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	midBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(midBuf, uint32(msgID))
	sb := []byte(senderLogin)

	type target struct {
		conn net.Conn
		env  []byte
	}
	sessMu.RLock()
	targets := make([]target, 0, len(clients))
	for sid, cs := range clients {
		if sid == senderSID {
			continue
		}
		if env, ok := envelopes[cs.login]; ok {
			targets = append(targets, target{conn: cs.conn, env: env})
		}
	}
	sessMu.RUnlock()

	for _, t := range targets {
		payload := make([]byte, 0, 4+1+len(sb)+4+len(storedBlob)+80)
		payload = append(payload, idBuf...)
		payload = append(payload, byte(len(sb)))
		payload = append(payload, sb...)
		payload = append(payload, midBuf...)
		payload = append(payload, storedBlob...)
		payload = append(payload, t.env...)
		writeFrame(t.conn, CmdRoomMsgIncoming, payload)
	}
}

// handleRoomInvite: [RoomID(4LE)][UserLen(1)][User(N)]
func handleRoomInvite(senderSID uint16, payload []byte) {
	if len(payload) < 6 {
		return
	}
	roomID := binary.LittleEndian.Uint32(payload[:4])
	uLen := int(payload[4])
	if len(payload) < 5+uLen {
		return
	}
	targetUser := string(payload[5 : 5+uLen])

	sessMu.RLock()
	inviter := ""
	if cs, ok := clients[senderSID]; ok {
		inviter = cs.login
	}
	sessMu.RUnlock()
	if inviter == "" {
		return
	}

	var isMember int
	if err := db.QueryRow("SELECT COUNT(*) FROM room_members WHERE room_id = ? AND login = ?", roomID, inviter).Scan(&isMember); err != nil {
		return
	}
	if isMember == 0 {
		return
	}

	var roomName string
	var isPublic int
	var owner string
	if err := db.QueryRow("SELECT name, is_public, owner FROM rooms WHERE id = ?", roomID).Scan(&roomName, &isPublic, &owner); err != nil {
		return
	}

	db.Exec("INSERT OR IGNORE INTO room_members (room_id, login, is_admin) VALUES (?, ?, 0)", roomID, targetUser) //nolint:errcheck
	fmt.Printf("📨 %s пригласил %s в комнату %s (ID: %d)\n", inviter, targetUser, roomName, roomID)

	// Notify invited user if online
	sessMu.RLock()
	var targetConn net.Conn
	var targetSID uint16
	for sid, cs := range clients {
		if cs.login == targetUser {
			targetConn = cs.conn
			targetSID = sid
			break
		}
	}
	sessMu.RUnlock()

	if targetConn != nil {
		idBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(idBuf, roomID)
		nb := []byte(roomName)
		ob := []byte(owner)
		ib := []byte(inviter)
		notif := make([]byte, 0, 4+1+len(nb)+1+1+len(ob)+1+len(ib))
		notif = append(notif, idBuf...)
		notif = append(notif, byte(len(nb)))
		notif = append(notif, nb...)
		notif = append(notif, byte(isPublic))
		notif = append(notif, byte(len(ob)))
		notif = append(notif, ob...)
		notif = append(notif, byte(len(ib)))
		notif = append(notif, ib...)
		writeFrame(targetConn, CmdRoomCreated, notif)
		sendRoomMembersTo(targetConn, roomID)
		sendRoomHistory(targetConn, roomID, targetUser)
	}
	broadcastRoomMemberAdd(roomID, targetUser, targetSID)
}

// broadcastRoomMemberAdd sends CmdRoomMemberAdd to all online room members.
// Format: [RoomID(4LE)][LoginLen(1)][Login(N)]
func broadcastRoomMemberAdd(roomID uint32, login string, exceptSID uint16) {
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	lb := []byte(login)
	payload := make([]byte, 0, 4+1+len(lb))
	payload = append(payload, idBuf...)
	payload = append(payload, byte(len(lb)))
	payload = append(payload, lb...)

	rows, err := db.Query("SELECT login FROM room_members WHERE room_id = ?", roomID)
	if err != nil {
		return
	}
	memberSet := make(map[string]bool)
	for rows.Next() {
		var ml string
		if rows.Scan(&ml) == nil {
			memberSet[ml] = true
		}
	}
	rows.Close()

	sessMu.RLock()
	conns := make([]net.Conn, 0)
	for sid, cs := range clients {
		if sid != exceptSID && memberSet[cs.login] {
			conns = append(conns, cs.conn)
		}
	}
	sessMu.RUnlock()
	for _, c := range conns {
		writeFrame(c, CmdRoomMemberAdd, payload)
	}
}

// broadcastRoomMemberRem sends CmdRoomMemberRem to all online room members (including the leaver).
// Format: [RoomID(4LE)][LoginLen(1)][Login(N)]
func broadcastRoomMemberRem(roomID uint32, login string, leaverSID uint16) {
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	lb := []byte(login)
	payload := make([]byte, 0, 4+1+len(lb))
	payload = append(payload, idBuf...)
	payload = append(payload, byte(len(lb)))
	payload = append(payload, lb...)

	rows, err := db.Query("SELECT login FROM room_members WHERE room_id = ?", roomID)
	if err != nil {
		return
	}
	memberSet := make(map[string]bool)
	for rows.Next() {
		var ml string
		if rows.Scan(&ml) == nil {
			memberSet[ml] = true
		}
	}
	rows.Close()

	sessMu.RLock()
	conns := make([]net.Conn, 0)
	for _, cs := range clients {
		if memberSet[cs.login] {
			conns = append(conns, cs.conn)
		}
	}
	// Also notify the leaver themselves
	if cs, ok := clients[leaverSID]; ok {
		conns = append(conns, cs.conn)
	}
	sessMu.RUnlock()
	for _, c := range conns {
		writeFrame(c, CmdRoomMemberRem, payload)
	}
}
