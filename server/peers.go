package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// PeerInfo holds the known addresses for a federated server peer.
type PeerInfo struct {
	ClientAddr string // "host:9999" — advertised to clients via CmdServerList
	S2SAddr    string // "host:9998" — used for outgoing gossip dials
	LastSeen   int64  // unix timestamp of last successful gossip exchange
	LastSync   int64  // unix timestamp of last successful message sync
}

// PeerStore is a thread-safe registry of known peer servers.
// Keyed by ClientAddr; seeds (known only by S2SAddr) are kept in a separate set.
type PeerStore struct {
	mu    sync.RWMutex
	peers map[string]*PeerInfo // key = ClientAddr
	seeds map[string]bool      // key = S2SAddr (from config, ClientAddr not yet known)
}

var peerStore = &PeerStore{
	peers: make(map[string]*PeerInfo),
	seeds: make(map[string]bool),
}

// AddSeed registers a peer by its S2S address only (before first gossip exchange).
func (ps *PeerStore) AddSeed(s2sAddr string) {
	if s2sAddr == "" {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, p := range ps.peers {
		if p.S2SAddr == s2sAddr {
			return
		}
	}
	ps.seeds[s2sAddr] = true
}

// Add registers or updates a full peer (both ClientAddr and S2SAddr known).
func (ps *PeerStore) Add(clientAddr, s2sAddr string) {
	if clientAddr == "" {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if s2sAddr != "" {
		delete(ps.seeds, s2sAddr)
	}
	if existing, ok := ps.peers[clientAddr]; ok {
		existing.LastSeen = time.Now().Unix()
		if s2sAddr != "" {
			existing.S2SAddr = s2sAddr
		}
	} else {
		ps.peers[clientAddr] = &PeerInfo{
			ClientAddr: clientAddr,
			S2SAddr:    s2sAddr,
			LastSeen:   time.Now().Unix(),
		}
	}
}

// ClientAddrs returns all known client-facing addresses (for CmdServerList).
func (ps *PeerStore) ClientAddrs() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	addrs := make([]string, 0, len(ps.peers))
	for addr := range ps.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

// S2SAddrs returns all addresses available for gossip dialing.
func (ps *PeerStore) S2SAddrs() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	seen := make(map[string]bool)
	var addrs []string
	for _, p := range ps.peers {
		if p.S2SAddr != "" && !seen[p.S2SAddr] {
			seen[p.S2SAddr] = true
			addrs = append(addrs, p.S2SAddr)
		}
	}
	for s := range ps.seeds {
		if !seen[s] {
			seen[s] = true
			addrs = append(addrs, s)
		}
	}
	return addrs
}

// Snapshot returns a copy of all full peers (not seeds).
func (ps *PeerStore) Snapshot() []PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]PeerInfo, 0, len(ps.peers))
	for _, p := range ps.peers {
		out = append(out, *p)
	}
	return out
}

// Count returns the number of known full peers.
func (ps *PeerStore) Count() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

// GetLastSync returns the LastSync timestamp for the peer with the given S2S address.
func (ps *PeerStore) GetLastSync(s2sAddr string) int64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, p := range ps.peers {
		if p.S2SAddr == s2sAddr {
			return p.LastSync
		}
	}
	return 0
}

// SetLastSync updates the LastSync timestamp for the peer with the given S2S address.
func (ps *PeerStore) SetLastSync(s2sAddr string, ts int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, p := range ps.peers {
		if p.S2SAddr == s2sAddr {
			p.LastSync = ts
			return
		}
	}
}

// recentIDs is a fixed-size ring buffer used to deduplicate relay messages.
var (
	recentIDsMu   sync.Mutex
	recentIDs     [1024]string
	recentIDsHead int
)

// sawGlobalID returns true if id was seen recently; otherwise records it and returns false.
func sawGlobalID(id string) bool {
	recentIDsMu.Lock()
	defer recentIDsMu.Unlock()
	for _, rid := range recentIDs {
		if rid == id {
			return true
		}
	}
	recentIDs[recentIDsHead] = id
	recentIDsHead = (recentIDsHead + 1) % len(recentIDs)
	return false
}

type gossipPeer struct {
	Client string `json:"client"`
	S2S    string `json:"s2s"`
}

// s2sMsg is the unified server-to-server message type (newline-delimited JSON).
type s2sMsg struct {
	Type string `json:"type"`
	// gossip
	Client string       `json:"client,omitempty"`
	S2S    string       `json:"s2s,omitempty"`
	Peers  []gossipPeer `json:"peers,omitempty"`
	// relay_msg
	GlobalID     string            `json:"global_id,omitempty"`
	Sender       string            `json:"sender,omitempty"`
	StoredBlob   []byte            `json:"stored_blob,omitempty"` // base64 in JSON
	OriginServer string            `json:"origin_server,omitempty"`
	Timestamp    int64             `json:"timestamp,omitempty"`
	Envelopes    map[string][]byte `json:"envelopes,omitempty"` // login → 80 bytes
	Auth         string            `json:"auth,omitempty"`      // HMAC-SHA256
	// sync_request
	Since int64 `json:"since,omitempty"`
	// sync_response
	Messages []relayMsgPayload `json:"messages,omitempty"`
}

// relayMsgPayload is a single message inside a sync_response batch.
type relayMsgPayload struct {
	GlobalID     string            `json:"global_id"`
	Sender       string            `json:"sender"`
	StoredBlob   []byte            `json:"stored_blob"`
	OriginServer string            `json:"origin_server"`
	Timestamp    int64             `json:"timestamp"`
	Envelopes    map[string][]byte `json:"envelopes"`
}

// effectiveS2SPublicAddr derives the public S2S address from cfg.PublicAddr + cfg.S2SAddr port.
func effectiveS2SPublicAddr() string {
	if cfg.PublicAddr == "" || cfg.S2SAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(cfg.PublicAddr)
	if err != nil {
		return ""
	}
	_, port, err := net.SplitHostPort(cfg.S2SAddr)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(host, port)
}

func buildGossipMsg() s2sMsg {
	myS2S := effectiveS2SPublicAddr()
	peers := peerStore.Snapshot()
	gpeers := make([]gossipPeer, 0, len(peers))
	for _, p := range peers {
		if p.ClientAddr == cfg.PublicAddr {
			continue
		}
		gpeers = append(gpeers, gossipPeer{Client: p.ClientAddr, S2S: p.S2SAddr})
	}
	return s2sMsg{Type: "gossip", Client: cfg.PublicAddr, S2S: myS2S, Peers: gpeers}
}

// buildServerListPayload returns a CmdServerList payload:
// [Count(1)][AddrLen(1)][Addr(N)]...
// Prepends cfg.PublicAddr (self) if set. Truncates to fit within MaxFrameSize.
func buildServerListPayload() []byte {
	addrs := peerStore.ClientAddrs()
	if cfg.PublicAddr != "" {
		addrs = append([]string{cfg.PublicAddr}, addrs...)
	}
	if len(addrs) == 0 {
		return nil
	}
	maxPayload := cfg.MaxFrameSize - 3
	if maxPayload < 2 {
		return nil
	}
	payload := make([]byte, 0, maxPayload)
	payload = append(payload, 0)
	count := 0
	for _, addr := range addrs {
		b := []byte(addr)
		if len(b) == 0 || len(b) > 255 {
			continue
		}
		if len(payload)+1+len(b) > maxPayload {
			break
		}
		payload = append(payload, byte(len(b)))
		payload = append(payload, b...)
		count++
		if count == 255 {
			break
		}
	}
	payload[0] = byte(count)
	if count == 0 {
		return nil
	}
	return payload
}

func startS2SListener() {
	if cfg.S2SAddr == "" {
		return
	}
	ln, err := net.Listen("tcp", cfg.S2SAddr)
	if err != nil {
		fmt.Printf("⚠️  S2S listener error on %s: %v\n", cfg.S2SAddr, err)
		return
	}
	fmt.Printf("🔗 S2S listener started on %s\n", cfg.S2SAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleS2SConnection(conn)
	}
}

func handleS2SConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second)) //nolint:errcheck

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 65536), 65536)

	// Track the remote peer's advertised S2S address (learned from its gossip message)
	// to avoid re-forwarding relay messages back to the source.
	var sourcePeerS2SAddr string

	for scanner.Scan() {
		conn.SetDeadline(time.Now().Add(30 * time.Second)) //nolint:errcheck
		var msg s2sMsg
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "gossip":
			if sourcePeerS2SAddr == "" { // handle gossip only once per connection
				sourcePeerS2SAddr = msg.S2S
				handleGossipMsg(conn, msg)
				savePeers()
				fmt.Printf("🔗 S2S gossip from %s — %d peer(s) known\n", msg.Client, peerStore.Count())
			}
		case "relay_msg":
			handleRelayMsg(msg, sourcePeerS2SAddr)
		case "sync_request":
			handleSyncRequest(conn, msg)
		}
	}
}

// handleGossipMsg processes an incoming gossip message and sends our own gossip reply.
func handleGossipMsg(conn net.Conn, msg s2sMsg) {
	if msg.Client != "" && msg.Client != cfg.PublicAddr {
		peerStore.Add(msg.Client, msg.S2S)
	}
	for _, p := range msg.Peers {
		if p.Client != "" && p.Client != cfg.PublicAddr {
			peerStore.Add(p.Client, p.S2S)
		}
	}
	resp := buildGossipMsg()
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	conn.Write(append(data, '\n')) //nolint:errcheck
}

// computeRelayAuth computes HMAC-SHA256(s2s_secret, globalID+timestamp).
func computeRelayAuth(globalID string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(cfg.S2SSecret))
	mac.Write([]byte(globalID))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// handleRelayMsg stores an incoming relay message and forwards it to other peers.
func handleRelayMsg(msg s2sMsg, sourcePeerS2SAddr string) {
	if msg.GlobalID == "" || len(msg.StoredBlob) == 0 {
		return
	}
	// Verify HMAC if a shared secret is configured
	if cfg.S2SSecret != "" {
		expected := computeRelayAuth(msg.GlobalID, msg.Timestamp)
		if !hmac.Equal([]byte(msg.Auth), []byte(expected)) {
			fmt.Printf("⚠️  S2S relay auth failed (global_id=%s)\n", msg.GlobalID)
			return
		}
	}
	// In-memory dedup (ring buffer of last 1024 IDs)
	if sawGlobalID(msg.GlobalID) {
		return
	}
	// Persist (UNIQUE INDEX on global_id prevents duplicate rows)
	result, err := db.Exec(
		`INSERT OR IGNORE INTO messages (sender, content, encrypted_content, global_id, origin_server, created_at)
		 VALUES (?, '', ?, ?, ?, ?)`,
		msg.Sender, msg.StoredBlob, msg.GlobalID, msg.OriginServer, msg.Timestamp,
	)
	if err != nil {
		fmt.Printf("❌ Ошибка сохранения relay сообщения: %v\n", err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return // Row already existed in DB
	}
	msgID, _ := result.LastInsertId()

	for login, env := range msg.Envelopes {
		db.Exec( //nolint:errcheck
			"INSERT OR IGNORE INTO msg_keys (msg_id, login, key_envelope) VALUES (?, ?, ?)",
			msgID, login, env,
		)
	}

	// Deliver to locally-connected clients
	broadcastE2EEncrypted(0, msgID, msg.StoredBlob, msg.Envelopes)

	shortID := msg.GlobalID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	fmt.Printf("🔗 Relay recv: %s… from %s\n", shortID, msg.Sender)

	// Forward to all other known peers (not the source)
	for _, addr := range peerStore.S2SAddrs() {
		if addr != sourcePeerS2SAddr {
			go dialRelay(addr, msg)
		}
	}
}

// handleSyncRequest responds with messages stored after msg.Since.
func handleSyncRequest(conn net.Conn, msg s2sMsg) {
	syncDays := cfg.FederationSyncDays
	if syncDays <= 0 {
		syncDays = 7
	}
	minTime := msg.Since
	if minTime == 0 {
		minTime = time.Now().Unix() - int64(syncDays)*86400
	}

	rows, err := db.Query(`
		SELECT m.id, m.sender, m.encrypted_content, m.global_id, m.origin_server, CAST(m.created_at AS INTEGER)
		FROM messages m
		WHERE m.created_at > ? AND m.global_id IS NOT NULL AND m.global_id != ''
		ORDER BY m.id ASC
		LIMIT 500
	`, minTime)
	if err != nil {
		return
	}
	defer rows.Close()

	batch := make([]relayMsgPayload, 0, 50)
	for rows.Next() {
		var (
			msgID        int64
			sender       string
			storedBlob   []byte
			globalID     string
			originServer string
			ts           int64
		)
		if err := rows.Scan(&msgID, &sender, &storedBlob, &globalID, &originServer, &ts); err != nil {
			continue
		}
		if len(storedBlob) == 0 {
			continue
		}

		envRows, err := db.Query("SELECT login, key_envelope FROM msg_keys WHERE msg_id = ?", msgID)
		if err != nil {
			continue
		}
		envelopes := make(map[string][]byte)
		for envRows.Next() {
			var login string
			var env []byte
			envRows.Scan(&login, &env) //nolint:errcheck
			envelopes[login] = env
		}
		envRows.Close()

		batch = append(batch, relayMsgPayload{
			GlobalID:     globalID,
			Sender:       sender,
			StoredBlob:   storedBlob,
			OriginServer: originServer,
			Timestamp:    ts,
			Envelopes:    envelopes,
		})
		if len(batch) >= 50 {
			sendSyncResponse(conn, batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		sendSyncResponse(conn, batch)
	}
	// Send empty batch to signal end of sync (so the reader doesn't wait for a timeout)
	sendSyncResponse(conn, []relayMsgPayload{})
}

func sendSyncResponse(conn net.Conn, batch []relayMsgPayload) {
	data, err := json.Marshal(s2sMsg{Type: "sync_response", Messages: batch})
	if err != nil {
		return
	}
	conn.Write(append(data, '\n')) //nolint:errcheck
}

// processSyncedMessage stores a message received during a sync_response.
func processSyncedMessage(m relayMsgPayload) {
	if m.GlobalID == "" || len(m.StoredBlob) == 0 {
		return
	}
	if sawGlobalID(m.GlobalID) {
		return
	}
	result, err := db.Exec(
		`INSERT OR IGNORE INTO messages (sender, content, encrypted_content, global_id, origin_server, created_at)
		 VALUES (?, '', ?, ?, ?, ?)`,
		m.Sender, m.StoredBlob, m.GlobalID, m.OriginServer, m.Timestamp,
	)
	if err != nil {
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return
	}
	msgID, _ := result.LastInsertId()
	for login, env := range m.Envelopes {
		db.Exec( //nolint:errcheck
			"INSERT OR IGNORE INTO msg_keys (msg_id, login, key_envelope) VALUES (?, ?, ?)",
			msgID, login, env,
		)
	}
	broadcastE2EEncrypted(0, msgID, m.StoredBlob, m.Envelopes)
}

// dialRelay connects to a peer, performs gossip handshake, then sends a relay message.
func dialRelay(s2sAddr string, msg s2sMsg) {
	conn, err := net.DialTimeout("tcp", s2sAddr, 5*time.Second)
	if err != nil {
		fmt.Printf("⚠️  Relay: can't reach %s: %v\n", s2sAddr, err)
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck

	reader := bufio.NewReaderSize(conn, 65536)

	// Gossip handshake establishes our identity to the remote peer
	gossipData, err := json.Marshal(buildGossipMsg())
	if err != nil {
		return
	}
	if _, err := conn.Write(append(gossipData, '\n')); err != nil {
		return
	}
	// Consume gossip response (peer learns about us; we don't need to process it here)
	if _, err := reader.ReadBytes('\n'); err != nil {
		return
	}
	// Send the relay message
	relayData, err := json.Marshal(msg)
	if err != nil {
		return
	}
	conn.Write(append(relayData, '\n')) //nolint:errcheck
}

func startGossipLoop() {
	if !cfg.GossipEnabled {
		fmt.Println("ℹ️  Gossip disabled")
		return
	}
	interval := time.Duration(cfg.GossipIntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	for _, addr := range peerStore.S2SAddrs() {
		go dialGossip(addr)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		for _, addr := range peerStore.S2SAddrs() {
			go dialGossip(addr)
		}
	}
}

// dialGossip connects to a peer, exchanges gossip, then requests a message sync.
func dialGossip(s2sAddr string) {
	conn, err := net.DialTimeout("tcp", s2sAddr, 5*time.Second)
	if err != nil {
		fmt.Printf("⚠️  Gossip: can't reach %s: %v\n", s2sAddr, err)
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck

	reader := bufio.NewReaderSize(conn, 65536)

	// --- Gossip exchange ---
	data, err := json.Marshal(buildGossipMsg())
	if err != nil {
		return
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return
	}
	line, err := reader.ReadBytes('\n')
	if err != nil || len(line) < 2 {
		return
	}
	var resp s2sMsg
	if err := json.Unmarshal(line[:len(line)-1], &resp); err != nil || resp.Type != "gossip" {
		return
	}
	beforeCount := peerStore.Count()
	if resp.Client != "" && resp.Client != cfg.PublicAddr {
		peerStore.Add(resp.Client, resp.S2S)
	}
	for _, p := range resp.Peers {
		if p.Client != "" && p.Client != cfg.PublicAddr {
			peerStore.Add(p.Client, p.S2S)
		}
	}
	afterCount := peerStore.Count()
	savePeers()
	fmt.Printf("🔗 Gossip %s — %d peer(s) known\n", s2sAddr, afterCount)
	if afterCount > beforeCount {
		for _, addr := range peerStore.S2SAddrs() {
			if addr != s2sAddr {
				go dialGossip(addr)
			}
		}
	}

	// --- Sync request ---
	lastSync := peerStore.GetLastSync(s2sAddr)
	syncReqData, err := json.Marshal(s2sMsg{Type: "sync_request", Since: lastSync})
	if err != nil {
		return
	}
	conn.SetDeadline(time.Now().Add(60 * time.Second)) //nolint:errcheck
	if _, err := conn.Write(append(syncReqData, '\n')); err != nil {
		return
	}

	// Read sync_response batches until the last batch (< 50 items) or error
	syncCount := 0
	for {
		conn.SetDeadline(time.Now().Add(30 * time.Second)) //nolint:errcheck
		line, err := reader.ReadBytes('\n')
		if err != nil || len(line) < 2 {
			break
		}
		var syncResp s2sMsg
		if err := json.Unmarshal(line[:len(line)-1], &syncResp); err != nil {
			break
		}
		if syncResp.Type != "sync_response" {
			break
		}
		for _, m := range syncResp.Messages {
			processSyncedMessage(m)
			syncCount++
		}
		if len(syncResp.Messages) < 50 {
			break // last batch
		}
	}
	if syncCount > 0 {
		fmt.Printf("🔗 Synced %d message(s) from %s\n", syncCount, s2sAddr)
	}
	peerStore.SetLastSync(s2sAddr, time.Now().Unix())
}

// relayToAllPeers forwards an E2E message to all known federated S2S peers.
func relayToAllPeers(globalID string, sender string, storedBlob []byte, envelopes map[string][]byte, originServer string) {
	addrs := peerStore.S2SAddrs()
	if len(addrs) == 0 {
		return
	}
	ts := time.Now().Unix()
	msg := s2sMsg{
		Type:         "relay_msg",
		GlobalID:     globalID,
		Sender:       sender,
		StoredBlob:   storedBlob,
		OriginServer: originServer,
		Timestamp:    ts,
		Envelopes:    envelopes,
	}
	if cfg.S2SSecret != "" {
		msg.Auth = computeRelayAuth(globalID, ts)
	}
	for _, addr := range addrs {
		go dialRelay(addr, msg)
	}
}

const peersFile = "peers.json"

type savedPeer struct {
	Client string `json:"client"`
	S2S    string `json:"s2s"`
}

func savePeers() {
	peers := peerStore.Snapshot()
	out := make([]savedPeer, 0, len(peers))
	for _, p := range peers {
		if p.ClientAddr != "" {
			out = append(out, savedPeer{Client: p.ClientAddr, S2S: p.S2SAddr})
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(peersFile, data, 0644) //nolint:errcheck
}

func loadPeers() {
	data, err := os.ReadFile(peersFile)
	if err != nil {
		return
	}
	var peers []savedPeer
	if err := json.Unmarshal(data, &peers); err != nil {
		return
	}
	for _, p := range peers {
		if p.Client != "" {
			peerStore.Add(p.Client, p.S2S)
		}
	}
	if len(peers) > 0 {
		fmt.Printf("📋 Loaded %d peer(s) from %s\n", len(peers), peersFile)
	}
}
