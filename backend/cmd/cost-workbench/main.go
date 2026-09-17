//go:build linux || darwin

// Local acceptance server for the same native ledger API. Never a production auth replacement.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
)

func main() {
	path := flag.String("ledger", "", "private ledger path, outside the checkout")
	addr := flag.String("addr", "127.0.0.1:0", "literal loopback IP only, default dynamic port")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("literal loopback listener required")
	}
	if *path == "" {
		log.Fatal("--ledger is required")
	}
	absolute, err := filepath.Abs(*path)
	if err != nil {
		log.Fatal("invalid ledger path")
	}
	store := &costing.FileLedgerStore{Path: absolute}
	if _, err = store.Snapshot(context.Background()); err != nil {
		log.Fatal("private ledger could not be read; check permissions and integrity")
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		log.Fatal("random source unavailable")
	}
	token := base64.RawURLEncoding.EncodeToString(key)
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("listener unavailable")
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	h := costing.HTTPHandler{Authorize: func(r *http.Request) bool {
		return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(token)) == 1
	}, Actor: func(*http.Request) int64 { return 1 }, Ledger: &costing.Ledger{Store: store}}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listener.Addr().String() || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) {
			http.Error(w, "origin denied", 403)
			return
		}
		h.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	fmt.Println("URL=" + origin)
	fmt.Println("TOKEN=" + token)
	fmt.Println("MODE=local-acceptance-only; file-backed; no production credentials")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal("server stopped unexpectedly")
	}
}
