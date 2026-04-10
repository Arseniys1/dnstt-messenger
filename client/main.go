package main

import (
	"bufio"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/net/proxy"
)

type Config struct {
	ProxyAddr    string `json:"proxy_addr"`
	ServerAddr   string `json:"server_addr"`
	DirectMode   bool   `json:"direct_mode"`
	MaxFrameSize int    `json:"max_frame_size"`
}

var (
	cfg         Config
	sessionID   uint16
	conn        net.Conn
	sharedKey   []byte
	sendCounter atomic.Uint64
	fragCounter atomic.Uint64
	sidNames     = make(map[uint16]string) // SID → username (populated from CmdOnlineList/Add/Remove)
	knownServers []string                  // list of known federated server addresses
)

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

		fmt.Println("\n--- История чата ---")
		<-historyDone
		fmt.Println("--- Конец истории ---\n")
		break
	}

	fmt.Println("✅ Авторизация успешна! (/exit для выхода, /servers — список серверов сети)")

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
		if text == "" {
			continue
		}
		sendMessage(text)
	}
}

// writeFrame sends [TotalLen(2 LE)][cmd(1)][payload...] to c.
func writeFrame(c net.Conn, cmd byte, payload []byte) {
	total := 1 + len(payload)
	frame := make([]byte, 2+total)
	binary.LittleEndian.PutUint16(frame[0:2], uint16(total))
	frame[2] = cmd
	copy(frame[3:], payload)
	c.Write(frame)
}

// readOneFrame reads exactly one framed message (blocking until complete).
// Returns [cmd(1)][payload...].
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

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := readOneFrame(conn)
	conn.SetReadDeadline(time.Time{})
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

func sendMessage(text string) {
	aead, err := chacha20poly1305.New(sharedKey)
	if err != nil {
		fmt.Println("❌ Ошибка ключа:", err)
		return
	}
	// Counter-based nonce: 8-byte LE counter + 4 zero bytes = 12 bytes
	cnt := sendCounter.Add(1)
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce[:8], cnt)
	ct := aead.Seal(nil, nonce, []byte(text), nil)

	// msgPayload = [Counter(8 LE)][Ciphertext(N+16)]
	msgPayload := make([]byte, 8+len(ct))
	copy(msgPayload[:8], nonce[:8])
	copy(msgPayload[8:], ct)

	maxFrame := cfg.MaxFrameSize
	if maxFrame < 20 {
		maxFrame = 180
	}

	// Frame = 2 (length prefix) + 1 (cmd) + len(msgPayload)
	if 3+len(msgPayload) <= maxFrame {
		writeFrame(conn, CmdMsg, msgPayload)
		return
	}

	// Message too large for one frame — fragment it.
	// Each CmdFragment frame: 2 (len) + 1 (cmd) + 3 (msgID, fragIdx, fragCount) + chunk
	msgID := byte(fragCounter.Add(1))
	maxChunk := maxFrame - 6
	if maxChunk < 1 {
		maxChunk = 100
	}

	var chunks [][]byte
	remaining := msgPayload
	for len(remaining) > 0 {
		end := maxChunk
		if end > len(remaining) {
			end = len(remaining)
		}
		chunks = append(chunks, remaining[:end])
		remaining = remaining[end:]
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

func readLoop(loginDone chan bool, historyDone chan struct{}) {
	// Accumulating buffer: handles TCP fragmentation common in DNS tunnels.
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
				// Message delivery acknowledgment — nothing to display.

			case CmdHistory:
				// Payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][Content...]
				// Content length is implicit (payload length minus fixed prefix).
				// Minimum: 1 (SenderLen) + 4 (Timestamp) = 5 bytes.
				if len(payload) < 5 {
					continue
				}
				senderLen := int(payload[0])
				if len(payload) < 1+senderLen+4 {
					continue
				}
				sender := string(payload[1 : 1+senderLen])
				tsOff := 1 + senderLen
				ts := uint32(payload[tsOff])<<24 | uint32(payload[tsOff+1])<<16 |
					uint32(payload[tsOff+2])<<8 | uint32(payload[tsOff+3])
				content := string(payload[tsOff+4:])
				timeStr := time.Unix(int64(ts), 0).Local().Format("2006-01-02 15:04")
				fmt.Printf("  [%s] %s: %s\n", timeStr, sender, content)

			case CmdHistoryEnd:
				if !historyFinished {
					historyFinished = true
					close(historyDone)
				}

			case CmdIncoming:
				// Payload: [SenderSID(2 BE)][Counter(8 LE)][Ciphertext(N+16)]
				if len(payload) < 2+8+16 {
					continue
				}
				senderSID := uint16(payload[0])<<8 | uint16(payload[1])
				nonce := make([]byte, 12)
				copy(nonce[:8], payload[2:10])
				ciphertext := payload[10:]
				if plaintext, decErr := decryptMsg(ciphertext, nonce); decErr == nil {
					senderName := sidNames[senderSID]
					if senderName == "" {
						senderName = fmt.Sprintf("SID:%d", senderSID)
					}
					fmt.Printf("\n📨 [%s]: %s\n>> ", senderName, string(plaintext))
				}

			case CmdOnlineList:
				// Payload: [Count(1)][SID(2 BE)][NameLen(1)][Name]...
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
					fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(names), strings.Join(names, ", "))
				}

			case CmdOnlineAdd:
				// Payload: [SID(2 BE)][NameLen(1)][Name]
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
				allNames := make([]string, 0, len(sidNames))
				for _, n := range sidNames {
					allNames = append(allNames, n)
				}
				fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(allNames), strings.Join(allNames, ", "))

			case CmdOnlineRemove:
				// Payload: [SID(2 BE)]
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
				// Payload: [Count(1)][AddrLen(1)][Addr(N)]...
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
			}
		}
	}
}

func decryptMsg(ciphertext, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(sharedKey)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
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
		json.NewDecoder(file).Decode(&cfg)
	} else {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
	}
}
