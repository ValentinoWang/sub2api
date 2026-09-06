package membership

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type fakeVault struct {
	values  map[string]Credential
	deletes []string
	err     error
}

func (v *fakeVault) Put(_ context.Context, id string, credential Credential) error {
	if v.err != nil {
		return v.err
	}
	if v.values == nil {
		v.values = make(map[string]Credential)
	}
	v.values[id] = credential
	return nil
}

func (v *fakeVault) Get(_ context.Context, id string) (Credential, error) {
	if v.err != nil {
		return Credential{}, v.err
	}
	credential, ok := v.values[id]
	if !ok {
		return Credential{}, ErrCredential
	}
	return credential, nil
}

func (v *fakeVault) Delete(_ context.Context, id string) error {
	v.deletes = append(v.deletes, id)
	delete(v.values, id)
	return v.err
}

func (v *fakeVault) Ready(context.Context) error { return v.err }

type fakeAudit struct{ err error }

func (a fakeAudit) Store(context.Context, string, []byte) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return "audit/key", nil
}

type emptyAudit struct{}

func (emptyAudit) Store(context.Context, string, []byte) (string, error) { return "", nil }

type scriptedBrowser struct {
	calls  []BrowserInput
	result BrowserResult
	err    error
}

func (b *scriptedBrowser) Execute(_ context.Context, input BrowserInput) (BrowserResult, error) {
	b.calls = append(b.calls, input)
	return b.result, b.err
}

type submitBrowser struct{ calls []BrowserInput }

func (b *submitBrowser) Execute(_ context.Context, input BrowserInput) (BrowserResult, error) {
	b.calls = append(b.calls, input)
	switch input.Operation {
	case "validateCredential":
		return BrowserResult{
			State:      "valid",
			SKU:        input.SKU,
			PeriodDays: input.PeriodDays,
			AccountID:  input.Credential.AccountID,
		}, nil
	case "submitRecharge":
		return BrowserResult{State: "submitted", UpstreamTaskID: "upstream-1"}, nil
	default:
		return BrowserResult{}, errors.New("unexpected browser operation")
	}
}

func testKeyring(t *testing.T) *Keyring {
	t.Helper()
	k, err := ParseKeyring(`{"k1":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}`, "k1", "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=")
	if err != nil {
		t.Fatalf("ParseKeyring() error = %v", err)
	}
	return k
}

func TestParseCredentialAndResultNormalization(t *testing.T) {
	credential, err := parseCredential(CredentialInput{
		Mode:           "session",
		Value:          `{"accessToken":"token","account":{"id":"target_12"}}`,
		Consent:        true,
		ConsentVersion: ConsentVersion,
	})
	if err != nil {
		t.Fatalf("parseCredential() error = %v", err)
	}
	if credential.AccountID != "target_12" {
		t.Fatalf("account id = %q, want target_12", credential.AccountID)
	}
	if _, err := parseCredential(CredentialInput{Mode: "account_id", Value: "target_12"}); !errors.Is(err, ErrConsent) {
		t.Fatalf("missing consent error = %v, want ErrConsent", err)
	}

	result := NormalizeResult(BrowserResult{State: "succeeded", EntitlementVerified: false, ErrorCode: "untrusted"})
	if result.State != "review_required" || result.ErrorCode != "REVIEW_REQUIRED" {
		t.Fatalf("unverified success = %#v, want review-required", result)
	}
	result = NormalizeResult(BrowserResult{State: "unexpected", ErrorCode: "secret"})
	if result.State != "review_required" || result.ErrorCode != "REVIEW_REQUIRED" {
		t.Fatalf("unexpected browser output = %#v, want sanitized review", result)
	}
}

func TestApprovedPaymentRedirectTargetRejectsUnsafeProviderTargets(t *testing.T) {
	target, err := approvedPaymentRedirectTarget("https://pay.example.test/checkout", "")
	if err != nil || target != "https://pay.example.test/checkout" {
		t.Fatalf("approvedPaymentRedirectTarget() = %q, %v", target, err)
	}
	for _, raw := range []string{
		"javascript:alert(1)", "weixin://wxpay/bizpayurl?private=1", "data:text/html,private", "//pay.example.test/relative", "/checkout", "https://user@pay.example.test/checkout",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := approvedPaymentRedirectTarget(raw, "")
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("approvedPaymentRedirectTarget(%q) error = %v, want ErrUnavailable", raw, err)
			}
		})
	}
}

func TestCreatePaymentRedirectTicketStoresOnlyDigestAndFailsWithoutEntropy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	engine := &Engine{DB: db, redirectTicketRandom: func(raw []byte) (int, error) {
		for i := range raw {
			raw[i] = 7
		}
		return len(raw), nil
	}}
	mock.ExpectExec("DELETE FROM membership_payment_redirect_tickets").
		WillReturnError(errors.New("cleanup unavailable"))
	mock.ExpectExec("INSERT INTO membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(42), int64(9), int(membershipPaymentRedirectTicketTTL.Seconds())).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ticket, err := engine.CreatePaymentRedirectTicket(context.Background(), 42, "membership-order-1", 9)
	if err != nil {
		t.Fatalf("CreatePaymentRedirectTicket() error = %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil || len(raw) != 32 {
		t.Fatalf("ticket = %q, decoded length = %d, error = %v", ticket, len(raw), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	engine.redirectTicketRandom = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	if _, err = engine.CreatePaymentRedirectTicket(context.Background(), 42, "membership-order-1", 9); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("entropy failure = %v, want ErrUnavailable", err)
	}
}

func TestReissuePaymentRedirectTicketResolvesBoundPendingPayment(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	engine := &Engine{DB: db, redirectTicketRandom: func(raw []byte) (int, error) {
		for i := range raw {
			raw[i] = 8
		}
		return len(raw), nil
	}}

	mock.ExpectQuery("SELECT l.payment_order_id").
		WithArgs("membership-order-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(int64(9)))
	mock.ExpectExec("DELETE FROM membership_payment_redirect_tickets").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(42), int64(9), int(membershipPaymentRedirectTicketTTL.Seconds())).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ticket, paymentOrderID, err := engine.ReissuePaymentRedirectTicket(context.Background(), 42, "membership-order-1")
	if err != nil {
		t.Fatalf("ReissuePaymentRedirectTicket() error = %v", err)
	}
	if paymentOrderID != 9 {
		t.Fatalf("payment order id = %d, want 9", paymentOrderID)
	}
	if ticket != base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)) {
		t.Fatalf("ticket = %q", ticket)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReissuePaymentRedirectTicketRejectsUnusableBindings(t *testing.T) {
	for _, name := range []string{"wrong owner", "membership not pending", "payment not pending", "payment expired"} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New() error = %v", err)
			}
			defer db.Close()
			engine := &Engine{DB: db}
			mock.ExpectQuery("SELECT l.payment_order_id").
				WithArgs("membership-order-1", int64(42)).
				WillReturnError(sql.ErrNoRows)

			ticket, paymentOrderID, err := engine.ReissuePaymentRedirectTicket(context.Background(), 42, "membership-order-1")
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("ReissuePaymentRedirectTicket() error = %v, want ErrUnavailable", err)
			}
			if ticket != "" || paymentOrderID != 0 {
				t.Fatalf("reissue result = (%q, %d), want empty", ticket, paymentOrderID)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReissuePaymentRedirectTicketRecoversAfterInitialTicketInsertFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	engine := &Engine{DB: db, redirectTicketRandom: func(raw []byte) (int, error) {
		for i := range raw {
			raw[i] = 9
		}
		return len(raw), nil
	}}

	mock.ExpectExec("DELETE FROM membership_payment_redirect_tickets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(42), int64(9), int(membershipPaymentRedirectTicketTTL.Seconds())).
		WillReturnError(errors.New("ticket store unavailable"))
	if _, err := engine.CreatePaymentRedirectTicket(context.Background(), 42, "membership-order-1", 9); err == nil {
		t.Fatal("CreatePaymentRedirectTicket() error = nil, want ticket store error")
	}

	mock.ExpectQuery("SELECT l.payment_order_id").
		WithArgs("membership-order-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(int64(9)))
	mock.ExpectExec("DELETE FROM membership_payment_redirect_tickets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(42), int64(9), int(membershipPaymentRedirectTicketTTL.Seconds())).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ticket, paymentOrderID, err := engine.ReissuePaymentRedirectTicket(context.Background(), 42, "membership-order-1")
	if err != nil {
		t.Fatalf("ReissuePaymentRedirectTicket() error = %v", err)
	}
	if ticket == "" || paymentOrderID != 9 {
		t.Fatalf("reissue result = (%q, %d), want ticket and payment order 9", ticket, paymentOrderID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestKeyringBindsCiphertextToPurpose(t *testing.T) {
	k := testKeyring(t)
	ciphertext, err := k.Encrypt("credential:a", "sensitive")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := k.Decrypt("credential:a", ciphertext)
	if err != nil || plaintext != "sensitive" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := k.Decrypt("credential:b", ciphertext); !errors.Is(err, ErrCredential) {
		t.Fatalf("wrong purpose error = %v, want ErrCredential", err)
	}
}

func TestHTTPBrowserNormalizesUntrustedResponse(t *testing.T) {
	wantCredential := Credential{Mode: "session", Value: "sensitive-session", AccountID: "target_12"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+strings.Repeat("x", 32) {
			t.Errorf("authorization = %q", got)
		}
		if r.URL.Path != "/execute" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var input BrowserInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("decode browser input: %v", err)
		} else if input.Credential == nil || *input.Credential != wantCredential {
			t.Errorf("credential = %#v, want %#v", input.Credential, wantCredential)
		}
		_, _ = w.Write([]byte(`{"state":"succeeded","error_code":"unexpected"}`))
	}))
	defer server.Close()

	browser, err := NewHTTPBrowser(server.URL, strings.Repeat("x", 32))
	if err != nil {
		t.Fatalf("NewHTTPBrowser() error = %v", err)
	}
	result, err := browser.Execute(context.Background(), BrowserInput{Operation: "submitRecharge", Credential: &wantCredential})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.State != "review_required" || result.ErrorCode != "REVIEW_REQUIRED" {
		t.Fatalf("result = %#v, want sanitized review", result)
	}
}

func TestPutCredentialRejectsActiveTargetAndRemovesNewSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault := &fakeVault{}
	engine := &Engine{DB: db, Keys: testKeyring(t), Vault: vault, Browser: &scriptedBrowser{}, Audit: fakeAudit{}, Enabled: true}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT credential_mode,payment_state,channel,kind,target_hash,credential_ref FROM membership_orders").
		WithArgs("order-1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"credential_mode", "payment_state", "channel", "kind", "target_hash", "credential_ref"}).AddRow("account_id", "paid", "gpt", "customer", nil, nil))
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM membership_tasks WHERE target_hash").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	err = engine.PutCredential(context.Background(), 7, "order-1", CredentialInput{Mode: "account_id", Value: "target_12", Consent: true, ConsentVersion: ConsentVersion})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PutCredential() error = %v, want ErrConflict", err)
	}
	if len(vault.values) != 0 || len(vault.deletes) != 1 {
		t.Fatalf("vault state = values:%d deletes:%d, new credential must be removed", len(vault.values), len(vault.deletes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequestRefundFreezesTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT payment_state FROM membership_orders").WithArgs("order-1", int64(7)).WillReturnRows(sqlmock.NewRows([]string{"payment_state"}).AddRow("paid"))
	mock.ExpectExec("UPDATE membership_orders SET refund_requested_at").WithArgs("order-1", "not_delivered").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_tasks SET state='review_required'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "refund_requested", "7", "", "", "not_delivered").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := engine.RequestRefund(context.Background(), 7, "order-1", "not_delivered"); err != nil {
		t.Fatalf("RequestRefund() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestControlledEvidenceRefAcceptsOnlyDurableReferences(t *testing.T) {
	valid := []string{
		"evidence:0f8fad5b-d9cb-469f-a165-70867728950e",
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	for _, value := range valid {
		if !isControlledEvidenceRef(value) {
			t.Fatalf("isControlledEvidenceRef(%q) = false, want true", value)
		}
	}

	for _, value := range []string{
		"task-12345678",
		"CDK-ABCD-1234-EFGH-5678",
		"https://user:password@example.invalid/evidence",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature",
		"evidence:0F8FAD5B-D9CB-469F-A165-70867728950E",
		"sha256:0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef",
		"sha256:0123456789abcdef",
	} {
		if isControlledEvidenceRef(value) {
			t.Fatalf("isControlledEvidenceRef(%q) = true, want false", value)
		}
	}
}

func TestVerifyProductAndReviewRejectUncontrolledEvidenceRefs(t *testing.T) {
	engine := &Engine{}
	for _, evidence := range []string{
		"session=super-secret-session",
		"CDK-ABCD-1234-EFGH-5678",
		"https://example.invalid/evidence",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature",
		"task-12345678",
	} {
		if err := engine.VerifyProduct(context.Background(), "admin", "product-1", "run-1", evidence); !errors.Is(err, ErrInvalid) {
			t.Fatalf("VerifyProduct(%q) error = %v, want ErrInvalid", evidence, err)
		}
		if err := engine.Review(context.Background(), "admin", "order-1", "query", evidence); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Review(%q) error = %v, want ErrInvalid", evidence, err)
		}
	}
}

func TestRecoverPaymentCreationReleasesOnlyKnownAbsentGatewayCreate(t *testing.T) {
	tests := []struct {
		name      string
		uncertain bool
	}{
		{name: "known absent releases reservation", uncertain: false},
		{name: "unknown provider outcome preserves reservation", uncertain: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			engine := &Engine{DB: db}

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(int64(9)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("order-1", "pending"))
			if tt.uncertain {
				mock.ExpectExec("UPDATE membership_orders SET payment_state='manual_review'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_creation_unknown", "payment_gateway", "manual_review", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec("UPDATE membership_orders SET payment_state='closed'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE membership_cdks SET state='available',order_id=NULL").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE membership_coupons SET used=used-1").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_creation_failed", "payment_gateway", "closed", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			if err := engine.RecoverPaymentCreation(context.Background(), 9, tt.uncertain); err != nil {
				t.Fatalf("RecoverPaymentCreation() error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

const confirmPaymentLockQuery = `(?s)SELECT o\.id,o\.payment_state,o\.channel,po\.status,o\.coupon_code.*FOR UPDATE OF o,po`

func TestConfirmPaymentSettlesOrdinaryExpiredOrderToManualReviewWithoutTask(t *testing.T) {
	assertConfirmPaymentSettlesExpiredOrderToManualReview(t)
}

func TestConfirmPaymentSettlesExpiredOutsideGraceToManualReviewWithoutTask(t *testing.T) {
	assertConfirmPaymentSettlesExpiredOrderToManualReview(t)
}

func assertConfirmPaymentSettlesExpiredOrderToManualReview(t *testing.T) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(confirmPaymentLockQuery).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state", "channel", "status", "coupon_code", "input_required", "pay_amount", "price_minor", "target_hash"}).
			AddRow("order-1", "closed", "gpt", "EXPIRED", "SAVE10", false, 10.0, int64(1000), nil))
	mock.ExpectExec("UPDATE payment_orders SET status='PAID'").WithArgs(int64(9), "trade-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_coupons SET used=used\\+1").WithArgs("SAVE10").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_orders SET payment_state=\\$2").WithArgs("order-1", "manual_review").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_ledger").WithArgs(sqlmock.AnyArg(), "order-1", int64(1000), "9").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_confirmed", "payment_webhook", "manual_review", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err = engine.ConfirmPayment(context.Background(), 9, "trade-1"); err != nil {
		t.Fatalf("ConfirmPayment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmPaymentAlreadyPaidIsIdempotentAndDoesNotCreateTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(confirmPaymentLockQuery).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state", "channel", "status", "coupon_code", "input_required", "pay_amount", "price_minor", "target_hash"}).
			AddRow("order-1", "paid", "gpt", "PAID", nil, false, 10.0, int64(1000), nil))
	mock.ExpectCommit()

	if err := engine.ConfirmPayment(context.Background(), 9, "trade-1"); err != nil {
		t.Fatalf("ConfirmPayment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmPaymentSettlesLateSuccessToManualReviewWithoutTask(t *testing.T) {
	tests := []struct {
		name         string
		membership   string
		paymentState string
	}{
		{name: "released reservation after cancellation", membership: "closed", paymentState: "CANCELLED"},
		{name: "unknown gateway create outcome", membership: "manual_review", paymentState: "FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			engine := &Engine{DB: db}

			mock.ExpectBegin()
			coupon := any(nil)
			if tt.membership == "closed" {
				coupon = "SAVE10"
			}
			mock.ExpectQuery(confirmPaymentLockQuery).WithArgs(int64(9)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state", "channel", "status", "coupon_code", "input_required", "pay_amount", "price_minor", "target_hash"}).
					AddRow("order-1", tt.membership, "gpt", tt.paymentState, coupon, false, 10.0, int64(1000), nil))
			mock.ExpectExec("UPDATE payment_orders SET status='PAID'").WithArgs(int64(9), "trade-1").WillReturnResult(sqlmock.NewResult(0, 1))
			if tt.membership == "closed" {
				mock.ExpectExec("UPDATE membership_coupons SET used=used\\+1").WithArgs("SAVE10").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("UPDATE membership_orders SET payment_state=\\$2").WithArgs("order-1", "manual_review").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_ledger").WithArgs(sqlmock.AnyArg(), "order-1", int64(1000), "9").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_confirmed", "payment_webhook", "manual_review", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			if err := engine.ConfirmPayment(context.Background(), 9, "trade-1"); err != nil {
				t.Fatalf("ConfirmPayment() error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfirmPaymentSettlesReleasedOrderWithoutCouponToManualReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(confirmPaymentLockQuery).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state", "channel", "status", "coupon_code", "input_required", "pay_amount", "price_minor", "target_hash"}).
			AddRow("order-1", "closed", "gpt", "CANCELLED", nil, false, 10.0, int64(1000), nil))
	mock.ExpectExec("UPDATE payment_orders SET status='PAID'").WithArgs(int64(9), "trade-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_orders SET payment_state=\\$2").WithArgs("order-1", "manual_review").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_ledger").WithArgs(sqlmock.AnyArg(), "order-1", int64(1000), "9").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_confirmed", "payment_webhook", "manual_review", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err = engine.ConfirmPayment(context.Background(), 9, "trade-1"); err != nil {
		t.Fatalf("ConfirmPayment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmPaymentDuplicateManualReviewCallbackDoesNotRecountCoupon(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(confirmPaymentLockQuery).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state", "channel", "status", "coupon_code", "input_required", "pay_amount", "price_minor", "target_hash"}).
			AddRow("order-1", "manual_review", "gpt", "PAID", "SAVE10", false, 10.0, int64(1000), nil))
	mock.ExpectExec("UPDATE payment_orders SET status='PAID'").WithArgs(int64(9), "trade-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err = engine.ConfirmPayment(context.Background(), 9, "trade-1"); err != nil {
		t.Fatalf("ConfirmPayment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAbortRefundRestoresPaidAndFreezesCanceledTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("order-1", "refund_pending"))
	mock.ExpectExec("UPDATE membership_orders SET payment_state='paid'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_tasks SET state='review_required'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "refund_aborted", "payment_gateway", "paid", "REFUND_FAILED", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := engine.AbortRefund(context.Background(), 9); err != nil {
		t.Fatalf("AbortRefund() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExpireOrdersReturnsStockCouponAndCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault := &fakeVault{values: map[string]Credential{"credential-1": {Mode: "account_id", Value: "target_12", AccountID: "target_12"}}}
	engine := &Engine{DB: db, Vault: vault}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id,COALESCE\\(credential_ref,''\\) FROM membership_orders").WithArgs(int(OrderTTL.Seconds())).WillReturnRows(sqlmock.NewRows([]string{"id", "credential_ref"}).AddRow("order-1", "credential-1"))
	mock.ExpectExec("UPDATE membership_orders SET payment_state='closed'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_cdks SET state='available',order_id=NULL").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_coupons SET used=used-1").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "order_expired", "worker", "closed", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := engine.ExpireOrders(context.Background()); err != nil {
		t.Fatalf("ExpireOrders() error = %v", err)
	}
	if len(vault.deletes) != 1 || vault.deletes[0] != "credential-1" {
		t.Fatalf("deleted refs = %#v, want credential-1", vault.deletes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExportIntentRequiresDurableObjectKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := &Engine{DB: db, Audit: emptyAudit{}}
	mock.ExpectQuery("SELECT id,row_to_json\\(ev\\)::text FROM membership_events").WithArgs("attempt-1").WillReturnRows(sqlmock.NewRows([]string{"id", "raw"}).AddRow("event-1", `{}`))

	err = engine.exportIntent(context.Background(), "attempt-1")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("exportIntent() error = %v, want ErrUnavailable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

const queuedLeaseGate = `AND \(t\.state<>'queued' OR \(NOT p\.paused AND \(o\.kind='validation' OR o\.payment_state='paid'\)\)\)`
const submitGate = `SELECT \(NOT p\.paused AND \(o\.kind='validation' OR o\.payment_state='paid'\)\) AND o\.refund_requested_at IS NULL AND o\.credential_expires_at>now\(\)`
const stableLeaseOrder = `ORDER BY t\.next_run_at,t\.created_at,t\.id FOR UPDATE OF t SKIP LOCKED LIMIT 1`

func TestRunOneUsesStableLeaseOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keys := testKeyring(t)
	ciphertext, err := keys.Encrypt("cdk:cdk-earliest", "code")
	if err != nil {
		t.Fatal(err)
	}
	browser := &scriptedBrowser{err: errors.New("transient upstream failure")}
	engine := &Engine{DB: db, Keys: keys, Vault: &fakeVault{}, Browser: browser, Audit: fakeAudit{}, Enabled: true}

	mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	mock.ExpectBegin()
	mock.ExpectQuery(stableLeaseOrder).WithArgs("gpt").WillReturnRows(sqlmock.NewRows([]string{
		"id", "order_id", "state", "channel", "sku", "cdk_id", "ciphertext", "credential_ref", "kind", "period_days", "poll_seconds", "wait_seconds", "query_errors", "deadline_at", "paused",
	}).AddRow("task-earliest", "order-earliest", "submitted", "gpt", "sku-1", "cdk-earliest", ciphertext, "", "customer", 30, 5, 600, 0, nil, false))
	mock.ExpectExec("UPDATE membership_tasks SET lease_token").WithArgs("task-earliest", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,upstream_task_id FROM membership_attempts").WithArgs("task-earliest").WillReturnRows(sqlmock.NewRows([]string{"id", "upstream_task_id"}).AddRow("attempt-earliest", "upstream-earliest"))
	mock.ExpectExec("UPDATE membership_tasks SET query_errors=query_errors\\+1").WithArgs("task-earliest", sqlmock.AnyArg(), 5).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

	err = engine.RunOne(context.Background(), "gpt")
	if err == nil || err.Error() != "transient upstream failure" {
		t.Fatalf("RunOne() error = %v, want browser failure", err)
	}
	if len(browser.calls) != 1 || browser.calls[0].Operation != "queryStatus" || browser.calls[0].AttemptID != "attempt-earliest" {
		t.Fatalf("browser calls = %#v, want query for the stably selected task", browser.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunOnePausedQueuedTasksCannotLease(t *testing.T) {
	for _, orderKind := range []string{"customer", "validation"} {
		t.Run(orderKind, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			browser := &scriptedBrowser{}
			engine := &Engine{DB: db, Keys: testKeyring(t), Vault: &fakeVault{}, Browser: browser, Audit: fakeAudit{}, Enabled: true}

			mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectBegin()
			mock.ExpectQuery(queuedLeaseGate).WithArgs("gpt").WillReturnError(sql.ErrNoRows)
			mock.ExpectRollback()
			mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

			if err := engine.RunOne(context.Background(), "gpt"); err != nil {
				t.Fatalf("RunOne() error = %v", err)
			}
			if len(browser.calls) != 0 {
				t.Fatalf("browser calls = %#v, paused queued task must not be leased", browser.calls)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRunOnePausedBeforeSubmitCannotRecharge(t *testing.T) {
	for _, orderKind := range []string{"customer", "validation"} {
		t.Run(orderKind, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			keys := testKeyring(t)
			ciphertext, err := keys.Encrypt("cdk:cdk-1", "code")
			if err != nil {
				t.Fatal(err)
			}
			credential := Credential{AccountID: "target_12"}
			browser := &scriptedBrowser{result: BrowserResult{State: "valid", SKU: "sku-1", PeriodDays: 30, AccountID: credential.AccountID}}
			engine := &Engine{DB: db, Keys: keys, Vault: &fakeVault{values: map[string]Credential{"credential-1": credential}}, Browser: browser, Audit: fakeAudit{}, Enabled: true}

			mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectBegin()
			mock.ExpectQuery(queuedLeaseGate).WithArgs("gpt").WillReturnRows(sqlmock.NewRows([]string{
				"id", "order_id", "state", "channel", "sku", "cdk_id", "ciphertext", "credential_ref", "kind", "period_days", "poll_seconds", "wait_seconds", "query_errors", "deadline_at", "paused",
			}).AddRow("task-1", "order-1", "queued", "gpt", "sku-1", "cdk-1", ciphertext, "credential-1", orderKind, 30, 5, 600, 0, nil, false))
			mock.ExpectExec("UPDATE membership_tasks SET lease_token").WithArgs("task-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectBegin()
			mock.ExpectQuery(submitGate).WithArgs("order-1").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(false))
			mock.ExpectRollback()
			mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

			err = engine.RunOne(context.Background(), "gpt")
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("RunOne() error = %v, want ErrConflict", err)
			}
			if len(browser.calls) != 1 || browser.calls[0].Operation != "validateCredential" {
				t.Fatalf("browser calls = %#v, paused task must not submit recharge", browser.calls)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRunOneUnpausedValidationAndPaidCustomerCanSubmit(t *testing.T) {
	tests := []struct {
		name, orderKind string
	}{
		{name: "unpaused validation", orderKind: "validation"},
		{name: "paid customer", orderKind: "customer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			keys := testKeyring(t)
			ciphertext, err := keys.Encrypt("cdk:cdk-1", "code")
			if err != nil {
				t.Fatal(err)
			}
			credential := Credential{AccountID: "target_12"}
			vault := &fakeVault{values: map[string]Credential{"credential-1": credential}}
			browser := &submitBrowser{}
			engine := &Engine{DB: db, Keys: keys, Vault: vault, Browser: browser, Audit: fakeAudit{}, Enabled: true}

			mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectBegin()
			mock.ExpectQuery(queuedLeaseGate).WithArgs("gpt").WillReturnRows(sqlmock.NewRows([]string{
				"id", "order_id", "state", "channel", "sku", "cdk_id", "ciphertext", "credential_ref", "kind", "period_days", "poll_seconds", "wait_seconds", "query_errors", "deadline_at", "paused",
			}).AddRow("task-1", "order-1", "queued", "gpt", "sku-1", "cdk-1", ciphertext, "credential-1", tt.orderKind, 30, 5, 600, 0, nil, false))
			mock.ExpectExec("UPDATE membership_tasks SET lease_token").WithArgs("task-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectBegin()
			mock.ExpectQuery(submitGate).WithArgs("order-1").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			mock.ExpectExec("UPDATE membership_tasks SET state='submitted'").WithArgs("task-1", sqlmock.AnyArg(), 600).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_attempts").WithArgs(sqlmock.AnyArg(), "task-1", "cdk-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_cdks SET state='submitted'").WithArgs("cdk-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "submit_intent", "worker", "submitted", "", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectQuery("SELECT id,row_to_json\\(ev\\)::text FROM membership_events").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "raw"}).AddRow("event-1", `{}`))
			mock.ExpectExec("INSERT INTO membership_event_exports").WithArgs("event-1", "audit/key").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE membership_tasks SET state=\\$3").WithArgs("task-1", sqlmock.AnyArg(), "submitted", "", 5, "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_attempts SET state=\\$2").WithArgs(sqlmock.AnyArg(), "submitted", "", "upstream-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_cdks SET state=\\$2").WithArgs("cdk-1", "submitted").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_orders SET updated_at=now\\(\\)").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "fulfillment_updated", "worker", "submitted", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectExec("UPDATE membership_orders SET credential_ref=NULL").WithArgs("order-1", "credential-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

			if err := engine.RunOne(context.Background(), "gpt"); err != nil {
				t.Fatalf("RunOne() error = %v", err)
			}
			if len(browser.calls) != 2 || browser.calls[0].Operation != "validateCredential" || browser.calls[1].Operation != "submitRecharge" {
				t.Fatalf("browser calls = %#v, want validation followed by recharge submit", browser.calls)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRunOneRestartsSubmittedPausedProductWithQueryOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keys := testKeyring(t)
	ciphertext, err := keys.Encrypt("cdk:cdk-1", "code")
	if err != nil {
		t.Fatal(err)
	}
	browser := &scriptedBrowser{err: errors.New("transient upstream failure")}
	engine := &Engine{DB: db, Keys: keys, Vault: &fakeVault{}, Browser: browser, Audit: fakeAudit{}, Enabled: true}

	mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT t.id,t.order_id,t.state,t.channel,o.sku,c.id,c.ciphertext").WithArgs("gpt").WillReturnRows(sqlmock.NewRows([]string{
		"id", "order_id", "state", "channel", "sku", "cdk_id", "ciphertext", "credential_ref", "kind", "period_days", "poll_seconds", "wait_seconds", "query_errors", "deadline_at", "paused",
	}).AddRow("task-1", "order-1", "submitted", "gpt", "sku-1", "cdk-1", ciphertext, "", "customer", 30, 5, 600, 0, nil, true))
	mock.ExpectExec("UPDATE membership_tasks SET lease_token").WithArgs("task-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,upstream_task_id FROM membership_attempts").WithArgs("task-1").WillReturnRows(sqlmock.NewRows([]string{"id", "upstream_task_id"}).AddRow("attempt-1", "upstream-1"))
	mock.ExpectExec("UPDATE membership_tasks SET query_errors=query_errors\\+1").WithArgs("task-1", sqlmock.AnyArg(), 5).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

	err = engine.RunOne(context.Background(), "gpt")
	if expectationErr := mock.ExpectationsWereMet(); expectationErr != nil {
		t.Fatalf("RunOne() database expectations: %v", expectationErr)
	}
	if err == nil || err.Error() != "transient upstream failure" {
		t.Fatalf("RunOne() error = %v, want browser failure", err)
	}
	if len(browser.calls) != 1 || browser.calls[0].Operation != "queryStatus" || browser.calls[0].AttemptID != "attempt-1" {
		t.Fatalf("browser calls = %#v, restart must query the prior attempt only", browser.calls)
	}
}

func TestRunOneEscalatesThirdQueryFailureToReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keys := testKeyring(t)
	ciphertext, err := keys.Encrypt("cdk:cdk-1", "code")
	if err != nil {
		t.Fatal(err)
	}
	browser := &scriptedBrowser{err: errors.New("transient upstream failure")}
	engine := &Engine{DB: db, Keys: keys, Vault: &fakeVault{}, Browser: browser, Audit: fakeAudit{}, Enabled: true}

	mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs("membership:gpt").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT t.id,t.order_id,t.state,t.channel,o.sku,c.id,c.ciphertext").WithArgs("gpt").WillReturnRows(sqlmock.NewRows([]string{
		"id", "order_id", "state", "channel", "sku", "cdk_id", "ciphertext", "credential_ref", "kind", "period_days", "poll_seconds", "wait_seconds", "query_errors", "deadline_at", "paused",
	}).AddRow("task-1", "order-1", "processing", "gpt", "sku-1", "cdk-1", ciphertext, "", "customer", 30, 5, 600, 2, nil, false))
	mock.ExpectExec("UPDATE membership_tasks SET lease_token").WithArgs("task-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,upstream_task_id FROM membership_attempts").WithArgs("task-1").WillReturnRows(sqlmock.NewRows([]string{"id", "upstream_task_id"}).AddRow("attempt-1", "upstream-1"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE membership_tasks SET state=\\$3").WithArgs("task-1", sqlmock.AnyArg(), "review_required", "REVIEW_REQUIRED", 5, "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_attempts SET state=\\$2").WithArgs("attempt-1", "review_required", "REVIEW_REQUIRED", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_cdks SET state=\\$2").WithArgs("cdk-1", "uncertain").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_orders SET updated_at=now\\(\\)").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_products SET failures=failures\\+1").WithArgs("sku-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "fulfillment_updated", "worker", "review_required", "REVIEW_REQUIRED", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE membership_orders SET credential_ref=NULL").WithArgs("order-1", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs("membership:gpt").WillReturnResult(driver.ResultNoRows)

	if err := engine.RunOne(context.Background(), "gpt"); err != nil {
		t.Fatalf("RunOne() error = %v", err)
	}
	if len(browser.calls) != 1 || browser.calls[0].Operation != "queryStatus" {
		t.Fatalf("browser calls = %#v, want one status query", browser.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOrderTTLMatchesCredentialTTL(t *testing.T) {
	if OrderTTL != CredentialTTL {
		t.Fatalf("OrderTTL = %s, want credential TTL", OrderTTL)
	}
	if OrderTTL != 30*time.Minute {
		t.Fatalf("OrderTTL = %s, want 30 minutes", OrderTTL)
	}
}
