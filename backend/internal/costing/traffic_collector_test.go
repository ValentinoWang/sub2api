package costing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCollectorReadsExactKernelCounters(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys/kernel/random"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "net"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sys/kernel/random/boot_id"), []byte("epoch-a"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := " eth0: 9007199254740993 1 0 0 0 0 0 0 9007199254741093 1 0 0 0 0 0 0\n veth0: 999 1 0 0 0 0 0 0 999 1 0 0 0 0 0 0\n"
	if err := os.WriteFile(filepath.Join(root, "net/dev"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	sample, err := (TrafficCollector{Source: "test", Interface: "eth0", Adapter: "linux_kernel", Scope: "container_interface", ProcRoot: root}).Collect(context.Background())
	if err != nil || sample.RXBytes != "9007199254740993" || sample.TXBytes != "9007199254741093" || sample.BillingInterfaceVerified {
		t.Fatal(sample, err)
	}
	after := sample
	after.ObservedAt = "2026-10-01T00:00:00Z"
	sample.ObservedAt = "2026-09-01T00:00:00Z"
	after.RXBytes = "9007199254740995"
	after.TXBytes = "9007199254741098"
	rx, tx, err := NetworkDelta(sample, after)
	if err != nil || rx != "2" || tx != "5" {
		t.Fatal(rx, tx, err)
	}
	after.Epoch = "new-epoch"
	if _, _, err = NetworkDelta(sample, after); !errors.Is(err, ErrCounterReset) {
		t.Fatal("epoch change accepted")
	}
}
func TestNativeVnstatCollectorKeepsSourceTimestamp(t *testing.T) {
	raw := `{"jsonversion":"2","interfaces":[{"name":"eth0","created":{"timestamp":1},"updated":{"timestamp":1788220800},"traffic":{"total":{"rx":100,"tx":200}}}]}`
	sample, err := parseCollectedVnstat(NetworkSample{Source: "test", Interface: "eth0", Adapter: "vnstat", Scope: "host_interface"}, []byte(raw))
	if err != nil || sample.ObservedAt != "2026-09-01T00:00:00Z" || sample.RXBytes != "100" {
		t.Fatal(sample, err)
	}
	if _, err = parseCollectedVnstat(sample, []byte(strings.ReplaceAll(raw, `"jsonversion":"2"`, `"jsonversion":"1"`))); err == nil {
		t.Fatal("unknown byte format accepted")
	}
	if (TrafficCollector{Source: "test", Interface: "eth0;id", Adapter: "vnstat", Scope: "host_interface", SSHHost: "host"}).Validate() == nil {
		t.Fatal("unsafe interface accepted")
	}
}
