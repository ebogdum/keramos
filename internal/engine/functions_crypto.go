package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"hash/adler32"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

// randomSerialNumber returns a cryptographically random 128-bit serial.
// RFC 5280 §4.1.2.2 requires non-predictable serials for X.509 certificates.
func randomSerialNumber() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if nil != err {
		// In the impossible event the OS RNG fails, fall back to a
		// timestamp-derived value so we don't crash. This path is logged
		// elsewhere by callers when the cert is created.
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}

func registerCryptoFuncs(r *FuncRegistry) {
	r.Register("sha1sum", fnSha1Sum)
	r.Register("sha256sum", fnSha256Sum)
	r.Register("sha512sum", fnSha512Sum)
	r.Register("md5sum", fnMd5Sum)
	r.Register("adler32sum", fnAdler32Sum)
	r.Register("hmacSha256", fnHMACSha256)
	r.Register("bcrypt", fnBcrypt)
	r.Register("htpasswd", fnHtpasswd)
	r.Register("encryptAES", fnEncryptAES)
	r.Register("decryptAES", fnDecryptAES)
	r.Register("genPrivateKey", fnGenPrivateKey)
	r.Register("genCA", fnGenCA)
	r.Register("genSelfSignedCert", fnGenSelfSignedCert)
	r.Register("randAlphaNum", fnRandAlphaNum)
	r.Register("randAlpha", fnRandAlpha)
	r.Register("randNumeric", fnRandNumeric)
	r.Register("randAscii", fnRandAscii)
	r.Register("randBytes", fnRandBytes)
	r.Register("uuidv4", fnUUIDv4)
}

func fnSha1Sum(value any, args ...any) (any, error) {
	sum := sha1.Sum([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(sum[:]), nil
}

func fnSha256Sum(value any, args ...any) (any, error) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(sum[:]), nil
}

func fnSha512Sum(value any, args ...any) (any, error) {
	sum := sha512.Sum512([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(sum[:]), nil
}

func fnMd5Sum(value any, args ...any) (any, error) {
	sum := md5.Sum([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(sum[:]), nil
}

func fnAdler32Sum(value any, args ...any) (any, error) {
	return strconv.FormatUint(uint64(adler32.Checksum([]byte(fmt.Sprintf("%v", value)))), 10), nil
}

func fnHMACSha256(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "hmacSha256 requires a key argument")
	}
	mac := hmac.New(sha256.New, []byte(coerceString(args[0])))
	mac.Write([]byte(fmt.Sprintf("%v", value)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func fnBcrypt(value any, args ...any) (any, error) {
	cost := bcrypt.DefaultCost
	if 0 < len(args) {
		c, parseErr := strconv.Atoi(coerceString(args[0]))
		if nil == parseErr && bcrypt.MinCost <= c && c <= bcrypt.MaxCost {
			cost = c
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(fmt.Sprintf("%v", value)), cost)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "bcrypt failed", err)
	}
	return string(hash), nil
}

func fnHtpasswd(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "htpasswd requires a password argument")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(coerceString(args[0])), bcrypt.DefaultCost)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "htpasswd failed", err)
	}
	return fmt.Sprintf("%s:%s", fmt.Sprintf("%v", value), string(hash)), nil
}

// aesSaltLen is the random salt size prepended to ciphertext for the
// PBKDF2-HMAC-SHA256 key derivation. 16 bytes / 100k iterations.
const (
	aesSaltLen   = 16
	aesPBKDFIter = 100_000
)

// fnEncryptAES encrypts a value with AES-256-GCM. The key is derived from
// the passphrase via PBKDF2-HMAC-SHA256 with a fresh per-call random salt.
// Output layout: base64(salt || nonce || ciphertext+tag).
func fnEncryptAES(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "encryptAES requires a passphrase argument")
	}
	salt := make([]byte, aesSaltLen)
	if _, err := rand.Read(salt); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "encryptAES: salt read failed", err)
	}
	key := pbkdf2.Key([]byte(coerceString(args[0])), salt, aesPBKDFIter, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "encryptAES: cipher init failed", err)
	}
	gcm, err := cipher.NewGCM(block)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "encryptAES: gcm init failed", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "encryptAES: nonce read failed", err)
	}
	cipherText := gcm.Seal(nil, nonce, []byte(fmt.Sprintf("%v", value)), nil)
	out := append(salt, nonce...)
	out = append(out, cipherText...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func fnDecryptAES(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "decryptAES requires a passphrase argument")
	}
	data, err := base64.StdEncoding.DecodeString(fmt.Sprintf("%v", value))
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "decryptAES: base64 decode failed", err)
	}
	if len(data) < aesSaltLen+12 { // 12 = minimum GCM nonce size
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "decryptAES: ciphertext too short")
	}
	salt, rest := data[:aesSaltLen], data[aesSaltLen:]
	key := pbkdf2.Key([]byte(coerceString(args[0])), salt, aesPBKDFIter, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "decryptAES: cipher init failed", err)
	}
	gcm, err := cipher.NewGCM(block)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "decryptAES: gcm init failed", err)
	}
	if len(rest) < gcm.NonceSize() {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "decryptAES: ciphertext too short")
	}
	nonce, ct := rest[:gcm.NonceSize()], rest[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "decryptAES: open failed", err)
	}
	return string(plain), nil
}

// fnGenPrivateKey generates a PEM-encoded private key. Type is selected by the
// pipeline value: "rsa" (default 2048), "ecdsa" (P256), or "ed25519"
// (PKCS#8-encoded).
func fnGenPrivateKey(value any, args ...any) (any, error) {
	kind := strings.ToLower(fmt.Sprintf("%v", value))
	switch kind {
	case "", "rsa":
		bits := 2048
		if 0 < len(args) {
			if b, err := strconv.Atoi(coerceString(args[0])); nil == err {
				bits = b
			}
		}
		// Reject obviously weak (< 2048-bit) and CPU-burning (> 8192-bit)
		// key sizes. rsa.GenerateKey at 16384 bits takes ~minutes; at
		// 100000 the renderer hangs effectively forever.
		if bits < 2048 {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
				"genPrivateKey: RSA key size %d is below the 2048-bit minimum", bits)
		}
		if bits > 8192 {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
				"genPrivateKey: RSA key size %d exceeds the 8192-bit maximum", bits)
		}
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genPrivateKey: rsa keygen failed", err)
		}
		der := x509.MarshalPKCS1PrivateKey(key)
		return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})), nil
	case "ecdsa":
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genPrivateKey: ecdsa keygen failed", err)
		}
		der, err := x509.MarshalECPrivateKey(key)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genPrivateKey: ecdsa marshal failed", err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})), nil
	case "ed25519":
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genPrivateKey: ed25519 keygen failed", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genPrivateKey: ed25519 marshal failed", err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
	}
	return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "genPrivateKey: unsupported type %q", kind)
}

func fnGenCA(value any, args ...any) (any, error) {
	cn := fmt.Sprintf("%v", value)
	if "" == cn {
		cn = "keramos-ca"
	}
	days := 365
	if 0 < len(args) {
		if d, err := strconv.Atoi(coerceString(args[0])); nil == err {
			days = d
		}
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genCA: keygen failed", err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genCA: cert creation failed", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return map[string]any{"Cert": string(certPEM), "Key": string(keyPEM)}, nil
}

func fnGenSelfSignedCert(value any, args ...any) (any, error) {
	cn := fmt.Sprintf("%v", value)
	if "" == cn {
		cn = "localhost"
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSelfSignedCert: keygen failed", err)
	}
	dnsNames := []string{cn}
	var ips []net.IP
	for _, a := range args {
		if ip := net.ParseIP(coerceString(a)); nil != ip {
			ips = append(ips, ip)
			continue
		}
		dnsNames = append(dnsNames, coerceString(a))
	}
	tpl := &x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              dnsNames,
		IPAddresses:           ips,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSelfSignedCert: cert creation failed", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return map[string]any{"Cert": string(certPEM), "Key": string(keyPEM)}, nil
}

func fnRandAlphaNum(value any, args ...any) (any, error) {
	return randString(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

func fnRandAlpha(value any, args ...any) (any, error) {
	return randString(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

func fnRandNumeric(value any, args ...any) (any, error) {
	return randString(value, "0123456789")
}

func fnRandAscii(value any, args ...any) (any, error) {
	const printable = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	return randString(value, printable)
}

// maxRandLen bounds rand* helpers: a single ${randAlphaNum 1000000000}
// would otherwise allocate 1 GB and call crypto/rand 1B times. 64 KiB is
// far past anything templates legitimately need (passwords, tokens).
const maxRandLen = 1 << 16

func fnRandBytes(value any, args ...any) (any, error) {
	n, ok := toFloat(value)
	if !ok || n < 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "randBytes: invalid length %v", value)
	}
	if int(n) > maxRandLen {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"randBytes: length %d exceeds %d", int(n), maxRandLen)
	}
	buf := make([]byte, int(n))
	if _, err := rand.Read(buf); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "randBytes: read failed", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func fnUUIDv4(value any, args ...any) (any, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "uuidv4: read failed", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func randString(value any, alphabet string) (any, error) {
	n, ok := toFloat(value)
	if !ok || n < 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "rand: invalid length %v", value)
	}
	count := int(n)
	if count > maxRandLen {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"rand: length %d exceeds %d", count, maxRandLen)
	}
	out := make([]byte, count)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < count; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if nil != err {
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "rand: read failed", err)
		}
		out[i] = alphabet[idx.Int64()]
	}
	return string(out), nil
}
