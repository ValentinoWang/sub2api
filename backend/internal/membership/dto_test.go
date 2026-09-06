package membership

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCustomerDTOsUseWhitelistAndNormalizeUnsafeTaskData(t *testing.T) {
	now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
	providerHost := "supplier-raw.example.internal"
	unknownError := "UPSTREAM_" + providerHost
	order := Order{
		ID:               "customer-order-1",
		SKU:              "gpt-membership",
		Name:             "Membership",
		Kind:             "customer",
		PaymentState:     providerHost,
		FulfillmentState: providerHost,
		ErrorCode:        unknownError,
		TargetMasked:     "acct...1234",
		CredentialMode:   "session",
		CreatedAt:        now,
		UpdatedAt:        now,
		Events: []Event{{
			ID:        "event-id-must-not-leak",
			Action:    providerHost,
			State:     "unrecognized_provider_state",
			ErrorCode: unknownError,
			CreatedAt: now,
		}},
	}

	body, err := json.Marshal(struct {
		Products []CustomerProductDTO `json:"products"`
		Orders   []CustomerOrderDTO   `json:"orders"`
		Order    CustomerOrderDTO     `json:"order"`
	}{
		Products: CustomerProductsDTO([]Product{{
			SKU: "gpt-membership", Name: "Membership", CredentialMode: "unexpected-provider-mode", Channel: providerHost,
		}}),
		Orders: []CustomerOrderDTO{CustomerOrderDTOFrom(order)},
		Order:  CustomerOrderDTOFrom(order),
	})
	if err != nil {
		t.Fatalf("marshal customer DTOs: %v", err)
	}
	jsonBody := string(body)
	for _, forbidden := range []string{providerHost, unknownError, "event-id-must-not-leak", "upstream_task_id", "raw_response", "cdk", "credential_ref"} {
		if strings.Contains(jsonBody, forbidden) {
			t.Fatalf("customer JSON leaked %q: %s", forbidden, jsonBody)
		}
	}
	if got := CustomerOrderDTOFrom(order); got.PaymentState != "pending" || got.FulfillmentState != "review_required" || got.ErrorCode != "REVIEW_REQUIRED" {
		t.Fatalf("unknown order states/error = %q/%q/%q, want pending/review_required/REVIEW_REQUIRED", got.PaymentState, got.FulfillmentState, got.ErrorCode)
	}
	if got := CustomerOrderDTOFrom(order).Events[0]; got.Action != "review_required" || got.State != "review_required" || got.ErrorCode != "REVIEW_REQUIRED" {
		t.Fatalf("unknown event projection = %#v, want normalized review fields", got)
	}
	if rawOrder, err := json.Marshal(order); err != nil || string(rawOrder) != "{}" {
		t.Fatalf("domain order must fail closed outside DTO mapping: %s, %v", rawOrder, err)
	}
}

func TestAdminDTORetainsSummaryWithoutSensitiveOrderFields(t *testing.T) {
	now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
	overview := AdminOverview{
		Products: []Product{{SKU: "membership", Name: "Membership", Channel: "supplier.example", PollSeconds: 30, WaitSeconds: 600}},
		Orders: []Order{{
			ID: "order-1", Kind: "validation", FulfillmentState: "processing", ErrorCode: "", Events: []Event{{ID: "event-secret", Action: "submit_intent", State: "processing", CreatedAt: now}},
		}},
		Stats:  AdminStats{Total: 1, Queued: 1},
		Ledger: map[string]int64{"payment": 100},
	}
	body, err := json.Marshal(AdminOverviewDTOFrom(overview))
	if err != nil {
		t.Fatalf("marshal admin DTO: %v", err)
	}
	jsonBody := string(body)
	for _, forbidden := range []string{"supplier.example", "event-secret", "upstream_task_id", "raw_response", "cdk", "credential_ref"} {
		if strings.Contains(jsonBody, forbidden) {
			t.Fatalf("admin JSON leaked %q: %s", forbidden, jsonBody)
		}
	}
	if !strings.Contains(jsonBody, `"poll_seconds":30`) || !strings.Contains(jsonBody, `"channel":"unknown"`) {
		t.Fatalf("admin summary missing or unsafe: %s", jsonBody)
	}
}
