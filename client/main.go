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
	ProxyAddr  string `json:"proxy_addr"`
	ServerAddr string `json:"server_addr"`
	DirectMode bool   `json:"direct_mode"`
}

var (
	cfg         Config
	sessionID   uint16
	conn        net.Conn
	sharedKey   []byte
	sendCounter atomic.Uint64
	recvCounter uint64
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
)

func main() {
	loadConfig("client_config.json")

	var err error
	if cfg.DirectMode {
		fmt.Printf("🌐 Direct Connect -> %s...\n", cfg.ServerAddr)
		conn, err = net.DialTimeout("tcp", cfg.ServerAddr, 10*time.Second)
	} else {
		fmt.Printf("🌐 SOCKS5 %s -> %s...\n", cfg.ProxyAddr, cfg.ServerAddr)
		baseDialer := &net.Dialer{Timeout: 10 * time.Second}
		socks5Dialer, dialErr := proxy.SOCKS5("tcp", cfg.ProxyAddr, nil, baseDialer)
		if dialErr != nil {
			fmt.Printf("❌ SOCKS5 dialer: %v\n", dialErr)
			return
		}
		conn, err = socks5Dialer.Dial("tcp", cfg.ServerAddr)
	}
	if err != nil {
		fmt.Printf("❌ Подключение: %v\n", err)
		return
	}
	defer conn.Close()

	sharedKey, err = ecdhHandshake(conn)
	if err != nil {
		fmt.Printf("❌ ECDH: %v\n", err)
		return
	}
	fmt.Println("🔐 Защищённый канал установлен.")

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n=== DNS Messenger ===")

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

func ecdhHandshake(conn net.Conn) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	serverPubBytes := make([]byte, 32)
	if _, err = readFull(conn, serverPubBytes); err != nil {
		return nil, err
	}
	serverPub, err := curve.NewPublicKey(serverPubBytes)
	if err != nil {
		return nil, err
	}
	if _, err = conn.Write(privKey.PublicKey().Bytes()); err != nil {
		return nil, err
	}
	return privKey.ECDH(serverPub)
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

	// nonce: 8-байтный счётчик (little-endian) + 4 нулевых байта
	cntVal := sendCounter.Add(1)
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce[:8], cntVal)

	ct := aead.Seal(nil, nonce, []byte(text), nil)

	// [CmdMsg(1)][nonce(8)][ciphertext]
	packet := []byte{CmdMsg}
	packet = append(packet, nonce[:8]...)
	packet = append(packet, ct...)

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
			fmt.Println("\n📡 Соединение закрыто.")
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
				// [CmdLoginOK(1)][SID(2)]
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

			case CmdHistory:
				// [CmdHistory(1)][senderLen(1)][sender][ts uint32 BE(4)][nonce(8)][ciphertext]
				if len(data) < 15 {
					data = nil
					continue
				}
				senderLen := int(data[1])
				off := 2 + senderLen
				if len(data) < off+4+8+1 {
					data = nil
					continue
				}
				sender := string(data[2:off])
				ts := binary.BigEndian.Uint32(data[off : off+4])
				off += 4
				nonce := make([]byte, 12)
				copy(nonce[:8], data[off:off+8])
				off += 8
				ciphertext := data[off:]

				// защита от replay
				if uint64(ts) > recvCounter {
					recvCounter = uint64(ts)
				}

				plaintext, decErr := decryptMsg(ciphertext, nonce)
				if decErr == nil {
					t := time.Unix(int64(ts), 0).Format("2006-01-02 15:04")
					fmt.Printf("  [%s] %s: %s\n", t, sender, string(plaintext))
				}
				data = nil // ciphertext до конца пакета

			case CmdHistoryEnd:
				if !historyFinished {
					historyFinished = true
					close(historyDone)
				}
				data = data[1:]

			case CmdIncoming:
				// [CmdIncoming(1)][senderLen(1)][sender][nonce(8)][ciphertext]
				if len(data) < 11 {
					data = nil
					continue
				}
				senderLen := int(data[1])
				off := 2 + senderLen
				if len(data) < off+8+1 {
					data = nil
					continue
				}
				sender := string(data[2 : 2+senderLen])
				nonce := make([]byte, 12)
				copy(nonce[:8], data[off:off+8])
				off += 8
				ciphertext := data[off:]

				if plaintext, decErr := decryptMsg(ciphertext, nonce); decErr == nil {
					fmt.Printf("\n📨 [%s]: %s\n>> ", sender, string(plaintext))
				}
				data = nil

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
