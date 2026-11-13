// Package backupops schedules the existing consistent backup and isolated restore
// primitives. It owns only directories carrying its private completion marker.
package backupops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const format = "painter-automatic-backup-v1"

var ErrBusy = errors.New("another backup or recovery operation is running")

type Hooks struct {
	Backup func(context.Context, string) (json.RawMessage, error)
	Check  func(context.Context, string) (json.RawMessage, error)
}
type Options struct {
	Root           string
	Keep           int
	ConfigSnapshot string
}
type Snapshot struct {
	Format              string          `json:"format"`
	Name                string          `json:"name"`
	CreatedAt           time.Time       `json:"created_at"`
	VerifiedAt          time.Time       `json:"verified_at"`
	Backup              json.RawMessage `json:"backup"`
	Check               json.RawMessage `json:"restore_check"`
	ConfigIncluded      bool            `json:"private_configuration_included"`
	ConfigurationSHA256 string          `json:"configuration_sha256,omitempty"`
}
type State struct {
	Format        string     `json:"format"`
	Phase         string     `json:"phase"`
	StartedAt     time.Time  `json:"started_at,omitempty"`
	FinishedAt    time.Time  `json:"finished_at,omitempty"`
	LastSuccessAt time.Time  `json:"last_success_at,omitempty"`
	LastSnapshot  string     `json:"last_snapshot,omitempty"`
	ErrorCode     string     `json:"error_code,omitempty"`
	Running       bool       `json:"running"`
	Snapshots     []Snapshot `json:"snapshots,omitempty"`
}
type Store struct {
	root string
	lock *os.File
}

func privateDirectory(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Reject symlinks in every existing path component, including ancestor dirs.
	walk := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(absolute, string(filepath.Separator)), string(filepath.Separator)) {
		walk = filepath.Join(walk, part)
		info, e := os.Lstat(walk)
		if errors.Is(e, os.ErrNotExist) {
			if e = os.Mkdir(walk, 0700); e != nil {
				return e
			}
			continue
		}
		if e != nil {
			return e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup path must contain only real directories")
		}
	}
	return os.Chmod(absolute, 0700)
}
func Open(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("backup root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = privateDirectory(root); err != nil {
		return nil, err
	}
	marker := filepath.Join(root, "automatic-store.json")
	if _, err = os.Lstat(marker); errors.Is(err, os.ErrNotExist) {
		entries, e := os.ReadDir(root)
		if e != nil {
			return nil, e
		}
		if len(entries) != 0 {
			return nil, errors.New("new automatic backup root must be empty")
		}
		if err = writeNewJSON(marker, map[string]string{"format": format}); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	var m struct {
		Format string `json:"format"`
	}
	if err = readJSON(marker, &m); err != nil || m.Format != format {
		return nil, errors.New("unrecognized automatic backup root")
	}
	lock, err := os.OpenFile(filepath.Join(root, "operation.lock"), os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return &Store{root: root, lock: lock}, nil
}
func (s *Store) Close() {
	if s.lock != nil {
		_ = unix.Flock(int(s.lock.Fd()), unix.LOCK_UN)
		_ = s.lock.Close()
		s.lock = nil
	}
}
func (s *Store) Root() string { return s.root }
func readJSON(path string, value any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return errors.New("invalid backup metadata file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 4<<20))
	if err = d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid trailing metadata")
	}
	return nil
}
func writeNewJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
func atomicJSON(path string, value any) error {
	suffix := randomID()
	tmp := path + ".tmp-" + suffix
	if err := writeNewJSON(tmp, value); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("operating system randomness unavailable")
	}
	return hex.EncodeToString(b[:])
}
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func validName(name string) bool {
	if !strings.HasPrefix(name, "auto-") || filepath.Base(name) != name || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
func (s *Store) snapshots() ([]Snapshot, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Snapshot{}
	for _, entry := range entries {
		if !entry.IsDir() || !validName(entry.Name()) {
			continue
		}
		var snap Snapshot
		if readJSON(filepath.Join(s.root, entry.Name(), "automatic.json"), &snap) != nil || snap.Format != format || snap.Name != entry.Name() || snap.VerifiedAt.IsZero() {
			continue
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) status() State {
	state := State{Format: format, Phase: "never_run"}
	_ = readJSON(filepath.Join(s.root, "status.json"), &state)
	return state
}

// Status is read-only, including when another process owns the operation lock.
func Status(root string) (State, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return State{}, err
	}
	for path := root; ; path = filepath.Dir(path) {
		info, checkErr := os.Lstat(path)
		if checkErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return State{}, errors.New("backup status root must contain only real directories")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	var m struct {
		Format string `json:"format"`
	}
	if err = readJSON(filepath.Join(root, "automatic-store.json"), &m); err != nil || m.Format != format {
		return State{}, errors.New("automatic backup root not initialized")
	}
	s := &Store{root: root}
	state := s.status()
	state.Snapshots, err = s.snapshots()
	if err != nil {
		return state, err
	}
	f, e := os.OpenFile(filepath.Join(root, "operation.lock"), os.O_RDWR|unix.O_NOFOLLOW, 0)
	if e != nil {
		return state, e
	}
	defer f.Close()
	e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(e, unix.EWOULDBLOCK) {
		state.Running = true
	} else if e == nil {
		state.Running = false
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		if state.Phase == "backup" || state.Phase == "restore_check" {
			state.Phase = "interrupted"
			state.ErrorCode = "INTERRUPTED"
		}
	} else {
		return state, e
	}
	return state, nil
}
func copyConfiguration(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<20 {
		return errors.New("configuration snapshot requires a private regular file (0600 or stricter, at most 1 MiB)")
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(in, (1<<20)+1))
	if err == nil && n > 1<<20 {
		err = errors.New("configuration grew beyond snapshot limit")
	}
	if err == nil {
		err = out.Sync()
	}
	ce := out.Close()
	if err == nil {
		err = ce
	}
	return err
}

// Run never prunes on backup/check failure. The existing maintenance gate makes
// DB+media consistent; this filesystem lock also serializes rotations and checks.
func Run(ctx context.Context, options Options, hooks Hooks) (result Snapshot, err error) {
	if options.Keep < 1 || options.Keep > 3650 {
		return result, errors.New("keep must be 1..3650")
	}
	if hooks.Backup == nil || hooks.Check == nil {
		return result, errors.New("backup and restore-check hooks required")
	}
	store, err := Open(options.Root)
	if err != nil {
		return result, err
	}
	defer store.Close()
	state := store.status()
	state.StartedAt = time.Now().UTC()
	state.FinishedAt = time.Time{}
	state.Phase = "backup"
	state.ErrorCode = ""
	state.Running = true
	state.Snapshots = nil
	if err = atomicJSON(filepath.Join(store.root, "status.json"), state); err != nil {
		return result, err
	}
	defer func() {
		state.Running = false
		state.FinishedAt = time.Now().UTC()
		if err != nil {
			state.Phase = "failed"
			if state.ErrorCode == "" {
				state.ErrorCode = "BACKUP_FAILED"
			}
		}
		writeErr := atomicJSON(filepath.Join(store.root, "status.json"), state)
		if err == nil {
			err = writeErr
		}
	}()
	name := "auto-" + state.StartedAt.Format("20060102T150405Z") + "-" + randomID()
	directory := filepath.Join(store.root, name)
	// Backup owns creation of this new directory and its consistent snapshot.
	result = Snapshot{Format: format, Name: name, CreatedAt: state.StartedAt}
	result.Backup, err = hooks.Backup(ctx, directory)
	if err != nil {
		return result, err
	}
	if options.ConfigSnapshot != "" {
		if err = copyConfiguration(options.ConfigSnapshot, filepath.Join(directory, "recovery.env")); err != nil {
			state.ErrorCode = "CONFIGURATION_SNAPSHOT_FAILED"
			return result, err
		}
		configuration, readErr := os.ReadFile(filepath.Join(directory, "recovery.env"))
		if readErr != nil {
			state.ErrorCode = "CONFIGURATION_SNAPSHOT_FAILED"
			return result, readErr
		}
		sum := sha256.Sum256(configuration)
		result.ConfigurationSHA256 = hex.EncodeToString(sum[:])
		result.ConfigIncluded = true
	}
	state.Phase = "restore_check"
	if err = atomicJSON(filepath.Join(store.root, "status.json"), state); err != nil {
		return result, err
	}
	result.Check, err = hooks.Check(ctx, directory)
	if err != nil {
		state.ErrorCode = "RESTORE_CHECK_FAILED"
		return result, err
	}
	var check struct {
		Verified bool `json:"verified"`
		Dropped  bool `json:"temporary_database_dropped"`
	}
	if json.Unmarshal(result.Check, &check) != nil || !check.Verified || !check.Dropped {
		state.ErrorCode = "RESTORE_CHECK_INCOMPLETE"
		return result, errors.New("restore check did not verify and remove its isolated database")
	}
	result.VerifiedAt = time.Now().UTC()
	if err = writeNewJSON(filepath.Join(directory, "automatic.json"), result); err != nil {
		return result, err
	}
	if err = syncDirectory(directory); err != nil {
		return result, err
	}
	if err = syncDirectory(store.root); err != nil {
		return result, err
	}
	state.LastSnapshot = name
	state.LastSuccessAt = result.VerifiedAt
	state.Phase = "complete"
	// Only our fully verified completion markers grant deletion authority.
	snapshots, e := store.snapshots()
	if e != nil {
		return result, e
	}
	for _, old := range snapshots[minimum(options.Keep, len(snapshots)):] {
		if err = os.RemoveAll(filepath.Join(store.root, old.Name)); err != nil {
			state.ErrorCode = "RETENTION_FAILED"
			return result, err
		}
	}
	if err = syncDirectory(store.root); err != nil {
		return result, err
	}
	return result, nil
}
func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func Check(ctx context.Context, root, name string, hooks Hooks) (json.RawMessage, error) {
	if !validName(name) || hooks.Check == nil {
		return nil, errors.New("valid automatic snapshot name and check hook required")
	}
	store, err := Open(root)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	path := filepath.Join(store.root, name)
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("snapshot must be a real directory")
	}
	return hooks.Check(ctx, path)
}
