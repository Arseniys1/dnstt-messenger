package main

import (
	"bufio"
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
	SharedKey  string `json:"shared_key"`
}

var (
	cfg       Config
	sessionID uint16
	conn      net.Conn
)

const (
	CmdRegister   = 0x01
	CmdLogin      = 0x02
	CmdMsg        = 0x03
	CmdIncoming   = 0x04
	CmdHistory    = 0x05
	CmdHistoryEnd = 0x07
)

func main() {
	loadConfig("client_config.json")

	// Validate key length for chacha20poly1305 (must be 32 bytes)
	if len(cfg.SharedKey) != 32 {
		fmt.Printf("❌ shared_key должен быть ровно 32 символа (сейчас %d)\n", len(cfg.SharedKey))
		return
	}

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

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== DNS Messenger Client ===")

	for sessionID == 0 {
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
			if register(login, pass) {
				fmt.Println("✨ Аккаунт создан! Теперь войдите.")
			}
		} else {
			loginUser(login, pass)
			if sessionID == 0 {
				fmt.Println("❌ Неверный логин или пароль.")
			}
		}
	}

	// Получаем историю до открытия чата
	fmt.Println("\n--- История чата ---")
	historyDone := make(chan struct{})
	go readLoop(historyDone)
	<-historyDone
	fmt.Println("--- Конец истории ---\n")

	fmt.Println("✅ Авторизация успешна! (/exit для выхода)")

	for {
		fmt.Print(">> ")
		text, _ := reader.ReadString('\n')
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

// --- СЕТЕВАЯ ЛОГИКА ---

func register(login, pass string) bool {
	if len(login) > 255 || len(pass) > 255 {
		fmt.Println("❌ Слишком длинные данные.")
		return false
	}
	packet := []byte{CmdRegister, byte(len(login))}
	packet = append(packet, []byte(login)...)
	packet = append(packet, []byte(pass)...)

	conn.Write(packet)

	res := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.Read(res)
	conn.SetReadDeadline(time.Time{})

	if res[0] == 0x01 {
		return true
	}
	fmt.Println("❌ Логин уже занят.")
	return false
}

func loginUser(login, pass string) {
	packet := []byte{CmdLogin, byte(len(login))}
	packet = append(packet, []byte(login)...)
	packet = append(packet, []byte(pass)...)

	conn.Write(packet)

	res := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _ := conn.Read(res)
	conn.SetReadDeadline(time.Time{})

	if n == 2 && (res[0] != 0 || res[1] != 0) {
		sessionID = uint16(res[0])<<8 | uint16(res[1])
	}
}

func sendMessage(text string) {
	aead, err := chacha20poly1305.New([]byte(cfg.SharedKey))
	if err != nil {
		fmt.Println("❌ Ошибка ключа:", err)
		return
	}

	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)

	ciphertext := aead.Seal(nil, nonce, []byte(text), nil)

	// [CMD(1)][SID(2)][Nonce(12)][Ciphertext]
	packet := []byte{CmdMsg, byte(sessionID >> 8), byte(sessionID & 0xFF)}
	packet = append(packet, nonce...)
	packet = append(packet, ciphertext...)

	_, err = conn.Write(packet)
	if err != nil {
		fmt.Println("❌ Связь разорвана:", err)
		os.Exit(1)
	}
}

func readLoop(historyDone chan struct{}) {
	buf := make([]byte, 2048)
	historyFinished := false

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("\n📡 Соединение закрыто сервером.")
			os.Exit(0)
		}
		if n < 1 {
			continue
		}

		switch buf[0] {

		case 0x06:
			// ACK — игнорируем

		case CmdHistory:
			if n < 5 {
				continue
			}
			senderLen := int(buf[1])
			off := 2 + senderLen
			if n < off+1 {
				continue
			}
			sender := string(buf[2:off])
			timeLen := int(buf[off])
			off++
			if n < off+timeLen+2 {
				continue
			}
			timeStr := string(buf[off : off+timeLen])
			off += timeLen
			msgLen := int(buf[off])<<8 | int(buf[off+1])
			off += 2
			if n < off+msgLen {
				continue
			}
			text := string(buf[off : off+msgLen])
			fmt.Printf("  [%s] %s: %s\n", timeStr, sender, text)

		case CmdHistoryEnd:
			if !historyFinished {
				historyFinished = true
				close(historyDone)
			}

		case CmdIncoming:
			if n < 16 {
				continue
			}
			senderLen := int(buf[1])
			if n < 2+senderLen+12+1 {
				continue
			}
			sender := string(buf[2 : 2+senderLen])
			nonce := buf[2+senderLen : 2+senderLen+12]
			ciphertext := buf[2+senderLen+12 : n]

			plaintext, err := decryptMsg(ciphertext, nonce)
			if err != nil {
				continue
			}
			fmt.Printf("\n📨 [%s]: %s\n>> ", sender, string(plaintext))
		}
	}
}

func decryptMsg(ciphertext, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New([]byte(cfg.SharedKey))
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
		SharedKey:  "12345678901234567890123456789012",
	}

	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		json.NewDecoder(file).Decode(&cfg)
	} else {
		fmt.Println("⚠️ Конфиг не найден, использую настройки по умолчанию.")
	}
}
