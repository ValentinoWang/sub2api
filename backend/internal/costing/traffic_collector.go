package costing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var interfacePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
var sshAliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,119}$`)

type NetworkSample struct {
	Source                   string `json:"source"`
	Interface                string `json:"interface"`
	Adapter                  string `json:"adapter"`
	Scope                    string `json:"scope"`
	ObservedAt               string `json:"observed_at"`
	Epoch                    string `json:"epoch"`
	RXBytes                  string `json:"rx_bytes"`
	TXBytes                  string `json:"tx_bytes"`
	BillingInterfaceVerified bool   `json:"billing_interface_verified"`
}
type TrafficCollector struct{ Source, Interface, Adapter, Scope, SSHHost, ProcRoot string }

func (c TrafficCollector) Validate() error {
	if !refOK(c.Source) || !interfacePattern.MatchString(c.Interface) || (c.Adapter != "vnstat" && c.Adapter != "linux_kernel") || (c.Scope != "host_interface" && c.Scope != "container_interface") || (c.SSHHost != "" && !sshAliasPattern.MatchString(c.SSHHost)) {
		return invalid("explicit traffic source, interface, adapter and scope required")
	}
	return nil
}
func (c TrafficCollector) Collect(ctx context.Context) (NetworkSample, error) {
	out := NetworkSample{Source: c.Source, Interface: c.Interface, Adapter: c.Adapter, Scope: c.Scope}
	if err := c.Validate(); err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var raw []byte
	var err error
	if c.Adapter == "vnstat" {
		if c.SSHHost == "" {
			raw, err = exec.CommandContext(ctx, "vnstat", "--json", "-i", c.Interface).Output()
		} else {
			raw, err = exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=8", c.SSHHost, "vnstat", "--json", "-i", c.Interface).Output()
		}
		if err != nil || len(raw) > 4_000_000 {
			return out, ErrUnavailable
		}
		return parseCollectedVnstat(out, raw)
	}
	if c.SSHHost != "" {
		raw, err = exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=8", c.SSHHost, "cat /proc/sys/kernel/random/boot_id; date -u +%Y-%m-%dT%H:%M:%SZ; cat /proc/net/dev").Output()
		if err != nil || len(raw) > 65536 {
			return out, ErrUnavailable
		}
		lines := strings.SplitN(string(raw), "\n", 3)
		if len(lines) != 3 {
			return out, ErrUnavailable
		}
		return parseKernelSample(out, lines[0], lines[1], lines[2])
	}
	root := c.ProcRoot
	if root == "" {
		root = "/proc"
	}
	boot, err := os.ReadFile(filepath.Join(root, "sys/kernel/random/boot_id"))
	if err != nil {
		return out, ErrUnavailable
	}
	raw, err = os.ReadFile(filepath.Join(root, "net/dev"))
	if err != nil {
		return out, ErrUnavailable
	}
	return parseKernelSample(out, strings.TrimSpace(string(boot)), time.Now().UTC().Format(time.RFC3339Nano), string(raw))
}
func parseKernelSample(out NetworkSample, epoch, at, raw string) (NetworkSample, error) {
	if epoch == "" || len(epoch) > 100 {
		return out, ErrUnavailable
	}
	if _, err := parseTime(at); err != nil {
		return out, ErrUnavailable
	}
	found := false
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) != out.Interface {
			continue
		}
		if found {
			return out, ErrUnavailable
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return out, ErrUnavailable
		}
		if _, ok := integer(fields[0]); !ok {
			return out, ErrUnavailable
		}
		if _, ok := integer(fields[8]); !ok {
			return out, ErrUnavailable
		}
		out.RXBytes = fields[0]
		out.TXBytes = fields[8]
		found = true
	}
	if !found {
		return out, ErrNotFound
	}
	sum := sha256.Sum256([]byte(epoch))
	out.Epoch = hex.EncodeToString(sum[:])
	out.ObservedAt = at
	return out, nil
}
func parseCollectedVnstat(out NetworkSample, raw []byte) (NetworkSample, error) {
	var data struct {
		Version    json.RawMessage   `json:"jsonversion"`
		Interfaces []vnstatInterface `json:"interfaces"`
	}
	if json.Unmarshal(raw, &data) != nil || (string(data.Version) != "2" && string(data.Version) != "\"2\"") {
		return out, ErrUnavailable
	}
	found := false
	for _, iface := range data.Interfaces {
		if iface.Name != out.Interface {
			continue
		}
		if found || iface.Updated.Timestamp <= 0 {
			return out, ErrUnavailable
		}
		at := time.Unix(iface.Updated.Timestamp, 0).UTC().Format(time.RFC3339Nano)
		checked, err := counter(raw, out.Interface, at)
		if err != nil {
			return out, err
		}
		var created any
		if json.Unmarshal(checked.Created, &created) != nil {
			return out, ErrUnavailable
		}
		normalized, err := json.Marshal(created)
		if err != nil {
			return out, ErrUnavailable
		}
		sum := sha256.Sum256(normalized)
		out.Epoch = hex.EncodeToString(sum[:])
		out.RXBytes = checked.Traffic.Total.RX.String()
		out.TXBytes = checked.Traffic.Total.TX.String()
		out.ObservedAt = at
		found = true
	}
	if !found {
		return out, ErrNotFound
	}
	return out, nil
}

type TrafficSampleStore struct {
	Open func() (*sql.DB, func(), error)
}

func (s *TrafficSampleStore) Record(ctx context.Context, sample NetworkSample) error {
	c := TrafficCollector{Source: sample.Source, Interface: sample.Interface, Adapter: sample.Adapter, Scope: sample.Scope}
	if c.Validate() != nil || len(sample.Epoch) != 64 {
		return invalid("traffic sample identity")
	}
	if _, err := parseTime(sample.ObservedAt); err != nil {
		return invalid("traffic sample time")
	}
	if _, ok := integer(sample.RXBytes); !ok {
		return invalid("traffic rx counter")
	}
	if _, ok := integer(sample.TXBytes); !ok {
		return invalid("traffic tx counter")
	}
	if s == nil || s.Open == nil {
		return ErrUnavailable
	}
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := json.Marshal(sample)
	if err != nil {
		return ErrUnavailable
	}
	result, err := db.ExecContext(ctx, `INSERT INTO cost_center_traffic_samples(source,interface,observed_at,payload) VALUES($1,$2,$3,$4::jsonb) ON CONFLICT(source,interface,observed_at) DO NOTHING`, sample.Source, sample.Interface, sample.ObservedAt, string(raw))
	if err != nil {
		return ErrUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil {
		return ErrUnavailable
	}
	if count == 0 {
		var previous []byte
		if db.QueryRowContext(ctx, `SELECT payload FROM cost_center_traffic_samples WHERE source=$1 AND interface=$2 AND observed_at=$3`, sample.Source, sample.Interface, sample.ObservedAt).Scan(&previous) != nil {
			return ErrUnavailable
		}
		var old NetworkSample
		if json.Unmarshal(previous, &old) != nil {
			return ErrUnavailable
		}
		if old != sample {
			return ErrConflict
		}
	}
	return nil
}
func (s *TrafficSampleStore) Latest(ctx context.Context) ([]NetworkSample, error) {
	if s == nil || s.Open == nil {
		return nil, ErrUnavailable
	}
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return nil, ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT payload FROM cost_center_traffic_samples ORDER BY observed_at DESC LIMIT 100`)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	out := []NetworkSample{}
	for rows.Next() {
		var raw []byte
		var sample NetworkSample
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &sample) != nil {
			return nil, ErrUnavailable
		}
		out = append(out, sample)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return out, nil
}

var ErrCounterReset = errors.New("counter epoch changed or regressed")

func NetworkDelta(before, after NetworkSample) (string, string, error) {
	if before.Source != after.Source || before.Interface != after.Interface || before.Adapter != after.Adapter || before.Epoch != after.Epoch {
		return "", "", ErrCounterReset
	}
	if !instant(after.ObservedAt).After(instant(before.ObservedAt)) {
		return "", "", invalid("traffic sample order")
	}
	rx0, ok0 := integer(before.RXBytes)
	rx1, ok1 := integer(after.RXBytes)
	tx0, ok2 := integer(before.TXBytes)
	tx1, ok3 := integer(after.TXBytes)
	if !ok0 || !ok1 || !ok2 || !ok3 {
		return "", "", invalid("traffic counters")
	}
	if rx1.Cmp(rx0) < 0 || tx1.Cmp(tx0) < 0 {
		return "", "", ErrCounterReset
	}
	return rx1.Sub(rx1, rx0).String(), tx1.Sub(tx1, tx0).String(), nil
}
