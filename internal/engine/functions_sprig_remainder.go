package engine

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	keramoserrors "github.com/ebogdum/keramos/v2/internal/errors"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

// registerSprigRemainder finishes Sprig parity by adding the previously-missing
// functions the audit identified: derivePassword, genSignedCert, htmlDate,
// dateModify, addf/mulf/subf/divf (float-forced math), deepCopy, deepEqual,
// chunk, and mustChunk.
func registerSprigRemainder(r *FuncRegistry) {
	r.Register("derivePassword", fnDerivePassword)
	r.Register("genSignedCert", fnGenSignedCert)
	r.Register("htmlDate", fnHTMLDate)
	r.Register("htmlDateInZone", fnHTMLDateInZone)
	r.Register("dateModify", fnDateModify)
	r.Register("mustDateModify", fnDateModify)
	r.Register("addf", fnAddf)
	r.Register("subf", fnSubf)
	r.Register("mulf", fnMulf)
	r.Register("divf", fnDivf)
	r.Register("deepCopy", fnDeepCopy)
	r.Register("mustDeepCopy", fnDeepCopy)
	r.Register("deepEqual", fnDeepEqual)
	r.Register("chunk", fnChunk)
	r.Register("mustChunk", fnChunk)
}

// fnDerivePassword implements Sprig's derivePassword: PBKDF2 over a counter,
// site, and master password. Keramos's variant uses SHA-512 with 100k rounds.
//
// Pipeline form: ${counter | derivePassword "long" "site" "user" "master"}.
// Args: passwordType, site, user, master.
func fnDerivePassword(value any, args ...any) (any, error) {
	if 4 != len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction,
			"derivePassword requires 4 args: passwordType, site, user, master")
	}
	counter, ok := coerceInt(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"derivePassword: counter must be integer, got %T", value)
	}
	passwordType := coerceString(args[0])
	site := coerceString(args[1])
	user := coerceString(args[2])
	master := coerceString(args[3])

	salt := fmt.Sprintf("com.lyndir.masterpassword%s%d", site, counter)
	key := pbkdf2.Key([]byte(master), []byte(user+salt), 100_000, 64, sha3.New512)

	template, ok := derivePasswordTemplates[passwordType]
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"derivePassword: unknown passwordType %q (want long/medium/short/basic/pin)", passwordType)
	}
	out := make([]byte, len(template))
	for i, c := range []byte(template) {
		alphabet, ok := derivePasswordAlphabets[c]
		if !ok {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
				"derivePassword: unknown template glyph %q", string(c))
		}
		out[i] = alphabet[int(key[i+1])%len(alphabet)]
	}
	return string(out), nil
}

var derivePasswordTemplates = map[string]string{
	"long":   "CvcvnoCvcvCvcv",
	"medium": "CvcnoCvc",
	"short":  "Cvcn",
	"basic":  "aaanaaan",
	"pin":    "nnnn",
}

var derivePasswordAlphabets = map[byte]string{
	'V': "AEIOU",
	'C': "BCDFGHJKLMNPQRSTVWXYZ",
	'v': "aeiou",
	'c': "bcdfghjklmnpqrstvwxyz",
	'A': "AEIOUBCDFGHJKLMNPQRSTVWXYZ",
	'a': "AEIOUaeiouBCDFGHJKLMNPQRSTVWXYZbcdfghjklmnpqrstvwxyz",
	'n': "0123456789",
	'o': "@&%?,=[]_:-+*$#!'^~;()/.",
	'x': "AEIOUaeiouBCDFGHJKLMNPQRSTVWXYZbcdfghjklmnpqrstvwxyz0123456789!@#$%^&*()",
}

// fnGenSignedCert produces a PEM-encoded cert signed by a CA's private key.
//
// Pipeline form: ${commonName | genSignedCert ipsCSV namesCSV daysInt caCertPEM caKeyPEM}.
//
// Returns {Cert: <PEM>, Key: <PEM>}.
func fnGenSignedCert(value any, args ...any) (any, error) {
	if 5 != len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction,
			"genSignedCert requires 5 args: ipsCSV, dnsCSV, days, caCertPEM, caKeyPEM")
	}
	cn := coerceString(value)
	ipsCSV := coerceString(args[0])
	dnsCSV := coerceString(args[1])
	days, ok := coerceInt(args[2])
	if !ok {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "genSignedCert: days must be integer")
	}
	caCertPEM := coerceString(args[3])
	caKeyPEM := coerceString(args[4])

	caCert, err := parseCertPEM(caCertPEM)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSignedCert: invalid CA cert", err)
	}
	caKey, err := parseRSAKeyPEM(caKeyPEM)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSignedCert: invalid CA key", err)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSignedCert: keygen failed", err)
	}

	dnsNames := splitCSV(dnsCSV)
	if "" != cn {
		dnsNames = prependUnique(dnsNames, cn)
	}
	var ips []net.IP
	for _, ipStr := range splitCSV(ipsCSV) {
		if ip := net.ParseIP(ipStr); nil != ip {
			ips = append(ips, ip)
		}
	}

	tpl := &x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              dnsNames,
		IPAddresses:           ips,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &priv.PublicKey, caKey)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "genSignedCert: cert creation failed", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return map[string]any{"Cert": string(certPEM), "Key": string(keyPEM)}, nil
}

func parseCertPEM(s string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(s))
	if nil == block || "CERTIFICATE" != block.Type {
		return nil, fmt.Errorf("not a CERTIFICATE PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseRSAKeyPEM(s string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(s))
	if nil == block {
		return nil, fmt.Errorf("not a PEM block")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if nil != err {
			return nil, err
		}
		rsaKey, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		return rsaKey, nil
	}
	return nil, fmt.Errorf("unsupported key PEM type %q", block.Type)
}

func splitCSV(s string) []string {
	if "" == strings.TrimSpace(s) {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if "" != p {
			out = append(out, p)
		}
	}
	return out
}

func prependUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append([]string{s}, list...)
}

// fnHTMLDate formats a time as YYYY-MM-DD (RFC 3339 calendar date).
func fnHTMLDate(value any, args ...any) (any, error) {
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	return t.Format("2006-01-02"), nil
}

// fnHTMLDateInZone is htmlDate with an explicit IANA zone.
func fnHTMLDateInZone(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "htmlDateInZone requires a zone argument")
	}
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	loc, locErr := time.LoadLocation(coerceString(args[0]))
	if nil != locErr {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, locErr, "htmlDateInZone: invalid zone %q", args[0])
	}
	return t.In(loc).Format("2006-01-02"), nil
}

// fnDateModify shifts a time by a Go duration string ("1h", "-30m", "24h").
//
// Pipeline form: ${time | dateModify "1h"}.
func fnDateModify(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "dateModify requires a duration argument")
	}
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	dur, parseErr := time.ParseDuration(coerceString(args[0]))
	if nil != parseErr {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, parseErr, "dateModify: invalid duration %q", args[0])
	}
	return t.Add(dur), nil
}

// Float-forced math: result type is always float64 even when both inputs are integral.

func fnAddf(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum, nil
}

func fnSubf(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	if 1 == len(nums) {
		return -nums[0], nil
	}
	r := nums[0]
	for _, n := range nums[1:] {
		r -= n
	}
	return r, nil
}

func fnMulf(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	r := 1.0
	for _, n := range nums {
		r *= n
	}
	return r, nil
}

func fnDivf(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	if 2 > len(nums) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "divf requires at least one divisor")
	}
	r := nums[0]
	for _, n := range nums[1:] {
		if 0 == n {
			return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "divf: division by zero")
		}
		r /= n
	}
	return r, nil
}

// fnDeepCopy returns a recursive deep copy of value. Maps and slices are
// duplicated; primitive types pass through. Useful when a package wants to
// mutate a sub-tree without affecting the original.
func fnDeepCopy(value any, args ...any) (any, error) {
	return deepCopyAny(value), nil
}

func deepCopyAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopyAny(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyAny(vv)
		}
		return out
	}
	return v
}

// fnDeepEqual returns true when `value` deeply equals `args[0]`.
func fnDeepEqual(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "deepEqual requires a comparison value")
	}
	return reflect.DeepEqual(value, args[0]), nil
}

// fnChunk splits a list into sub-lists of `size`. Sprig parity.
//
// Pipeline form: ${list | chunk 3}.
func fnChunk(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "chunk requires a size argument")
	}
	size, ok := coerceInt(args[0])
	if !ok || size <= 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "chunk: size must be a positive integer, got %v", args[0])
	}
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "chunk: expected list, got %T", value)
	}
	out := make([]any, 0, (len(list)+size-1)/size)
	for i := 0; i < len(list); i += size {
		end := i + size
		if end > len(list) {
			end = len(list)
		}
		out = append(out, append([]any{}, list[i:end]...))
	}
	return out, nil
}

// touch unused imports under build modes that strip them.
var (
	_ = bytes.Buffer{}
	_ = strconv.Itoa
)
