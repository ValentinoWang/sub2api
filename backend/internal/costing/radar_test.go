package costing

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const radarFixture = `<section id="other"><strong>$999999.00</strong></section>
<section id="quota-radar" class="quota-radar"><h2>Quota Radar <span>Updated Sep 13, 20:15</span></h2>
<article class="quota-radar-current-card quota-radar-current-card-astra"><span>20x Pro · Astra only</span><strong>$1,847.00</strong><em>Provided by the site owner</em></article>
<article class="quota-radar-current-card quota-radar-current-card-sol"><span>20x Pro · Sol only</span><strong>$1,919.83</strong><em>Quota observed</em></article></section>
<section class="fast-radar" id="fast-radar"><div class="fast-radar-head"><h2>Fast Acceleration Radar</h2><span>GPT-6 Astra</span></div>
<div class="fast-simple-row" role="row" data-fast-simple-effort="low"><strong>Astra low</strong><span role="cell" class="fast-simple-standard" data-value-present="true">33.3</span><span role="cell" class="fast-simple-speed">62.1</span></div></section>`

type radarRoundTrip func(*http.Request) (*http.Response, error)

func (f radarRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRadarParserDoesNotInventPeriodPriceOrHistory(t *testing.T) {
	r, err := ParseRadar([]byte(radarFixture), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Quotas) != 2 || r.Quotas[0].ReferenceUSD != "1847.00" || r.Quotas[0].Tier != "pro20x" || r.Quotas[0].Period != nil || r.Quotas[0].PriceVersion != nil || r.SevenDayAverage != nil || r.TwoMonthResetRate != nil {
		t.Fatal(r)
	}
	if len(r.Speeds) != 1 || r.Speeds[0].FastTPS != "62.1" || r.SpeedModelLabel != "GPT-6 Astra" {
		t.Fatal(r)
	}
	if _, err := ParseRadar([]byte(strings.ReplaceAll(radarFixture, "quota-radar-current-card ", "new-card ")), time.Now()); err == nil {
		t.Fatal("changed quota schema accepted as empty data")
	}
}
func TestRadarFixedOriginAnonymousAndCache(t *testing.T) {
	calls := 0
	service := &RadarService{Client: &http.Client{Transport: radarRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != RadarURL || r.Method != "GET" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("private context forwarded to public source")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(radarFixture))}, nil
	})}}
	first, err := service.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Get(context.Background())
	if err != nil || calls != 1 || first.SourceSHA256 != second.SourceSHA256 {
		t.Fatal("cache did not preserve source", err)
	}
	service.cached = nil
	service.Client = &http.Client{Transport: radarRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("denied"))}, nil
	})}
	if _, err := service.Get(context.Background()); err == nil {
		t.Fatal("unavailable source became default data")
	}
}
