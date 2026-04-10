package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// PeerInfo holds the known addresses for a federated server peer.
type PeerInfo struct {
	ClientAddr string // "host:9999" — advertised to clients via CmdServerList
	S2SAddr    string // "host:9998" — used for outgoing gossip dials
	LastSeen   int64  // unix timestamp of last successful gossip exchange
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

type gossipPeer struct {
	Client string `json:"client"`
	S2S    string `json:"s2s"`
}

type gossipMsg struct {
	Type   string       `json:"type"`
	Client string       `json:"client"`
	S2S    string       `json:"s2s"`
	Peers  []gossipPeer `json:"peers"`
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

func buildGossipMsg() gossipMsg {
	myS2S := effectiveS2SPublicAddr()
	peers := peerStore.Snapshot()
	gpeers := make([]gossipPeer, 0, len(peers))
	for _, p := range peers {
		if p.ClientAddr == cfg.PublicAddr {
			continue
		}
		gpeers = append(gpeers, gossipPeer{Client: p.ClientAddr, S2S: p.S2SAddr})
	}
	return gossipMsg{Type: "gossip", Client: cfg.PublicAddr, S2S: myS2S, Peers: gpeers}
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
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck

	buf := make([]byte, 4096)
	n, err := readUntilNewline(conn, buf)
	if err != nil || n == 0 {
		return
	}
	var msg gossipMsg
	if err := json.Unmarshal(buf[:n], &msg); err != nil || msg.Type != "gossip" {
		return
	}
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
	data = append(data, '\n')
	conn.Write(data) //nolint:errcheck
	savePeers()
	fmt.Printf("🔗 S2S gossip from %s — %d peer(s) known\n", msg.Client, peerStore.Count())
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

func dialGossip(s2sAddr string) {
	conn, err := net.DialTimeout("tcp", s2sAddr, 5*time.Second)
	if err != nil {
		fmt.Printf("⚠️  Gossip: can't reach %s: %v\n", s2sAddr, err)
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck

	data, err := json.Marshal(buildGossipMsg())
	if err != nil {
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return
	}
	buf := make([]byte, 4096)
	n, err := readUntilNewline(conn, buf)
	if err != nil || n == 0 {
		return
	}
	var resp gossipMsg
	if err := json.Unmarshal(buf[:n], &resp); err != nil || resp.Type != "gossip" {
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

// readUntilNewline reads bytes from conn one at a time into buf until '\n' or buf is full.
func readUntilNewline(conn net.Conn, buf []byte) (int, error) {
	n := 0
	tmp := make([]byte, 1)
	for n < len(buf) {
		_, err := conn.Read(tmp)
		if err != nil {
			return n, err
		}
		if tmp[0] == '\n' {
			return n, nil
		}
		buf[n] = tmp[0]
		n++
	}
	return n, nil
}
