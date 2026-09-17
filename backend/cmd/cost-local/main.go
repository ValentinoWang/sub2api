// cost-local serves a local development slice of the native costing package.
// Its admin authority is the existing local Sub2API session; no test login is installed.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
	_ "github.com/lib/pq"
)

func pool(dsn string) *sql.DB {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db
}
func main() {
	addr := flag.String("addr", "127.0.0.1:0", "development listener; container publication must be loopback only")
	authority := flag.String("authority", "http://127.0.0.1:8080", "existing local Sub2API origin")
	refreshRadar := flag.Bool("refresh-radar", false, "read and persist the fixed public Radar reference, then exit")
	flag.Parse()
	if os.Getenv("SUB2API_COST_LOCAL_ONLY") != "yes" {
		log.Fatal("explicit local development opt-in required")
	}
	u, err := url.Parse(*authority)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Path != "" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "sub2api") {
		log.Fatal("local session authority required")
	}
	if os.Getenv("SUB2API_COST_LEDGER_DSN") == "" || os.Getenv("SUB2API_COST_SOURCE_DSN") == "" {
		log.Fatal("separate ledger and read-only source configuration required")
	}
	db := pool(os.Getenv("SUB2API_COST_LEDGER_DSN"))
	defer db.Close()
	sourceDB := pool(os.Getenv("SUB2API_COST_SOURCE_DSN"))
	defer sourceDB.Close()
	store := &costing.SQLLedgerStore{Open: func() (*sql.DB, func(), error) { return db, func() {}, nil }}
	ledger := &costing.Ledger{Store: store}
	source := &costing.SQLSource{Origin: "local-sub2api", Open: func() (*sql.DB, func(), error) { return sourceDB, func() {}, nil }}
	quota := &costing.QuotaStore{Open: store.Open}
	radar := &costing.RadarService{Open: store.Open}
	snapshots := &costing.SourceSnapshotStore{Open: store.Open}
	traffic := &costing.TrafficSampleStore{Open: store.Open}
	if *refreshRadar {
		snapshot, err := radar.Get(context.Background())
		if err != nil {
			log.Fatal("public Radar reference unavailable")
		}
		if json.NewEncoder(os.Stdout).Encode(snapshot) != nil {
			log.Fatal("reference output failed")
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect denied") }}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://127.0.0.1:4174" {
			http.Error(w, "local origin required", 403)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/admin/cost-center/") {
			http.NotFound(w, r)
			return
		}
		token := r.Header.Get("Authorization")
		if !strings.HasPrefix(token, "Bearer ") {
			http.Error(w, "admin session required", 401)
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, *authority+"/api/v1/auth/me", nil)
		if err != nil {
			http.Error(w, "authority unavailable", 503)
			return
		}
		request.Header.Set("Authorization", token)
		response, err := client.Do(request)
		if err != nil {
			http.Error(w, "authority unavailable", 503)
			return
		}
		defer response.Body.Close()
		var identity struct {
			Code int `json:"code"`
			Data struct {
				ID     int64  `json:"id"`
				Role   string `json:"role"`
				Status string `json:"status"`
			} `json:"data"`
		}
		if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&identity) != nil || identity.Code != 0 || identity.Data.ID <= 0 {
			http.Error(w, "valid admin session required", 401)
			return
		}
		if identity.Data.Role != "admin" || identity.Data.Status != "active" {
			http.Error(w, "active administrator required", 403)
			return
		}
		h := costing.HTTPHandler{Authorize: func(*http.Request) bool { return true }, Actor: func(*http.Request) int64 { return identity.Data.ID }, Ledger: ledger, Source: source, Quota: quota, Radar: radar, Snapshots: snapshots, Traffic: traffic}
		h.ServeHTTP(w, r)
	})
	// Cache observations retain their original source timestamp. Polls never create
	// fictitious exposure coverage or call the provider reset/probe endpoints.
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			observations, err := source.QuotaSnapshots(ctx)
			if err != nil {
				log.Print("read-only quota source unavailable")
			} else {
				for _, q := range observations {
					if quota.Record(ctx, q) != nil {
						log.Print("quota observation not persisted")
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
	server := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Print("local native costing started; separate PostgreSQL; existing admin session; read-only source")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("local costing listener stopped")
	}
}
