package main

import (
	"bufio"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/net/proxy"
)

type Config struct {
	ProxyAddr  string `json:"proxy_addr"`
	ServerAddr string `json:"server_addr"`
	DirectMode bool   `json:"direct_mode"`
}

var (
	cfg       Config
	sessionID uint16
	conn      net.Conn
	sharedKey []byte
)

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

	// ECDH хендшейк — получаем общий ключ
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

	fmt.Println("✅ Авторизация успешна! (/exit для выхода)")

	reader2 := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		text, _ := reader2.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "/exit" {
			break
		}
		if text == "" {
			continue
		}
		sendMessage(text)
	}
}

// ecdhHandshake выполняет X25519 хендшейк.
// Сервер первым присылает свой публичный ключ (32 байта), клиент отвечает своим.
func ecdhHandshake(conn net.Conn) ([]byte, error) {
	curve := ecdh.X25519()

	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("генерация ключа: %w", err)
	}

	// Читаем публичный ключ сервера
	serverPubBytes := make([]byte, 32)
	if _, err = readFull(conn, serverPubBytes); err != nil {
		return nil, fmt.Errorf("чтение публичного ключа сервера: %w", err)
	}

	serverPub, err := curve.NewPublicKey(serverPubBytes)
	if err != nil {
		return nil, fmt.Errorf("парсинг публичного ключа сервера: %w", err)
	}

	// Отправляем свой публичный ключ
	if _, err = conn.Write(privKey.PublicKey().Bytes()); err != nil {
		return nil, fmt.Errorf("отправка публичного ключа: %w", err)
	}

	shared, err := privKey.ECDH(serverPub)
	if err != nil {
		return nil, fmt.Errorf("вычисление общего секрета: %w", err)
	}

	return shared, nil
}

// readFull читает ровно len(buf) байт.
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

// --- СЕТЕВАЯ ЛОГИКА ---

func register(login, pass string) (bool, error) {
	if len(login) > 255 || len(pass) > 255 {
		return false, fmt.Errorf("слишком длинные данные")
	}
	packet := []byte{CmdRegister, byte(len(login))}
	packet = append(packet, []byte(login)...)
	packet = append(packet, []byte(pass)...)
	conn.Write(packet)

	res := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Read(res)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return false, err
	}
	return res[0] == 0x01, nil
}

func sendLoginPacket(login, pass string) {
	packet := []byte{CmdLogin, byte(len(login))}
	packet = append(packet, []byte(login)...)
	packet = append(packet, []byte(pass)...)
	conn.Write(packet)
}

func sendMessage(text string) {
	aead, err := chacha20poly1305.New(sharedKey)
	if err != nil {
		fmt.Println("❌ Ошибка ключа:", err)
		return
	}
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)
	ciphertext := aead.Seal(nil, nonce, []byte(text), nil)

	packet := []byte{CmdMsg, byte(sessionID >> 8), byte(sessionID & 0xFF)}
	packet = append(packet, nonce...)
	packet = append(packet, ciphertext...)

	if _, err = conn.Write(packet); err != nil {
		fmt.Println("❌ Связь разорвана:", err)
		os.Exit(1)
	}
}

func readLoop(loginDone chan bool, historyDone chan struct{}) {
	buf := make([]byte, 4096)
	historyFinished := false
	loginHandled := false

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("\n📡 Соединение закрыто сервером.")
			os.Exit(0)
		}
		if n < 1 {
			continue
		}

		data := buf[:n]
		for len(data) > 0 {
			cmd := data[0]
			switch cmd {

			case CmdLoginOK:
				if len(data) < 3 {
					data = nil
					continue
				}
				sessionID = uint16(data[1])<<8 | uint16(data[2])
				if !loginHandled {
					loginHandled = true
					loginDone <- true
				}
				data = data[3:]

			case CmdLoginFail:
				if !loginHandled {
					loginHandled = true
					loginDone <- false
				}
				data = data[1:]

			case 0x06:
				data = data[1:]

			case CmdHistory:
				if len(data) < 5 {
					data = nil
					continue
				}
				senderLen := int(data[1])
				off := 2 + senderLen
				if len(data) < off+1 {
					data = nil
					continue
				}
				sender := string(data[2:off])
				timeLen := int(data[off])
				off++
				if len(data) < off+timeLen+2 {
					data = nil
					continue
				}
				timeStr := string(data[off : off+timeLen])
				off += timeLen
				msgLen := int(data[off])<<8 | int(data[off+1])
				off += 2
				if len(data) < off+msgLen {
					data = nil
					continue
				}
				text := string(data[off : off+msgLen])
				fmt.Printf("  [%s] %s: %s\n", timeStr, sender, text)
				data = data[off+msgLen:]

			case CmdHistoryEnd:
				if !historyFinished {
					historyFinished = true
					close(historyDone)
				}
				data = data[1:]

			case CmdIncoming:
				if len(data) < 16 {
					data = nil
					continue
				}
				senderLen := int(data[1])
				if len(data) < 2+senderLen+12+1 {
					data = nil
					continue
				}
				sender := string(data[2 : 2+senderLen])
				nonce := data[2+senderLen : 2+senderLen+12]
				ciphertext := data[2+senderLen+12:]
				if plaintext, err := decryptMsg(ciphertext, nonce); err == nil {
					fmt.Printf("\n📨 [%s]: %s\n>> ", sender, string(plaintext))
				}
				data = nil

			case CmdOnlineList:
				if len(data) < 2 {
					data = nil
					continue
				}
				count := int(data[1])
				off := 2
				names := make([]string, 0, count)
				valid := true
				for i := 0; i < count; i++ {
					if off >= len(data) {
						valid = false
						break
					}
					nLen := int(data[off])
					off++
					if off+nLen > len(data) {
						valid = false
						break
					}
					names = append(names, string(data[off:off+nLen]))
					off += nLen
				}
				if valid {
					fmt.Printf("\n🟢 Онлайн (%d): %s\n>> ", len(names), strings.Join(names, ", "))
				}
				data = data[off:]

			default:
				data = data[1:]
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
		ProxyAddr:  "127.0.0.1:8080",
		ServerAddr: "127.0.0.1:9999",
		DirectMode: false,
	}
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		json.NewDecoder(file).Decode(&cfg)
	} else {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
	}
}
