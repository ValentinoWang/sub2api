//go:build linux || darwin

package costing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileLedgerStore is a Unix local acceptance adapter. Production uses Postgres.
// The directory must be private and dedicated; no symlinks, multiprocess writes use flock.
type FileLedgerStore struct{ Path string }

func (s *FileLedgerStore) withLock(ctx context.Context, write bool, fn func([]LedgerEvent) (*LedgerEvent, error)) (events []LedgerEvent, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if s == nil || !filepath.IsAbs(s.Path) {
		return nil, ErrUnavailable
	}
	dir := filepath.Dir(s.Path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private ledger directory (0700) required")
	}
	fd, e := syscall.Open(s.Path+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	lock := os.NewFile(uintptr(fd), s.Path+".lock")
	defer lock.Close()
	mode := syscall.LOCK_SH
	if write {
		mode = syscall.LOCK_EX
	}
	for {
		e = syscall.Flock(fd, mode|syscall.LOCK_NB)
		if e == nil {
			break
		}
		if e != syscall.EWOULDBLOCK && e != syscall.EAGAIN {
			return nil, e
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	fdData, e := syscall.Open(s.Path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e == syscall.ENOENT {
		events = []LedgerEvent{}
	} else if e != nil {
		return nil, e
	} else {
		f := os.NewFile(uintptr(fdData), s.Path)
		stat, e := f.Stat()
		if e != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 {
			f.Close()
			return nil, errors.New("private regular ledger file required")
		}
		raw, e := io.ReadAll(io.LimitReader(f, 32*1024*1024+1))
		f.Close()
		if e != nil {
			return nil, e
		}
		if len(raw) > 32*1024*1024 {
			return nil, ErrLimit
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&events); e != nil {
			return nil, errors.New("ledger file corrupt; restore a verified backup")
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, errors.New("trailing ledger bytes")
		}
	}
	if e = ValidateEventStream(events); e != nil {
		return nil, e
	}
	if !write {
		return events, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	next, e := fn(events)
	if e != nil {
		return nil, e
	}
	if next == nil {
		return events, nil
	}
	events = append(events, *next)
	raw, e := json.Marshal(events)
	if e != nil {
		return nil, e
	}
	if len(raw) > 32*1024*1024 {
		return nil, ErrLimit
	}
	tmp, e := os.CreateTemp(dir, ".cost-ledger-*")
	if e != nil {
		return nil, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(raw)
	}
	if e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if e = os.Rename(name, s.Path); e != nil {
		return nil, e
	}
	directory, e := os.Open(dir)
	if e != nil {
		return nil, e
	}
	defer directory.Close()
	if e = directory.Sync(); e != nil {
		return nil, e
	}
	return events, nil
}
func (s *FileLedgerStore) Snapshot(ctx context.Context) ([]LedgerEvent, error) {
	return s.withLock(ctx, false, nil)
}
func (s *FileLedgerStore) Transact(ctx context.Context, fn func([]LedgerEvent) (*LedgerEvent, error)) error {
	_, err := s.withLock(ctx, true, fn)
	return err
}
