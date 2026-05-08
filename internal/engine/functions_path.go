package engine

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

func registerPathFuncs(r *FuncRegistry) {
	r.Register("base", fnBase)
	r.Register("dir", fnDir)
	r.Register("clean", fnClean)
	r.Register("ext", fnExt)
	r.Register("isAbs", fnIsAbs)
	r.Register("osBase", fnOSBase)
	r.Register("osDir", fnOSDir)
	r.Register("osClean", fnOSClean)
	r.Register("osExt", fnOSExt)
	r.Register("osIsAbs", fnOSIsAbs)
	r.Register("urlParse", fnURLParse)
	r.Register("urlJoin", fnURLJoin)
}

func fnBase(value any, args ...any) (any, error) {
	return path.Base(fmt.Sprintf("%v", value)), nil
}
func fnDir(value any, args ...any) (any, error) {
	return path.Dir(fmt.Sprintf("%v", value)), nil
}
func fnClean(value any, args ...any) (any, error) {
	return path.Clean(fmt.Sprintf("%v", value)), nil
}
func fnExt(value any, args ...any) (any, error) {
	return path.Ext(fmt.Sprintf("%v", value)), nil
}
func fnIsAbs(value any, args ...any) (any, error) {
	return strings.HasPrefix(fmt.Sprintf("%v", value), "/"), nil
}

func fnOSBase(value any, args ...any) (any, error) {
	return filepath.Base(fmt.Sprintf("%v", value)), nil
}
func fnOSDir(value any, args ...any) (any, error) {
	return filepath.Dir(fmt.Sprintf("%v", value)), nil
}
func fnOSClean(value any, args ...any) (any, error) {
	return filepath.Clean(fmt.Sprintf("%v", value)), nil
}
func fnOSExt(value any, args ...any) (any, error) {
	return filepath.Ext(fmt.Sprintf("%v", value)), nil
}
func fnOSIsAbs(value any, args ...any) (any, error) {
	return filepath.IsAbs(fmt.Sprintf("%v", value)), nil
}

func fnURLParse(value any, args ...any) (any, error) {
	u, err := url.Parse(fmt.Sprintf("%v", value))
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "urlParse: invalid URL", err)
	}
	return map[string]any{
		"scheme":   u.Scheme,
		"host":     u.Host,
		"hostname": u.Hostname(),
		"port":     u.Port(),
		"path":     u.Path,
		"query":    u.RawQuery,
		"fragment": u.Fragment,
		"userinfo": u.User.String(),
		"opaque":   u.Opaque,
	}, nil
}

func fnURLJoin(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "urlJoin: expected map, got %T", value)
	}
	u := url.URL{}
	if v, ok := m["scheme"].(string); ok {
		u.Scheme = v
	}
	if v, ok := m["host"].(string); ok {
		u.Host = v
	}
	if v, ok := m["path"].(string); ok {
		u.Path = v
	}
	if v, ok := m["query"].(string); ok {
		u.RawQuery = v
	}
	if v, ok := m["fragment"].(string); ok {
		u.Fragment = v
	}
	return u.String(), nil
}
