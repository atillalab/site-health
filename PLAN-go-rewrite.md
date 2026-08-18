# site-health: Go Rewrite Plan

**Dil:** Go | **Bağımlılık:** Sıfır (stdlib only) | **Hedef:** Tek binary, cross-platform

## Proje Yapısı

```
site-health/
├── go.mod
├── main.go                         # entry point, flag parsing, orchestration
├── internal/
│   ├── check/
│   │   ├── check.go                # Status, Report, Runner, Issue tracking
│   │   ├── dns.go                  # A/AAAA/CNAME → net.Resolver
│   │   ├── tcp.go                  # TCP 80/443 → net.DialTimeout
│   │   ├── http.go                 # HTTP probe + redirect → net/http
│   │   ├── ssl.go                  # TLS cert → crypto/tls + x509
│   │   ├── content.go              # HTML validation + error/parked patterns
│   │   ├── whois.go                # Domain registration → raw TCP
│   │   └── mail.go                 # MX/SPF/DMARC → net.LookupMX/TXT
│   ├── output/
│   │   ├── output.go               # Renderer interface, ANSI colors
│   │   ├── dashboard.go            # Terminal dashboard
│   │   └── json.go                 # encoding/json
│   └── domain/
│       └── domain.go               # URL normalization, same-site host check
```

## Bash → Go Haritalaması

| Bash fonksiyonu | Go karşılığı | Stdlib API |
|---|---|---|
| `dig +short A/AAAA` | `dns.Check()` | `net.Resolver.LookupIPAddr` |
| `dig +short CNAME www` | `dns.Check()` | `net.LookupCNAME` |
| `dig +short MX/TXT` | `mail.CheckMX/SPF/DMARC()` | `net.LookupMX`, `net.LookupTXT` |
| `nc -z domain 80/443` | `tcp.Check()` | `net.DialTimeout` |
| `curl --write-out` | `http.Check()` | `http.Client` + custom Transport |
| `openssl s_client + x509` | `ssl.Check()` | `tls.Dial` + `x509.Certificate` |
| `whois` binary | `whois.CheckRegistration()` | Raw TCP `net.Dial("tcp", server+":43")` |
| `grep -Eiq` (pattern matching) | `content.Check()` | `regexp.Compile` + `bytes.Contains` |
| `jq` / manual JSON | `json.Render()` | `encoding/json` |

**Elenen bağımlılıklar:** curl, dig, nc, openssl, whois, awk, sed, grep, mktemp

## Veri Modeli (temel struct'lar)

```go
type Status int  // OK=0, WARN=1, FAIL=2

type Report struct {
    Tool, Version, Domain, Mode, ExpectedURL string
    Forwarding  Forwarding
    Checks      Checks
    Issues      []Issue
    Summary     Summary
}

type Checks struct {
    DNS, HTTPS, SSL, Redirect, Response,
    DomainRegistration, Mail  (her biri ayrı result struct)
}
```

JSON çıktısı mevcut Bash versiyonuyla birebir uyumlu olacak.

## Eşzamanlılık (Go'nun en büyük avantajı)

Site modunda tüm kontroller **goroutine** ile paralel çalışır:

```go
go whois.CheckRegistration()  // ─┐
go dns.Check()                //  │
go tcp.Check()                //  ├── hepsi eşzamanlı
go http.Check()               //  │
go ssl.Check()                //  │
go content.Check()            //  │
go mail.CheckAll()            // ─┘
wg.Wait()
```

Bash'te sıralı çalışan kontroller Go'da ~3-5x hızlanacak.

## Uygulama Sırası

1. **Proje iskeleti** — go.mod, main.go, paket yapısı
2. **Domain + Status modelleri** — temel tipler
3. **DNS checks** — en basit, test edilmesi kolay
4. **TCP checks** — tek satır `net.DialTimeout`
5. **HTTP probe** — redirect detection + forwarding auto-detect
6. **SSL/TLS** — `crypto/tls` handshake
7. **WHOIS** — raw TCP + parser (en karmaşık kısım)
8. **Mail checks** — MX/SPF/DMARC
9. **Content validation** — pattern matching
10. **Çıktı katmanı** — dashboard + JSON renderer
11. **Testler**
12. **README güncelleme**

## Tasarım Kararları

- **WHOIS:** TLD'yi whois.iana.org:43'ten öğren → yetkili sunucuya bağlan → raw TCP oku → regex ile parse
- **SSL:** `tls.Dial` → `PeerCertificates[0]` → `x509.Certificate` doğrudan erişilebilir
- **DNS:** Go resolver `/etc/resolv.conf` (Unix) veya Windows DNS API kullanır — sıfır yapılandırma
- **HTTP:** `CheckRedirect` ile max 10 redirect, `time.Since` ile yanıt süresi ölçümü
- **Content:** Bash'teki aynı regex pattern'ları `regexp.Compile` ile compile edilir
- **Çıkış kodları:** 0=sağlıklı, 1=sorunlu, 2=kullanım hatası (bash ile aynı)
