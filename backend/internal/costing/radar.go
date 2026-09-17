package costing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const RadarURL = "https://codexradar.com/en/"

type RadarQuota struct {
	Tier         string  `json:"tier"`
	ModelLabel   string  `json:"model_label"`
	ReferenceUSD string  `json:"reference_usd"`
	Basis        string  `json:"basis"`
	Period       *string `json:"period"`
	PriceVersion *string `json:"price_version"`
}
type RadarSpeed struct {
	Effort      string `json:"effort"`
	StandardTPS string `json:"standard_tps"`
	FastTPS     string `json:"fast_tps"`
}
type RadarSnapshot struct {
	Source             string       `json:"source"`
	RetrievedAt        string       `json:"retrieved_at"`
	SourceUpdatedLabel string       `json:"source_updated_label"`
	SourceSHA256       string       `json:"source_sha256"`
	Status             string       `json:"status"`
	Quotas             []RadarQuota `json:"quotas"`
	Speeds             []RadarSpeed `json:"speeds"`
	SpeedModelLabel    string       `json:"speed_model_label"`
	SevenDayAverage    *string      `json:"seven_day_average"`
	TwoMonthResetRate  *string      `json:"two_month_reset_rate"`
	Warnings           []string     `json:"warnings"`
}

var radarSection = regexp.MustCompile(`(?s)<section\b[^>]*\bid="quota-radar"[^>]*>(.*?)</section>`)
var radarCard = regexp.MustCompile(`(?s)<article\b[^>]*class="quota-radar-current-card [^"]*"[^>]*>(.*?)</article>`)
var radarUpdated = regexp.MustCompile(`(?s)<h2[^>]*>\s*Quota Radar\s*<span[^>]*>(.*?)</span>`)
var radarSpan = regexp.MustCompile(`(?s)<span[^>]*>(.*?)</span>`)
var radarStrong = regexp.MustCompile(`(?s)<strong[^>]*>(.*?)</strong>`)
var radarBasis = regexp.MustCompile(`(?s)<em[^>]*>(.*?)</em>`)
var radarTags = regexp.MustCompile(`<[^>]*>`)
var radarAmount = regexp.MustCompile(`^\$([0-9]+(?:,[0-9]{3})*\.[0-9]{2})$`)
var radarFastSection = regexp.MustCompile(`(?s)<section\b[^>]*\bid="fast-radar"[^>]*>(.*?)</section>`)
var radarSpeedSection = regexp.MustCompile(`(?s)<div class="fast-simple-row"[^>]*data-fast-simple-effort="([a-z]+)"[^>]*>(.*?)</div>`)
var radarStandard = regexp.MustCompile(`(?s)<span[^>]*class="fast-simple-standard"[^>]*>(.*?)</span>`)
var radarFast = regexp.MustCompile(`(?s)<span[^>]*class="fast-simple-speed"[^>]*>(.*?)</span>`)

func sourceText(pattern *regexp.Regexp, body string) string {
	m := pattern.FindStringSubmatch(body)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(radarTags.ReplaceAllString(m[1], "")))
}

// ParseRadar extracts only explicitly labeled current public cards. It does not
// turn a site dollar valuation into a weekly allowance or a model price version.
func ParseRadar(raw []byte, at time.Time) (RadarSnapshot, error) {
	out := RadarSnapshot{Source: RadarURL, RetrievedAt: at.UTC().Format(time.RFC3339Nano), Status: "PUBLIC_REFERENCE_UNCALIBRATED", Quotas: []RadarQuota{}, Speeds: []RadarSpeed{}, Warnings: []string{"THIRD_PARTY_NOT_OWN_ACCOUNT", "PERIOD_AND_PRICE_VERSION_NOT_DECLARED", "MODEL_SCENARIOS_MUST_NOT_BE_SUMMED", "SOURCE_TIMEZONE_NOT_DECLARED", "NO_COMPLETE_DAILY_OR_RESET_HISTORY"}}
	digest := sha256.Sum256(raw)
	out.SourceSHA256 = hex.EncodeToString(digest[:])
	section := radarSection.FindSubmatch(raw)
	if len(section) != 2 {
		return out, errors.New("radar page schema changed")
	}
	out.SourceUpdatedLabel = sourceText(radarUpdated, string(section[1]))
	if out.SourceUpdatedLabel == "" {
		return out, errors.New("radar update label missing")
	}
	seen := map[string]bool{}
	for _, card := range radarCard.FindAllStringSubmatch(string(section[1]), -1) {
		label := sourceText(radarSpan, card[1])
		value := radarAmount.FindStringSubmatch(sourceText(radarStrong, card[1]))
		basis := sourceText(radarBasis, card[1])
		parts := strings.SplitN(label, " · ", 2)
		if len(parts) != 2 || len(value) != 2 || basis == "" || len(basis) > 160 {
			return out, errors.New("radar card fields changed")
		}
		tier := map[string]string{"20x Pro": "pro20x", "5x Pro": "pro5x", "Plus": "plus"}[parts[0]]
		if tier == "" || seen[label] || len(parts[1]) > 100 {
			return out, errors.New("radar card identity changed")
		}
		seen[label] = true
		amount := strings.ReplaceAll(value[1], ",", "")
		if !validateMoney(amount) {
			return out, errors.New("radar reference amount invalid")
		}
		out.Quotas = append(out.Quotas, RadarQuota{Tier: tier, ModelLabel: parts[1], ReferenceUSD: amount, Basis: basis})
	}
	if len(out.Quotas) == 0 || len(out.Quotas) > 20 {
		return out, errors.New("radar quota cards unavailable")
	}
	fastSection := radarFastSection.FindSubmatch(raw)
	if len(fastSection) != 2 {
		return out, nil
	}
	out.SpeedModelLabel = sourceText(radarSpan, string(fastSection[1]))
	if out.SpeedModelLabel == "" {
		return out, nil
	}
	for _, row := range radarSpeedSection.FindAllStringSubmatch(string(fastSection[1]), -1) {
		standard, fast := sourceText(radarStandard, row[2]), sourceText(radarFast, row[2])
		if !validateMoney(standard) || !validateMoney(fast) {
			continue
		}
		out.Speeds = append(out.Speeds, RadarSpeed{row[1], standard, fast})
	}
	return out, nil
}

// RadarService performs an on-demand, fixed-origin public GET, at most once per
// ten minutes. It is not a scheduled crawler and never carries admin credentials.
type RadarService struct {
	Open   func() (*sql.DB, func(), error)
	Client *http.Client
	mu     sync.Mutex
	cached *RadarSnapshot
}

func (s *RadarService) Get(ctx context.Context) (RadarSnapshot, error) {
	if s == nil {
		return RadarSnapshot{}, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(instant(s.cached.RetrievedAt)) < 10*time.Minute {
		return *s.cached, nil
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("radar redirects are not followed") }}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, RadarURL, nil)
	if err != nil {
		return RadarSnapshot{}, ErrUnavailable
	}
	req.Header.Set("User-Agent", "Sub2API-PublicReference/1.0")
	req.Header.Set("Accept", "text/html")
	response, err := client.Do(req)
	if err != nil {
		return RadarSnapshot{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
		return RadarSnapshot{}, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4_000_001))
	if err != nil || len(raw) > 4_000_000 {
		return RadarSnapshot{}, ErrUnavailable
	}
	snapshot, err := ParseRadar(raw, time.Now())
	if err != nil {
		return RadarSnapshot{}, ErrUnavailable
	}
	if s.Open != nil {
		db, release, err := s.Open()
		if err != nil || db == nil || release == nil {
			return RadarSnapshot{}, ErrUnavailable
		}
		defer release()
		payload, err := json.Marshal(snapshot)
		if err != nil {
			return RadarSnapshot{}, ErrUnavailable
		}
		dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err = db.ExecContext(dbCtx, `INSERT INTO cost_center_public_references(source,retrieved_at,source_hash,payload) VALUES($1,$2,$3,$4::jsonb)`, snapshot.Source, snapshot.RetrievedAt, snapshot.SourceSHA256, string(payload))
		if err != nil {
			return RadarSnapshot{}, ErrUnavailable
		}
	}
	s.cached = &snapshot
	return snapshot, nil
}
