package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
	_ "github.com/lib/pq"
)

func main() {
	source := flag.String("source", "", "explicit source identity")
	iface := flag.String("interface", "", "explicit network interface")
	adapter := flag.String("adapter", "vnstat", "vnstat or linux_kernel; no automatic substitution")
	scope := flag.String("scope", "host_interface", "host_interface or container_interface")
	remote := flag.String("ssh-host", "", "optional trusted SSH config alias")
	interval := flag.Duration("interval", time.Minute, "collection interval, at least 10s")
	once := flag.Bool("once", false, "collect one observation")
	flag.Parse()
	collector := costing.TrafficCollector{Source: *source, Interface: *iface, Adapter: *adapter, Scope: *scope, SSHHost: *remote}
	if collector.Validate() != nil || *interval < 10*time.Second {
		log.Fatal("explicit valid collector configuration required")
	}
	dsn := os.Getenv("SUB2API_COST_TRAFFIC_DSN")
	if dsn == "" {
		log.Fatal("traffic-only database credential required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("collector database unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	store := &costing.TrafficSampleStore{Open: func() (*sql.DB, func(), error) { return db, func() {}, nil }}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		sample, err := collector.Collect(ctx)
		if err == nil {
			err = store.Record(ctx, sample)
		}
		if err != nil {
			log.Print("traffic sample unavailable; no counter or price substituted")
			if *once {
				os.Exit(1)
			}
		} else {
			log.Printf("traffic sample persisted: source=%s interface=%s adapter=%s at=%s", sample.Source, sample.Interface, sample.Adapter, sample.ObservedAt)
		}
		if *once {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
