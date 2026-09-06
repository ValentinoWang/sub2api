package membership

import (
	"context"
	"errors"
	"time"
)

const ConsentVersion = "membership-2026-09-07-v1"
const CredentialTTL = 30 * time.Minute

// OrderTTL bounds an unpaid stock reservation. It is intentionally no longer
// than the credential lifetime, so an abandoned checkout cannot retain stock.
const OrderTTL = CredentialTTL

var (
	ErrInvalid     = errors.New("MEMBERSHIP_INVALID_INPUT")
	ErrUnavailable = errors.New("MEMBERSHIP_UNAVAILABLE")
	ErrNotFound    = errors.New("MEMBERSHIP_NOT_FOUND")
	ErrConflict    = errors.New("MEMBERSHIP_CONFLICT")
	ErrConsent     = errors.New("MEMBERSHIP_CONSENT_REQUIRED")
	ErrCredential  = errors.New("CREDENTIAL_EXPIRED")
	ErrReview      = errors.New("REVIEW_REQUIRED")
)

type Product struct {
	SKU                string     `json:"-"`
	Name               string     `json:"-"`
	PriceMinor         int64      `json:"-"`
	Currency           string     `json:"-"`
	PeriodDays         int        `json:"-"`
	CredentialMode     string     `json:"-"`
	ForSale            bool       `json:"-"`
	Available          int        `json:"-"`
	ETAMinutes         int        `json:"-"`
	Channel            string     `json:"-"`
	Paused             bool       `json:"-"`
	VerifiedAt         *time.Time `json:"-"`
	InventoryCheckedAt *time.Time `json:"-"`
	PollSeconds        int        `json:"-"`
	WaitSeconds        int        `json:"-"`
}

type Order struct {
	ID                string     `json:"-"`
	SKU               string     `json:"-"`
	Name              string     `json:"-"`
	Kind              string     `json:"-"`
	PaymentState      string     `json:"-"`
	FulfillmentState  string     `json:"-"`
	PriceMinor        int64      `json:"-"`
	DiscountMinor     int64      `json:"-"`
	PeriodDays        int        `json:"-"`
	TargetMasked      string     `json:"-"`
	CredentialMode    string     `json:"-"`
	InputRequired     bool       `json:"-"`
	ErrorCode         string     `json:"-"`
	RefundRequestedAt *time.Time `json:"-"`
	CreatedAt         time.Time  `json:"-"`
	UpdatedAt         time.Time  `json:"-"`
	Events            []Event    `json:"-"`
}

type Event struct {
	ID        string    `json:"-"`
	Action    string    `json:"-"`
	State     string    `json:"-"`
	ErrorCode string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

type CredentialInput struct {
	Mode           string `json:"mode"`
	Value          string `json:"value"`
	AccountID      string `json:"account_id"`
	ConsentVersion string `json:"consent_version"`
	Consent        bool   `json:"consent"`
}

type Credential struct {
	Mode      string `json:"mode"`
	Value     string `json:"value"`
	AccountID string `json:"account_id"`
}

type Capabilities struct {
	Cancel    bool `json:"cancel"`
	Refresh   bool `json:"refresh"`
	AccountID bool `json:"account_id"`
	Session   bool `json:"session"`
}

// Browser input never travels through a customer response or durable queue payload.
type BrowserInput struct {
	Operation      string      `json:"operation"`
	AttemptID      string      `json:"attempt_id"`
	Channel        string      `json:"channel"`
	SKU            string      `json:"sku"`
	PeriodDays     int         `json:"period_days"`
	CDK            string      `json:"cdk,omitempty"`
	Credential     *Credential `json:"credential,omitempty"`
	UpstreamTaskID string      `json:"upstream_task_id,omitempty"`
}

type BrowserResult struct {
	State               string `json:"state"`
	ErrorCode           string `json:"error_code"`
	UpstreamTaskID      string `json:"upstream_task_id"`
	Available           *int   `json:"available"`
	SKU                 string `json:"sku"`
	PeriodDays          int    `json:"period_days"`
	AccountID           string `json:"account_id"`
	NotSubmitted        bool   `json:"not_submitted"`
	EntitlementVerified bool   `json:"entitlement_verified"`
}

type Browser interface {
	Execute(context.Context, BrowserInput) (BrowserResult, error)
}
type Vault interface {
	Put(context.Context, string, Credential) error
	Get(context.Context, string) (Credential, error)
	Delete(context.Context, string) error
	Ready(context.Context) error
}
type AuditSink interface {
	Store(context.Context, string, []byte) (string, error)
}
type Notifier func(context.Context, int64, string, string) error

func NormalizeResult(r BrowserResult) BrowserResult {
	switch r.ErrorCode {
	case "", "CREDENTIAL_EXPIRED", "ACCOUNT_NOT_ELIGIBLE", "OUT_OF_STOCK", "PROCESSING", "REVIEW_REQUIRED", "FULFILLMENT_FAILED", "NOT_SUPPORTED", "CHANNEL_UNAVAILABLE", "PAGE_CHANGED", "LOGIN_REQUIRED", "CAPTCHA_REQUIRED", "PRODUCT_MISMATCH":
	default:
		r.ErrorCode = "REVIEW_REQUIRED"
	}
	switch r.State {
	case "available", "valid", "submitted", "processing", "failed", "review_required", "not_submitted":
	case "succeeded":
		if !r.EntitlementVerified {
			r.State, r.ErrorCode = "review_required", "REVIEW_REQUIRED"
		}
	default:
		r.State, r.ErrorCode = "review_required", "REVIEW_REQUIRED"
	}
	if r.State != "not_submitted" {
		r.NotSubmitted = false
	}
	return r
}
