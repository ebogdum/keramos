package engine

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

func registerEncodingFuncs(r *FuncRegistry) {
	r.Register("b64encode", fnB64Encode)
	r.Register("b64decode", fnB64Decode)
	r.Register("sha256", fnSha256)
}

func fnB64Encode(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	return base64.StdEncoding.EncodeToString([]byte(s)), nil
}

func fnB64Decode(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	b, err := base64.StdEncoding.DecodeString(s)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "b64decode: invalid base64", err)
	}
	return string(b), nil
}

func fnSha256(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h), nil
}
