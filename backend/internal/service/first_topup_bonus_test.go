package service

import "testing"

func TestParseFirstTopupBonusTiers(t *testing.T) {
	tiers := ParseFirstTopupBonusTiers(`[{"min_amount":50,"bonus_amount":15},{"min_amount":10,"bonus_amount":2},{"min_amount":0,"bonus_amount":9},{"min_amount":100,"bonus_amount":-1}]`)
	if len(tiers) != 2 {
		t.Fatalf("expected 2 valid tiers, got %d", len(tiers))
	}
	if tiers[0].MinAmount != 10 || tiers[1].MinAmount != 50 {
		t.Fatalf("expected ascending order, got %+v", tiers)
	}
	if ParseFirstTopupBonusTiers("") != nil || ParseFirstTopupBonusTiers("not json") != nil {
		t.Fatalf("empty or invalid input must yield nil")
	}
}

func TestPickFirstTopupBonus(t *testing.T) {
	tiers := ParseFirstTopupBonusTiers(`[{"min_amount":10,"bonus_amount":2},{"min_amount":50,"bonus_amount":15}]`)
	cases := map[float64]float64{5: 0, 10: 2, 49.99: 2, 50: 15, 500: 15}
	for amount, want := range cases {
		if got := PickFirstTopupBonus(tiers, amount); got != want {
			t.Errorf("amount %v: want %v got %v", amount, want, got)
		}
	}
	if PickFirstTopupBonus(nil, 100) != 0 {
		t.Errorf("no tiers must yield 0")
	}
}
