// Package automation exposes the existing authenticated HTTP API to terminal
// operators and stdio MCP clients. It never connects to the application database.
package automation

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const maxConfigBytes = 16 << 10

type credentials struct {
	Version   int    `json:"version"`
	Origin    string `json:"origin"`
	Session   string `json:"session"`
	AllowHTTP bool   `json:"allow_http,omitempty"`
}

func configPath(path string) (string, error) {
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", errors.New("cannot locate user configuration directory; specify --config")
		}
		path = filepath.Join(dir, "painter", "admin.json")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("invalid configuration path")
	}
	return path, nil
}

func validOrigin(raw string, allowHTTP bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Path != "" && u.Path != "/") || u.RawPath != "" || strings.ContainsAny(raw, "\r\n\t ") {
		return "", errors.New("--url must be one absolute HTTP(S) origin without credentials, path, query or fragment")
	}
	if u.Scheme != "https" {
		if u.Scheme != "http" || !allowHTTP {
			return "", errors.New("HTTPS is required; trusted local/LAN HTTP needs explicit --allow-http")
		}
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || (!ip.IsPrivate() && !ip.IsLoopback())) {
			return "", errors.New("--allow-http permits only localhost or a literal private/loopback IP; use HTTPS for other hosts")
		}
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", errors.New("invalid origin port")
		}
	}
	u.Path = ""
	return u.String(), nil
}

func openPrivate(path string, maxBytes int64) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("cannot open private file (symlinks are forbidden)")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private file must be regular and owner-only (0600 or 0400)")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("private file must belong to the current user")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, errors.New("private file exceeds size limit or cannot be read")
	}
	return data, nil
}

func loadCredentials(path string) (credentials, error) {
	var c credentials
	raw, err := openPrivate(path, maxConfigBytes)
	if err != nil {
		return c, err
	}
	defer clear(raw)
	if err = decodeStrict(raw, &c); err != nil || c.Version != 1 {
		return credentials{}, errors.New("invalid administrator configuration; log in again")
	}
	origin, err := validOrigin(c.Origin, c.AllowHTTP)
	if err != nil || origin != c.Origin {
		return credentials{}, errors.New("invalid administrator origin")
	}
	cookie := http.Cookie{Name: "painter_session", Value: c.Session}
	if len(c.Session) < 24 || len(c.Session) > 512 || cookie.Valid() != nil {
		return credentials{}, errors.New("invalid administrator session; log in again")
	}
	return c, nil
}

func saveCredentials(path string, c credentials) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("cannot create administrator configuration directory")
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("configuration directory must be a real directory")
	}
	if _, err := os.Lstat(path); err == nil {
		data, e := openPrivate(path, maxConfigBytes)
		if e != nil {
			return e
		}
		clear(data)
	} else if !os.IsNotExist(err) {
		return errors.New("cannot inspect configuration file")
	}
	file, err := os.CreateTemp(dir, ".painter-admin-*")
	if err != nil {
		return errors.New("cannot create private configuration file")
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		err = json.NewEncoder(file).Encode(c)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.New("cannot write private configuration file")
	}
	if err = os.Rename(name, path); err != nil {
		return errors.New("cannot save private configuration file")
	}
	directory, err := os.Open(dir)
	if err != nil {
		return errors.New("cannot sync configuration directory")
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil {
		return errors.New("cannot sync configuration directory")
	}
	return nil
}

func readPassword(stdin io.Reader, useStdin bool, path string) (string, error) {
	if useStdin == (path != "") {
		return "", errors.New("choose exactly one of --password-stdin or --password-file; command-line passwords are forbidden")
	}
	var raw []byte
	var err error
	if path != "" {
		raw, err = openPrivate(path, 4098)
	} else {
		raw, err = io.ReadAll(io.LimitReader(stdin, 4099))
	}
	if err != nil || len(raw) > 4098 {
		return "", errors.New("cannot read password input or input is too large")
	}
	defer clear(raw)
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if password == "" || len(password) > 4096 || strings.ContainsRune(password, '\x00') {
		return "", errors.New("invalid password input")
	}
	return password, nil
}
