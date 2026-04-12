package main

import (
	"bufio"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/net/proxy"
)

type Config struct {
	ProxyAddr    string `json:"proxy_addr"`
	ServerAddr   string `json:"server_addr"`
	DirectMode   bool   `json:"direct_mode"`
	MaxFrameSize int    `json:"max_frame_size"`
}

var (
	cfg          Config
	sessionID    uint16
	myLogin      string // set after successful login
	conn         net.Conn
	sharedKey    []byte
	fragCounter  atomic.Uint64
	sidNames     = make(map[uint16]string) // SID → username
	knownServers []string

	// E2E keys
	e2ePrivKey *ecdh.PrivateKey
	e2ePubKey  *ecdh.PublicKey

	// pubkey cache: login → 32-byte X25519 pubkey
	pubkeyMu     sync.RWMutex
	knownPubkeys = make(map[string][]byte)

	// messages queued while waiting for missing pubkeys
	pendingMu       sync.Mutex
	pendingMessages []string

	// Room state: id → name, id → member logins
	roomsMu    sync.RWMutex
	rooms      = make(map[uint32]string)   // roomID → name
	roomMembersMap = make(map[uint32][]string) // roomID → []login
)

const (
	CmdRegister     = 0x01
	CmdLogin        = 0x02
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
	// Rooms
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

func main() {
	loadConfig("client_config.json")

	var err error
	if cfg.DirectMode {
		fmt.Printf("🌐 Режим: Direct Connect | Подключение к: %s...\n", cfg.ServerAddr)
		conn, err = net.DialTimeout("tcp", cfg.ServerAddr, 10*time.Second)
	} else {
		fmt.Printf("🌐 Режим: DNSTT Proxy (SOCKS5) | Прокси: %s -> Сервер: %s...\n", cfg.ProxyAddr, cfg.ServerAddr)
		baseDialer := &net.Dialer{Timeout: 10 * time.Second}
		socks5Dialer, dialErr := proxy.SOCKS5("tcp", cfg.ProxyAddr, nil, baseDialer)
		if dialErr != nil {
			fmt.Printf("❌ Ошибка создания SOCKS5 диалера: %v\n", dialErr)
			return
		}
		conn, err = socks5Dialer.Dial("tcp", cfg.ServerAddr)
	}
	if err != nil {
		fmt.Printf("❌ Ошибка подключения: %v\n", err)
		return
	}
	defer conn.Close()

	sharedKey, err = ecdhHandshake(conn)
	if err != nil {
		fmt.Printf("❌ ECDH хендшейк не удался: %v\n", err)
		return
	}
	fmt.Println("🔐 Защищённый канал установлен.")

	// Load or generate long-term E2E keypair
	if err := loadOrGenerateE2EKey(); err != nil {
		fmt.Printf("❌ Ошибка E2E ключа: %v\n", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n=== DNS Messenger Client ===")

	for {
		fmt.Println("\n1. Вход\n2. Регистрация")
		fmt.Print("> ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice != "1" && choice != "2" {
			fmt.Println("❌ Введите 1 или 2.")
			continue
		}

		fmt.Print("Логин: ")
		login, _ := reader.ReadString('\n')
		login = strings.TrimSpace(login)
		if login == "" {
			fmt.Println("❌ Логин не может быть пустым.")
			continue
		}

		fmt.Print("Пароль: ")
		pass, _ := reader.ReadString('\n')
		pass = strings.TrimSpace(pass)
		if pass == "" {
			fmt.Println("❌ Пароль не может быть пустым.")
			continue
		}
		if len(login) > 255 || len(pass) > 255 {
			fmt.Println("❌ Логин и пароль не должны превышать 255 символов.")
			continue
		}

		if choice == "2" {
			ok, regErr := register(login, pass)
			if regErr != nil {
				fmt.Println("❌ Ошибка связи:", regErr)
				return
			}
			if ok {
				fmt.Println("✨ Аккаунт создан! Теперь войдите.")
			} else {
				fmt.Println("❌ Логин уже занят.")
			}
			continue
		}

		loginDone := make(chan bool, 1)
		historyDone := make(chan struct{})
		sendLoginPacket(login, pass)
		go readLoop(loginDone, historyDone)

		if ok := <-loginDone; !ok {
			fmt.Println("❌ Неверный логин или пароль.")
			return
		}

		myLogin = login

		// Upload our E2E public key to the server
		sendSetPublicKey()

		fmt.Println("\n--- История чата ---")
		<-historyDone
		fmt.Println("--- Конец истории ---")
		fmt.Println()
		break
	}

	fmt.Println("✅ Авторизация успешна! (/exit — выход, /servers — серверы, /dm <user> <text> — личное сообщение,")
	fmt.Println("   /rooms — список комнат, /join <id> — войти в комнату, /leave <id> — покинуть,")
	fmt.Println("   /create <name> [pub] — создать комнату, /room <id> <text> — сообщение в комнату,")
	fmt.Println("   /invite <roomID> <user> — пригласить)")

	reader2 := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		text, _ := reader2.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "/exit" {
			break
		}
		if text == "/servers" {
			if len(knownServers) == 0 {
				fmt.Println("📡 Список серверов пуст.")
			} else {
				fmt.Printf("📡 Известные серверы (%d):\n", len(knownServers))
				for i, s := range knownServers {
					fmt.Printf("  %d. %s\n", i+1, s)
				}
			}
			continue
		}
		if text == "/rooms" {
			roomsMu.RLock()
			if len(rooms) == 0 {
				fmt.Println("🏠 Нет доступных комнат.")
			} else {
				fmt.Printf("🏠 Комнаты (%d):\n", len(rooms))
				for id, name := range rooms {
					fmt.Printf("  [%d] %s\n", id, name)
				}
			}
			roomsMu.RUnlock()
			continue
		}
		// /dm <user> <text>
		if strings.HasPrefix(text, "/dm ") {
			parts := strings.SplitN(text[4:], " ", 2)
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				fmt.Println("Использование: /dm <user> <text>")
				continue
			}
			sendDM(parts[0], parts[1])
			continue
		}
		// /join <roomID>
		if strings.HasPrefix(text, "/join ") {
			idStr := strings.TrimSpace(text[6:])
			var id uint32
			if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
				fmt.Println("Использование: /join <roomID>")
				continue
			}
			sendJoinRoom(id)
			continue
		}
		// /leave <roomID>
		if strings.HasPrefix(text, "/leave ") {
			idStr := strings.TrimSpace(text[7:])
			var id uint32
			if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
				fmt.Println("Использование: /leave <roomID>")
				continue
			}
			sendLeaveRoom(id)
			continue
		}
		// /create <name> [pub]
		if strings.HasPrefix(text, "/create ") {
			args := strings.Fields(text[8:])
			if len(args) == 0 {
				fmt.Println("Использование: /create <name> [pub]")
				continue
			}
			name := args[0]
			isPublic := len(args) > 1 && args[1] == "pub"
			sendCreateRoom(name, isPublic, "")
			continue
		}
		// /room <roomID> <text>
		if strings.HasPrefix(text, "/room ") {
			parts := strings.SplitN(text[6:], " ", 2)
			if len(parts) < 2 {
				fmt.Println("Использование: /room <roomID> <text>")
				continue
			}
			var id uint32
			if _, err := fmt.Sscanf(parts[0], "%d", &id); err != nil {
				fmt.Println("Использование: /room <roomID> <text>")
				continue
			}
			sendRoomMessage(id, parts[1])
			continue
		}
		// /invite <roomID> <user>
		if strings.HasPrefix(text, "/invite ") {
			parts := strings.Fields(text[8:])
			if len(parts) < 2 {
				fmt.Println("Использование: /invite <roomID> <user>")
				continue
			}
			var id uint32
			if _, err := fmt.Sscanf(parts[0], "%d", &id); err != nil {
				fmt.Println("Использование: /invite <roomID> <user>")
				continue
			}
			sendRoomInvite(id, parts[1])
			continue
		}
		if text == "" {
			continue
		}
		sendE2EMessage(text)
	}
}

// writeFrame sends [TotalLen(2 LE)][cmd(1)][payload...] to c.
func writeFrame(c net.Conn, cmd byte, payload []byte) {
	total := 1 + len(payload)
	frame := make([]byte, 2+total)
	binary.LittleEndian.PutUint16(frame[0:2], uint16(total))
	frame[2] = cmd
	copy(frame[3:], payload)
	c.Write(frame) //nolint:errcheck
}

// readOneFrame reads exactly one framed message (blocking until complete).
func readOneFrame(c net.Conn) ([]byte, error) {
	lenBuf := make([]byte, 2)
	if _, err := readFull(c, lenBuf); err != nil {
		return nil, err
	}
	frameLen := int(binary.LittleEndian.Uint16(lenBuf))
	if frameLen < 1 {
		return nil, fmt.Errorf("пустой фрейм")
	}
	frame := make([]byte, frameLen)
	if _, err := readFull(c, frame); err != nil {
		return nil, err
	}
	return frame, nil
}

func ecdhHandshake(c net.Conn) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("генерация ключа: %w", err)
	}
	serverPubBytes := make([]byte, 32)
	if _, err = readFull(c, serverPubBytes); err != nil {
		return nil, fmt.Errorf("чтение публичного ключа сервера: %w", err)
	}
	serverPub, err := curve.NewPublicKey(serverPubBytes)
	if err != nil {
		return nil, fmt.Errorf("парсинг публичного ключа сервера: %w", err)
	}
	if _, err = c.Write(privKey.PublicKey().Bytes()); err != nil {
		return nil, fmt.Errorf("отправка публичного ключа: %w", err)
	}
	shared, err := privKey.ECDH(serverPub)
	if err != nil {
		return nil, fmt.Errorf("вычисление общего секрета: %w", err)
	}
	return shared, nil
}

func readFull(c net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func register(login, pass string) (bool, error) {
	if len(login) > 255 || len(pass) > 255 {
		return false, fmt.Errorf("слишком длинные данные")
	}
	payload := []byte{byte(len(login))}
	payload = append(payload, []byte(login)...)
	payload = append(payload, []byte(pass)...)
	writeFrame(conn, CmdRegister, payload)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	frame, err := readOneFrame(conn)
	conn.SetReadDeadline(time.Time{}) //nolint:errcheck
	if err != nil {
		return false, err
	}
	if len(frame) < 1 {
		return false, fmt.Errorf("пустой ответ")
	}
	return frame[0] == 0x01, nil
}

func sendLoginPacket(login, pass string) {
	payload := []byte{byte(len(login))}
	payload = append(payload, []byte(login)...)
	payload = append(payload, []byte(pass)...)
	writeFrame(conn, CmdLogin, payload)
}

// sendSetPublicKey uploads our long-term E2E public key to the server.
func sendSetPublicKey() {
	if e2ePubKey == nil {
		return
	}
	writeFrame(conn, CmdSetPublicKey, e2ePubKey.Bytes())
}

// sendPublicKeyRequest asks the server for a user's E2E public key.
func sendPublicKeyRequest(username string) {
	payload := []byte{byte(len(username))}
	payload = append(payload, []byte(username)...)
	writeFrame(conn, CmdPublicKeyRequest, payload)
}

// sendE2EMessage encrypts text for all online users and sends via CmdE2EMsg fragments.
func sendE2EMessage(text string) {
	if e2ePrivKey == nil {
		fmt.Println("❌ E2E ключ не инициализирован")
		return
	}

	// Collect recipients: all users whose pubkeys are known (local + federated) + self
	pubkeyMu.RLock()
	recipients := make(map[string][]byte, len(knownPubkeys)+1)
	for name, pk := range knownPubkeys {
		pkCopy := make([]byte, len(pk))
		copy(pkCopy, pk)
		recipients[name] = pkCopy
	}
	// Check for missing pubkeys among locally-online users (same server)
	var missing []string
	for _, name := range sidNames {
		if name == myLogin {
			continue
		}
		if _, ok := knownPubkeys[name]; !ok {
			missing = append(missing, name)
		}
	}
	pubkeyMu.RUnlock()

	// Add self unconditionally — we always have our own pubkey
	if myLogin != "" {
		recipients[myLogin] = e2ePubKey.Bytes()
	}

	if len(missing) > 0 {
		// Queue the message; request missing pubkeys
		pendingMu.Lock()
		pendingMessages = append(pendingMessages, text)
		pendingMu.Unlock()
		for _, name := range missing {
			sendPublicKeyRequest(name)
		}
		fmt.Printf("⏳ Ожидаем ключи (%s)...\n>> ", strings.Join(missing, ", "))
		return
	}

	doSendE2EMessage(text, recipients)
}

func doSendE2EMessage(text string, recipients map[string][]byte) {
	// 1. Random message key
	msgKey := make([]byte, 32)
	if _, err := rand.Read(msgKey); err != nil {
		fmt.Println("❌ Ошибка генерации msgKey:", err)
		return
	}

	// 2. Random nonce for content
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		fmt.Println("❌ Ошибка генерации nonce:", err)
		return
	}

	// 3. Encrypt content
	aead, err := chacha20poly1305.New(msgKey)
	if err != nil {
		fmt.Println("❌ Ошибка AEAD:", err)
		return
	}
	encContent := aead.Seal(nil, nonce, []byte(text), nil)

	// 4. Build envelopes for each recipient
	type envEntry struct {
		login string
		env   []byte
	}
	var envelopes []envEntry
	for login, pubkey := range recipients {
		env, err := sealEnvelope(pubkey, msgKey)
		if err != nil {
			fmt.Printf("⚠️ Не удалось создать envelope для %s: %v\n", login, err)
			continue
		}
		envelopes = append(envelopes, envEntry{login: login, env: env})
	}

	// 5. Build assembled payload
	// [Nonce(12)][EncContentLen(2 LE)][EncContent(N)][EnvelopeCount(1)]
	// per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
	var assembled []byte
	assembled = append(assembled, nonce...)
	ecLenBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(ecLenBuf, uint16(len(encContent)))
	assembled = append(assembled, ecLenBuf...)
	assembled = append(assembled, encContent...)
	assembled = append(assembled, byte(len(envelopes)))
	for _, e := range envelopes {
		assembled = append(assembled, byte(len(e.login)))
		assembled = append(assembled, []byte(e.login)...)
		assembled = append(assembled, e.env...)
	}

	// 6. Fragment and send (always fragmented; assembled is prefixed with CmdE2EMsg tag)
	sendFragmented(CmdE2EMsg, assembled)
}

// sendFragmented sends payload as CmdFragment frames with cmd as the first byte of assembled data.
func sendFragmented(cmd byte, payload []byte) {
	maxFrame := cfg.MaxFrameSize
	if maxFrame < 20 {
		maxFrame = 180
	}

	// Prepend cmd byte so server's handleFragment can route
	full := append([]byte{cmd}, payload...)

	msgID := byte(fragCounter.Add(1))
	maxChunk := maxFrame - 6 // 2(len) + 1(CmdFragment) + 3(msgID, fragIdx, fragCount)
	if maxChunk < 1 {
		maxChunk = 100
	}

	var chunks [][]byte
	remaining := full
	for len(remaining) > 0 {
		end := maxChunk
		if end > len(remaining) {
			end = len(remaining)
		}
		chunks = append(chunks, remaining[:end])
		remaining = remaining[end:]
	}

	if len(chunks) > 255 {
		fmt.Println("❌ Сообщение слишком большое для фрагментации")
		return
	}

	fragCount := byte(len(chunks))
	for i, chunk := range chunks {
		fp := make([]byte, 3+len(chunk))
		fp[0] = msgID
		fp[1] = byte(i)
		fp[2] = fragCount
		copy(fp[3:], chunk)
		writeFrame(conn, CmdFragment, fp)
	}
}

// flushPendingMessages sends queued messages if all pubkeys are now available.
func flushPendingMessages() {
	pendingMu.Lock()
	msgs := pendingMessages
	pendingMessages = nil
	pendingMu.Unlock()

	for _, text := range msgs {
		pubkeyMu.RLock()
		recipients := make(map[string][]byte, len(knownPubkeys)+1)
		for name, pk := range knownPubkeys {
			pkCopy := make([]byte, len(pk))
			copy(pkCopy, pk)
			recipients[name] = pkCopy
		}
		var missing []string
		for _, name := range sidNames {
			if name == myLogin {
				continue
			}
			if _, ok := knownPubkeys[name]; !ok {
				missing = append(missing, name)
			}
		}
		pubkeyMu.RUnlock()

		if myLogin != "" {
			recipients[myLogin] = e2ePubKey.Bytes()
		}

		if len(missing) > 0 {
			pendingMu.Lock()
			pendingMessages = append(pendingMessages, text)
			pendingMu.Unlock()
		} else {
			doSendE2EMessage(text, recipients)
		}
	}
}

func readLoop(loginDone chan bool, historyDone chan struct{}) {
	var pending []byte
	tmp := make([]byte, 4096)
	historyFinished := false
	loginHandled := false

	for {
		n, err := conn.Read(tmp)
		if err != nil {
			fmt.Println("\n📡 Соединение закрыто сервером.")
			os.Exit(0)
		}
		pending = append(pending, tmp[:n]...)

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

			case CmdLoginOK:
				if len(payload) < 2 {
					continue
				}
				sessionID = uint16(payload[0])<<8 | uint16(payload[1])
				if !loginHandled {
					loginHandled = true
					loginDone <- true
				}

			case CmdLoginFail:
				if !loginHandled {
					loginHandled = true
					loginDone <- false
				}

			case CmdAck:
				// Message delivery acknowledgment.

			case CmdE2EHistory:
				// Payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
				handleE2EHistory(payload)

			case CmdHistoryEnd:
				if !historyFinished {
					historyFinished = true
					close(historyDone)
				}

			case CmdE2EIncoming:
				// Payload: [SenderSID(2 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
				handleE2EIncoming(payload)

			case CmdPublicKey:
				// Payload: [UsernameLen(1)][Username(N)][pubkey(32)]
				handlePublicKeyResponse(payload)

			case CmdOnlineList:
				if len(payload) < 1 {
					continue
				}
				count := int(payload[0])
				off := 1
				newMap := make(map[uint16]string)
				names := make([]string, 0, count)
				valid := true
				for i := 0; i < count; i++ {
					if off+3 > len(payload) {
						valid = false
						break
					}
					sid := uint16(payload[off])<<8 | uint16(payload[off+1])
					off += 2
					nLen := int(payload[off])
					off++
					if off+nLen > len(payload) {
						valid = false
						break
					}
					name := string(payload[off : off+nLen])
					off += nLen
					newMap[sid] = name
					names = append(names, name)
				}
				if valid {
					sidNames = newMap
					// Request pubkeys for all online users
					for _, name := range names {
						pubkeyMu.RLock()
						_, have := knownPubkeys[name]
						pubkeyMu.RUnlock()
						if !have {
							sendPublicKeyRequest(name)
						}
					}
					fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(names), strings.Join(names, ", "))
				}

			case CmdOnlineAdd:
				if len(payload) < 4 {
					continue
				}
				sid := uint16(payload[0])<<8 | uint16(payload[1])
				nLen := int(payload[2])
				if 3+nLen > len(payload) {
					continue
				}
				name := string(payload[3 : 3+nLen])
				sidNames[sid] = name
				// Request pubkey for the new user
				pubkeyMu.RLock()
				_, have := knownPubkeys[name]
				pubkeyMu.RUnlock()
				if !have {
					sendPublicKeyRequest(name)
				}
				allNames := make([]string, 0, len(sidNames))
				for _, n := range sidNames {
					allNames = append(allNames, n)
				}
				fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(allNames), strings.Join(allNames, ", "))

			case CmdOnlineRemove:
				if len(payload) < 2 {
					continue
				}
				sid := uint16(payload[0])<<8 | uint16(payload[1])
				delete(sidNames, sid)
				allNames := make([]string, 0, len(sidNames))
				for _, n := range sidNames {
					allNames = append(allNames, n)
				}
				fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(allNames), strings.Join(allNames, ", "))

			case CmdServerList:
				if len(payload) < 1 {
					continue
				}
				count := int(payload[0])
				off := 1
				servers := make([]string, 0, count)
				for i := 0; i < count; i++ {
					if off >= len(payload) {
						break
					}
					aLen := int(payload[off])
					off++
					if off+aLen > len(payload) {
						break
					}
					servers = append(servers, string(payload[off:off+aLen]))
					off += aLen
				}
				knownServers = servers
				if len(servers) > 0 {
					fmt.Printf("\n📡 Серверы сети (%d): %s\n>> ", len(servers), strings.Join(servers, ", "))
				}

			case CmdDMIncoming:
				handleDMIncoming(payload)

			case CmdDMHistory:
				handleDMHistory(payload)

			case CmdRoomList:
				handleRoomList(payload)

			case CmdRoomCreated:
				handleRoomCreated(payload)

			case CmdRoomMembers:
				handleRoomMembers(payload)

			case CmdRoomMemberAdd:
				handleRoomMemberAdd(payload)

			case CmdRoomMemberRem:
				handleRoomMemberRem(payload)

			case CmdRoomMsgIncoming:
				handleRoomMsgIncoming(payload)

			case CmdRoomHistory:
				handleRoomHistory(payload)
			}
		}
	}
}

// handleE2EIncoming decrypts and displays an incoming E2E message.
// Payload: [SenderLen(1)][Sender(N)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
func handleE2EIncoming(payload []byte) {
	// minimum: 1(senderLen) + 4(msgID) + 14(storedBlob min) + 80(envelope) = 99
	if len(payload) < 99 {
		return
	}
	senderLen := int(payload[0])
	if 1+senderLen+4+14+80 > len(payload) {
		return
	}
	senderName := string(payload[1 : 1+senderLen])
	// MsgID at offset 1+senderLen (4 bytes LE) — unused on client side
	off := 1 + senderLen + 4
	storedBlob := payload[off : len(payload)-80]
	envelope := payload[len(payload)-80:]

	if len(storedBlob) < 14 {
		return
	}
	nonce := storedBlob[:12]
	encContent := storedBlob[12:]
	encContentLen := int(binary.LittleEndian.Uint16(encContent[:2]))
	if len(encContent) < 2+encContentLen {
		return
	}
	ciphertext := encContent[2 : 2+encContentLen]

	plain, err := decryptE2E(envelope, nonce, ciphertext)
	if err != nil {
		return
	}

	if senderName == "" {
		senderName = "unknown"
	}
	now := time.Now().Local().Format("15:04")
	fmt.Printf("\n📨 [%s] [%s]: %s\n>> ", now, senderName, string(plain))
}

// handleE2EHistory decrypts and displays a history message.
// Payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][MsgID(4 LE)][Nonce(12)][EncContentLen(2 LE)][EncContent(N)][Envelope(80)]
func handleE2EHistory(payload []byte) {
	// minimum: 1 + 0 + 4 + 4 + 12 + 2 + 0 + 80 = 103 bytes
	if len(payload) < 103 {
		return
	}
	senderLen := int(payload[0])
	if 1+senderLen+4+4+12+2+80 > len(payload) {
		return
	}
	sender := string(payload[1 : 1+senderLen])
	tsOff := 1 + senderLen
	ts := uint32(payload[tsOff])<<24 | uint32(payload[tsOff+1])<<16 |
		uint32(payload[tsOff+2])<<8 | uint32(payload[tsOff+3])
	// MsgID at tsOff+4 (4 bytes, LE) — unused on client side
	blobOff := tsOff + 4 + 4

	storedBlob := payload[blobOff : len(payload)-80]
	envelope := payload[len(payload)-80:]

	if len(storedBlob) < 12+2 {
		return
	}
	nonce := storedBlob[:12]
	encContentLen := int(binary.LittleEndian.Uint16(storedBlob[12:14]))
	if len(storedBlob) < 14+encContentLen {
		return
	}
	ciphertext := storedBlob[14 : 14+encContentLen]

	plain, err := decryptE2E(envelope, nonce, ciphertext)
	if err != nil {
		return
	}

	timeStr := time.Unix(int64(ts), 0).Local().Format("2006-01-02 15:04")
	fmt.Printf("  [%s] %s: %s\n", timeStr, sender, string(plain))
}

// handlePublicKeyResponse stores a received pubkey and flushes pending messages.
// Payload: [UsernameLen(1)][Username(N)][pubkey(32)]
func handlePublicKeyResponse(payload []byte) {
	if len(payload) < 1+32 {
		return
	}
	lLen := int(payload[0])
	if len(payload) < 1+lLen+32 {
		return
	}
	username := string(payload[1 : 1+lLen])
	pubkey := make([]byte, 32)
	copy(pubkey, payload[1+lLen:1+lLen+32])

	pubkeyMu.Lock()
	knownPubkeys[username] = pubkey
	pubkeyMu.Unlock()

	// Try to flush any pending messages
	flushPendingMessages()
}

// decryptE2E decrypts a message using the stored envelope and our private E2E key.
func decryptE2E(envelope, nonce, ciphertext []byte) ([]byte, error) {
	if e2ePrivKey == nil {
		return nil, fmt.Errorf("no E2E private key")
	}
	msgKey, err := openEnvelope(e2ePrivKey, envelope)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.New(msgKey)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

// sealEnvelope creates an 80-byte envelope: ephemeral_pub(32) + ChaCha20Poly1305(msgKey)(48)
func sealEnvelope(recipientPub []byte, msgKey []byte) ([]byte, error) {
	curve := ecdh.X25519()
	ephPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	ephPubBytes := ephPriv.PublicKey().Bytes()

	recipKey, err := curve.NewPublicKey(recipientPub)
	if err != nil {
		return nil, err
	}
	shared, err := ephPriv.ECDH(recipKey)
	if err != nil {
		return nil, err
	}

	// HKDF(shared, salt=ephPub, info="dnstt-e2e-v1") → 44 bytes (32 key + 12 nonce)
	hk := hkdf.New(sha256.New, shared, ephPubBytes, []byte("dnstt-e2e-v1"))
	km := make([]byte, 44)
	if _, err = io.ReadFull(hk, km); err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.New(km[:32])
	if err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, km[32:], msgKey, nil) // 32+16 = 48 bytes

	envelope := make([]byte, 80)
	copy(envelope[:32], ephPubBytes)
	copy(envelope[32:], ct)
	return envelope, nil
}

// openEnvelope decrypts an 80-byte envelope and returns the 32-byte msgKey.
func openEnvelope(privKey *ecdh.PrivateKey, envelope []byte) ([]byte, error) {
	if len(envelope) != 80 {
		return nil, fmt.Errorf("invalid envelope length: %d", len(envelope))
	}
	curve := ecdh.X25519()
	ephPub, err := curve.NewPublicKey(envelope[:32])
	if err != nil {
		return nil, err
	}
	shared, err := privKey.ECDH(ephPub)
	if err != nil {
		return nil, err
	}

	hk := hkdf.New(sha256.New, shared, envelope[:32], []byte("dnstt-e2e-v1"))
	km := make([]byte, 44)
	if _, err = io.ReadFull(hk, km); err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.New(km[:32])
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, km[32:], envelope[32:], nil) // decrypt 48 bytes → 32 bytes
}

// E2E key persistence

type e2eKeyFile struct {
	PrivKey []byte `json:"priv_key"` // 32 bytes raw X25519
	PubKey  []byte `json:"pub_key"`  // 32 bytes
}

func loadOrGenerateE2EKey() error {
	const keyPath = "e2e_key.json"
	data, err := os.ReadFile(keyPath)
	if err == nil {
		var kf e2eKeyFile
		if json.Unmarshal(data, &kf) == nil && len(kf.PrivKey) == 32 {
			curve := ecdh.X25519()
			priv, err := curve.NewPrivateKey(kf.PrivKey)
			if err == nil {
				e2ePrivKey = priv
				e2ePubKey = priv.PublicKey()
				fmt.Println("🔑 E2E ключ загружен из e2e_key.json")
				return nil
			}
		}
	}

	// Generate new key
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	e2ePrivKey = priv
	e2ePubKey = priv.PublicKey()

	kf := e2eKeyFile{
		PrivKey: priv.Bytes(),
		PubKey:  priv.PublicKey().Bytes(),
	}
	keyData, _ := json.Marshal(kf)
	os.WriteFile(keyPath, keyData, 0600) //nolint:errcheck
	fmt.Println("🔑 Новый E2E ключ сгенерирован и сохранён в e2e_key.json")
	return nil
}

func loadConfig(path string) {
	cfg = Config{
		ProxyAddr:    "127.0.0.1:8080",
		ServerAddr:   "127.0.0.1:9999",
		DirectMode:   false,
		MaxFrameSize: 180,
	}
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		json.NewDecoder(file).Decode(&cfg) //nolint:errcheck
	} else {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
	}
}

// ─── Direct Messages ─────────────────────────────────────────────────────────

// sendDM encrypts and sends a direct message to recipientLogin.
func sendDM(recipientLogin, text string) {
	if e2ePrivKey == nil {
		fmt.Println("❌ E2E ключ не инициализирован")
		return
	}
	pubkeyMu.RLock()
	recipPub, ok := knownPubkeys[recipientLogin]
	pubkeyMu.RUnlock()
	if !ok {
		fmt.Printf("⏳ Ключ %s не известен, запрашиваем...\n>> ", recipientLogin)
		sendPublicKeyRequest(recipientLogin)
		return
	}

	msgKey := make([]byte, 32)
	nonce := make([]byte, 12)
	rand.Read(msgKey)  //nolint:errcheck
	rand.Read(nonce)   //nolint:errcheck

	aead, err := chacha20poly1305.New(msgKey)
	if err != nil {
		fmt.Println("❌ Ошибка AEAD:", err)
		return
	}
	encContent := aead.Seal(nil, nonce, []byte(text), nil)

	// Envelopes: recipient + self
	type envEntry struct {
		login string
		env   []byte
	}
	var envelopes []envEntry
	if env, err := sealEnvelope(recipPub, msgKey); err == nil {
		envelopes = append(envelopes, envEntry{recipientLogin, env})
	}
	pubkeyMu.RLock()
	selfPub, hasSelf := knownPubkeys[myLogin]
	pubkeyMu.RUnlock()
	if !hasSelf && e2ePubKey != nil {
		selfPub = e2ePubKey.Bytes()
		hasSelf = true
	}
	if hasSelf && myLogin != "" {
		if env, err := sealEnvelope(selfPub, msgKey); err == nil {
			envelopes = append(envelopes, envEntry{myLogin, env})
		}
	}

	// Assemble: [RecipLen(1)][Recip(N)][Nonce(12)][EncContentLen(2LE)][EncContent][EnvCount(1)][envs...]
	rl := []byte(recipientLogin)
	var assembled []byte
	assembled = append(assembled, byte(len(rl)))
	assembled = append(assembled, rl...)
	assembled = append(assembled, nonce...)
	ecLenBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(ecLenBuf, uint16(len(encContent)))
	assembled = append(assembled, ecLenBuf...)
	assembled = append(assembled, encContent...)
	assembled = append(assembled, byte(len(envelopes)))
	for _, e := range envelopes {
		lb := []byte(e.login)
		assembled = append(assembled, byte(len(lb)))
		assembled = append(assembled, lb...)
		assembled = append(assembled, e.env...)
	}
	sendFragmented(CmdDM, assembled)
	fmt.Printf("💬 [DM → %s]: %s\n>> ", recipientLogin, text)
}

// handleDMIncoming: [SenderLen(1)][Sender(N)][MsgID(4LE)][storedBlob][Envelope(80)]
func handleDMIncoming(payload []byte) {
	if len(payload) < 1+1+4+14+80 {
		return
	}
	sLen := int(payload[0])
	if len(payload) < 1+sLen+4+14+80 {
		return
	}
	sender := string(payload[1 : 1+sLen])
	storedBlob := payload[1+sLen+4 : len(payload)-80]
	envelope := payload[len(payload)-80:]
	if len(storedBlob) < 14 {
		return
	}
	nonce := storedBlob[:12]
	encContentLen := int(binary.LittleEndian.Uint16(storedBlob[12:14]))
	if len(storedBlob) < 14+encContentLen {
		return
	}
	plain, err := decryptE2E(envelope, nonce, storedBlob[14:14+encContentLen])
	if err != nil {
		return
	}
	now := time.Now().Local().Format("15:04")
	fmt.Printf("\n💬 [DM от %s] [%s]: %s\n>> ", sender, now, string(plain))
}

// handleDMHistory: [SenderLen(1)][Sender(N)][RecipLen(1)][Recip(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
func handleDMHistory(payload []byte) {
	if len(payload) < 1+1+1+4+4+14+80 {
		return
	}
	sLen := int(payload[0])
	if len(payload) < 1+sLen+1 {
		return
	}
	sender := string(payload[1 : 1+sLen])
	rLen := int(payload[1+sLen])
	if len(payload) < 1+sLen+1+rLen+4+4+14+80 {
		return
	}
	recip := string(payload[1+sLen+1 : 1+sLen+1+rLen])
	tsOff := 1 + sLen + 1 + rLen
	ts := uint32(payload[tsOff])<<24 | uint32(payload[tsOff+1])<<16 |
		uint32(payload[tsOff+2])<<8 | uint32(payload[tsOff+3])
	blobOff := tsOff + 4 + 4
	storedBlob := payload[blobOff : len(payload)-80]
	envelope := payload[len(payload)-80:]
	if len(storedBlob) < 14 {
		return
	}
	nonce := storedBlob[:12]
	encContentLen := int(binary.LittleEndian.Uint16(storedBlob[12:14]))
	if len(storedBlob) < 14+encContentLen {
		return
	}
	plain, err := decryptE2E(envelope, nonce, storedBlob[14:14+encContentLen])
	if err != nil {
		return
	}
	timeStr := time.Unix(int64(ts), 0).Local().Format("2006-01-02 15:04")
	fmt.Printf("  [DM %s→%s] [%s]: %s\n", sender, recip, timeStr, string(plain))
}

// ─── Rooms ────────────────────────────────────────────────────────────────────

// sendCreateRoom: [NameLen(1)][Name(N)][IsPublic(1)][DescLen(2LE)][Desc(N)]
func sendCreateRoom(name string, isPublic bool, desc string) {
	nb := []byte(name)
	db := []byte(desc)
	pub := byte(0)
	if isPublic {
		pub = 1
	}
	descLenBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(descLenBuf, uint16(len(db)))
	var payload []byte
	payload = append(payload, byte(len(nb)))
	payload = append(payload, nb...)
	payload = append(payload, pub)
	payload = append(payload, descLenBuf...)
	payload = append(payload, db...)
	writeFrame(conn, CmdCreateRoom, payload)
}

// sendJoinRoom: [RoomID(4LE)]
func sendJoinRoom(roomID uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, roomID)
	writeFrame(conn, CmdJoinRoom, buf)
}

// sendLeaveRoom: [RoomID(4LE)]
func sendLeaveRoom(roomID uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, roomID)
	writeFrame(conn, CmdLeaveRoom, buf)
}

// sendRoomInvite: [RoomID(4LE)][UserLen(1)][User(N)]
func sendRoomInvite(roomID uint32, username string) {
	ub := []byte(username)
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, roomID)
	var payload []byte
	payload = append(payload, buf...)
	payload = append(payload, byte(len(ub)))
	payload = append(payload, ub...)
	writeFrame(conn, CmdRoomInvite, payload)
}

// sendRoomMessage encrypts text for all known room members and sends via CmdRoomMsg.
func sendRoomMessage(roomID uint32, text string) {
	if e2ePrivKey == nil {
		fmt.Println("❌ E2E ключ не инициализирован")
		return
	}

	// Collect room members (not just online users) + self
	roomsMu.RLock()
	members := roomMembersMap[roomID]
	roomsMu.RUnlock()
	if len(members) == 0 {
		fmt.Println("⚠️ Список участников комнаты ещё не получен, попробуйте по��же")
		return
	}
	pubkeyMu.RLock()
	recipients := make(map[string][]byte)
	for _, name := range members {
		if name == myLogin {
			continue // self handled below
		}
		if pk, ok := knownPubkeys[name]; ok {
			recipients[name] = pk
		}
	}
	if myLogin != "" {
		if pk, ok := knownPubkeys[myLogin]; ok {
			recipients[myLogin] = pk
		} else if e2ePubKey != nil {
			recipients[myLogin] = e2ePubKey.Bytes()
		}
	}
	pubkeyMu.RUnlock()

	if len(recipients) == 0 {
		fmt.Println("⚠️ Нет получателей с известными ключами")
		return
	}

	msgKey := make([]byte, 32)
	nonce := make([]byte, 12)
	rand.Read(msgKey)  //nolint:errcheck
	rand.Read(nonce)   //nolint:errcheck

	aead, err := chacha20poly1305.New(msgKey)
	if err != nil {
		fmt.Println("❌ Ошибка AEAD:", err)
		return
	}
	encContent := aead.Seal(nil, nonce, []byte(text), nil)

	type envEntry struct {
		login string
		env   []byte
	}
	var envelopes []envEntry
	for login, pubkey := range recipients {
		if env, err := sealEnvelope(pubkey, msgKey); err == nil {
			envelopes = append(envelopes, envEntry{login, env})
		}
	}

	// [RoomID(4LE)][Nonce(12)][EncContentLen(2LE)][EncContent][EnvCount(1)][envs...]
	idBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBuf, roomID)
	ecLenBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(ecLenBuf, uint16(len(encContent)))
	var assembled []byte
	assembled = append(assembled, idBuf...)
	assembled = append(assembled, nonce...)
	assembled = append(assembled, ecLenBuf...)
	assembled = append(assembled, encContent...)
	assembled = append(assembled, byte(len(envelopes)))
	for _, e := range envelopes {
		lb := []byte(e.login)
		assembled = append(assembled, byte(len(lb)))
		assembled = append(assembled, lb...)
		assembled = append(assembled, e.env...)
	}
	sendFragmented(CmdRoomMsg, assembled)
	fmt.Printf("💬 [Room %d]: %s\n>> ", roomID, text)
}

// handleRoomList: [RoomCount(2LE)] per room: [RoomID(4LE)][NameLen(1)][Name(N)][IsPublic(1)][OwnerLen(1)][Owner(N)][MemberCount(2LE)]
func handleRoomList(payload []byte) {
	if len(payload) < 2 {
		return
	}
	count := int(binary.LittleEndian.Uint16(payload[:2]))
	off := 2
	roomsMu.Lock()
	for i := 0; i < count; i++ {
		if off+4+1 > len(payload) {
			break
		}
		id := binary.LittleEndian.Uint32(payload[off : off+4])
		off += 4
		nLen := int(payload[off])
		off++
		if off+nLen+1+1+2 > len(payload) {
			break
		}
		name := string(payload[off : off+nLen])
		off += nLen
		isPublic := payload[off]
		off++
		oLen := int(payload[off])
		off++
		if off+oLen+2 > len(payload) {
			break
		}
		owner := string(payload[off : off+oLen])
		off += oLen
		memberCount := int(binary.LittleEndian.Uint16(payload[off : off+2]))
		off += 2
		rooms[id] = name
		_ = isPublic
		fmt.Printf("\n🏠 Комната [%d] %s (владелец: %s, участников: %d)\n>> ", id, name, owner, memberCount)
	}
	roomsMu.Unlock()
}

// handleRoomCreated: [RoomID(4LE)][NameLen(1)][Name(N)][IsPublic(1)][OwnerLen(1)][Owner(N)]
func handleRoomCreated(payload []byte) {
	if len(payload) < 4+1+1+1 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	nLen := int(payload[4])
	if len(payload) < 5+nLen+1+1 {
		return
	}
	name := string(payload[5 : 5+nLen])
	roomsMu.Lock()
	rooms[id] = name
	roomsMu.Unlock()
	fmt.Printf("\n🏠 Комната создана/доступна: [%d] %s\n>> ", id, name)
}

// handleRoomMembers: [RoomID(4LE)][MemberCount(2LE)] per: [LoginLen(1)][Login(N)][IsAdmin(1)]
func handleRoomMembers(payload []byte) {
	if len(payload) < 6 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	count := int(binary.LittleEndian.Uint16(payload[4:6]))
	off := 6
	members := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if off >= len(payload) {
			break
		}
		lLen := int(payload[off])
		off++
		if off+lLen+1 > len(payload) {
			break
		}
		members = append(members, string(payload[off:off+lLen]))
		off += lLen + 1 // skip isAdmin
	}
	roomsMu.Lock()
	roomMembersMap[id] = members
	name := rooms[id]
	roomsMu.Unlock()
	// Request pubkeys for members we don't know yet
	pubkeyMu.RLock()
	for _, m := range members {
		if _, have := knownPubkeys[m]; !have {
			sendPublicKeyRequest(m)
		}
	}
	pubkeyMu.RUnlock()
	fmt.Printf("\n👥 Участники комнаты [%d] %s: %s\n>> ", id, name, strings.Join(members, ", "))
}

// handleRoomMemberAdd: [RoomID(4LE)][LoginLen(1)][Login(N)]
func handleRoomMemberAdd(payload []byte) {
	if len(payload) < 6 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	lLen := int(payload[4])
	if len(payload) < 5+lLen {
		return
	}
	login := string(payload[5 : 5+lLen])
	roomsMu.Lock()
	existing := roomMembersMap[id]
	found := false
	for _, m := range existing {
		if m == login {
			found = true
			break
		}
	}
	if !found {
		roomMembersMap[id] = append(existing, login)
	}
	name := rooms[id]
	roomsMu.Unlock()
	pubkeyMu.RLock()
	_, have := knownPubkeys[login]
	pubkeyMu.RUnlock()
	if !have {
		sendPublicKeyRequest(login)
	}
	fmt.Printf("\n➕ %s вошёл в комнату [%d] %s\n>> ", login, id, name)
}

// handleRoomMemberRem: [RoomID(4LE)][LoginLen(1)][Login(N)]
func handleRoomMemberRem(payload []byte) {
	if len(payload) < 6 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	lLen := int(payload[4])
	if len(payload) < 5+lLen {
		return
	}
	login := string(payload[5 : 5+lLen])
	roomsMu.Lock()
	existing := roomMembersMap[id]
	filtered := existing[:0]
	for _, m := range existing {
		if m != login {
			filtered = append(filtered, m)
		}
	}
	if login == myLogin {
		delete(roomMembersMap, id)
		delete(rooms, id)
	} else {
		roomMembersMap[id] = filtered
	}
	name := rooms[id]
	roomsMu.Unlock()
	fmt.Printf("\n➖ %s покинул комнату [%d] %s\n>> ", login, id, name)
}

// handleRoomMsgIncoming: [RoomID(4LE)][SenderLen(1)][Sender(N)][MsgID(4LE)][storedBlob][Envelope(80)]
func handleRoomMsgIncoming(payload []byte) {
	if len(payload) < 4+1+1+4+14+80 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	sLen := int(payload[4])
	if len(payload) < 5+sLen+4+14+80 {
		return
	}
	sender := string(payload[5 : 5+sLen])
	storedBlob := payload[5+sLen+4 : len(payload)-80]
	envelope := payload[len(payload)-80:]
	if len(storedBlob) < 14 {
		return
	}
	nonce := storedBlob[:12]
	encContentLen := int(binary.LittleEndian.Uint16(storedBlob[12:14]))
	if len(storedBlob) < 14+encContentLen {
		return
	}
	plain, err := decryptE2E(envelope, nonce, storedBlob[14:14+encContentLen])
	if err != nil {
		return
	}
	roomsMu.RLock()
	name := rooms[id]
	roomsMu.RUnlock()
	now := time.Now().Local().Format("15:04")
	fmt.Printf("\n💬 [Room %d/%s] [%s] %s: %s\n>> ", id, name, now, sender, string(plain))
}

// handleRoomHistory: [RoomID(4LE)][SenderLen(1)][Sender(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
func handleRoomHistory(payload []byte) {
	if len(payload) < 4+1+1+4+4+14+80 {
		return
	}
	id := binary.LittleEndian.Uint32(payload[:4])
	sLen := int(payload[4])
	if len(payload) < 5+sLen+4+4+14+80 {
		return
	}
	sender := string(payload[5 : 5+sLen])
	tsOff := 5 + sLen
	ts := uint32(payload[tsOff])<<24 | uint32(payload[tsOff+1])<<16 |
		uint32(payload[tsOff+2])<<8 | uint32(payload[tsOff+3])
	blobOff := tsOff + 4 + 4
	storedBlob := payload[blobOff : len(payload)-80]
	envelope := payload[len(payload)-80:]
	if len(storedBlob) < 14 {
		return
	}
	nonce := storedBlob[:12]
	encContentLen := int(binary.LittleEndian.Uint16(storedBlob[12:14]))
	if len(storedBlob) < 14+encContentLen {
		return
	}
	plain, err := decryptE2E(envelope, nonce, storedBlob[14:14+encContentLen])
	if err != nil {
		return
	}
	roomsMu.RLock()
	name := rooms[id]
	roomsMu.RUnlock()
	timeStr := time.Unix(int64(ts), 0).Local().Format("2006-01-02 15:04")
	fmt.Printf("  [Room %d/%s] [%s] %s: %s\n", id, name, timeStr, sender, string(plain))
}
