# DNSTT Messenger (Türkçe)

DNSTT Messenger, DNS tünelleme (MasterDnsVPN) üzerinden çalışabilen, uçtan uca şifreli (E2E) bir mesajlaşma uygulamasıdır.

## Hızlı Başlangıç (İstemci)

1. MasterDnsVPN istemcisini indirip çalıştırın.
2. `client_config.toml` dosyasını yapılandırın (alan adı ve şifreleme anahtarını sunucu yöneticisinden alın).
3. MasterDnsVPN’i açık tutun.
4. DNSTT Messenger istemcisini başlatın.
5. Uygulama içinden **Settings** bölümünü açın, sunucu/proxy/dil ayarlarını kaydedin.
6. Kayıt olun veya giriş yapın.

## Kendi Sunucunuzu Kurma

1. Tünel alan adı için DNS kayıtlarını ayarlayın.
2. MasterDnsVPN sunucusunu çalıştırın.
3. Mesaj sunucusunu (`server`) `config.json` ile çalıştırın.
4. Kullanıcılara şu bağlantı bilgilerini verin:
   - `server_addr`
   - tünel alan adı
   - tünel şifreleme anahtarı

## Derleme

- Go istemci/sunucu: `go build`
- Electron istemci: `npm install && npm start` (veya `npm run build:*`)
- Android istemci: `./gradlew assembleRelease` (veya Android Studio)

## Güvenlik Notları

- MasterDnsVPN şifreleme anahtarını gizli tutun.
- Güçlü hesap parolaları kullanın.
- Güvenilir sunucu altyapısı kullanın.
- Sızıntı şüphesinde anahtarları hemen döndürün.
