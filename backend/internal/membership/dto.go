package membership

import "time"

// CustomerProductDTO is the complete customer-facing product contract.
// Keep this distinct from Product so operational provider metadata cannot be
// serialized when a new Product field is added.
type CustomerProductDTO struct {
	SKU            string `json:"sku"`
	Name           string `json:"name"`
	PriceMinor     int64  `json:"price_minor"`
	Currency       string `json:"currency"`
	PeriodDays     int    `json:"period_days"`
	CredentialMode string `json:"credential_mode"`
	ForSale        bool   `json:"for_sale"`
	Available      int    `json:"available"`
	ETAMinutes     int    `json:"eta_minutes"`
}

// CustomerOrderEventDTO deliberately excludes the immutable event ID, actor
// and evidence reference. Those values are audit data, never customer data.
type CustomerOrderEventDTO struct {
	Action    string    `json:"action"`
	State     string    `json:"state"`
	ErrorCode string    `json:"error_code"`
	CreatedAt time.Time `json:"created_at"`
}

// CustomerOrderDTO is the complete customer-facing order contract.
type CustomerOrderDTO struct {
	ID                string                  `json:"id"`
	SKU               string                  `json:"sku"`
	Name              string                  `json:"name"`
	Kind              string                  `json:"kind"`
	PaymentState      string                  `json:"payment_state"`
	FulfillmentState  string                  `json:"fulfillment_state"`
	PriceMinor        int64                   `json:"price_minor"`
	DiscountMinor     int64                   `json:"discount_minor"`
	PeriodDays        int                     `json:"period_days"`
	TargetMasked      string                  `json:"target_masked"`
	CredentialMode    string                  `json:"credential_mode"`
	InputRequired     bool                    `json:"input_required"`
	ErrorCode         string                  `json:"error_code"`
	RefundRequestedAt *time.Time              `json:"refund_requested_at"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	Events            []CustomerOrderEventDTO `json:"events"`
}

// AdminProductDTO contains only the operational product summary needed by the
// admin UI. It intentionally has no provider endpoint or secret fields.
type AdminProductDTO struct {
	SKU                string     `json:"sku"`
	Name               string     `json:"name"`
	PriceMinor         int64      `json:"price_minor"`
	Currency           string     `json:"currency"`
	PeriodDays         int        `json:"period_days"`
	CredentialMode     string     `json:"credential_mode"`
	ForSale            bool       `json:"for_sale"`
	Available          int        `json:"available"`
	ETAMinutes         int        `json:"eta_minutes"`
	Channel            string     `json:"channel"`
	Paused             bool       `json:"paused"`
	VerifiedAt         *time.Time `json:"verified_at"`
	InventoryCheckedAt *time.Time `json:"inventory_checked_at"`
	PollSeconds        int        `json:"poll_seconds"`
	WaitSeconds        int        `json:"wait_seconds"`
}

// AdminOrderDTO is separate from CustomerOrderDTO so the administrator
// contract can evolve independently without turning the customer response
// into an implicit projection of an internal order.
type AdminOrderDTO struct {
	ID                string                  `json:"id"`
	SKU               string                  `json:"sku"`
	Name              string                  `json:"name"`
	Kind              string                  `json:"kind"`
	PaymentState      string                  `json:"payment_state"`
	FulfillmentState  string                  `json:"fulfillment_state"`
	PriceMinor        int64                   `json:"price_minor"`
	DiscountMinor     int64                   `json:"discount_minor"`
	PeriodDays        int                     `json:"period_days"`
	TargetMasked      string                  `json:"target_masked"`
	CredentialMode    string                  `json:"credential_mode"`
	InputRequired     bool                    `json:"input_required"`
	ErrorCode         string                  `json:"error_code"`
	RefundRequestedAt *time.Time              `json:"refund_requested_at"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	Events            []CustomerOrderEventDTO `json:"events"`
}

type AdminStatsDTO struct {
	Total       int     `json:"total"`
	Success     int     `json:"success"`
	Review      int     `json:"review"`
	Queued      int     `json:"queued"`
	MeanSeconds float64 `json:"mean_seconds"`
}

// AdminOverviewDTO is a closed response contract for the membership console.
type AdminOverviewDTO struct {
	Products        []AdminProductDTO `json:"products"`
	Orders          []AdminOrderDTO   `json:"orders"`
	Stats           AdminStatsDTO     `json:"stats"`
	Ledger          map[string]int64  `json:"ledger"`
	RuntimeReady    bool              `json:"runtime_ready"`
	PaymentsEnabled bool              `json:"payments_enabled"`
	ConsentVersion  string            `json:"consent_version"`
}

func CustomerProductsDTO(products []Product) []CustomerProductDTO {
	out := make([]CustomerProductDTO, 0, len(products))
	for _, product := range products {
		out = append(out, customerProductDTO(product))
	}
	return out
}

func CustomerOrdersDTO(orders []Order) []CustomerOrderDTO {
	out := make([]CustomerOrderDTO, 0, len(orders))
	for _, order := range orders {
		out = append(out, CustomerOrderDTOFrom(order))
	}
	return out
}

func CustomerOrderDTOFrom(order Order) CustomerOrderDTO {
	paymentState, fulfillmentState, errorCode := normalizeOrderStates(order.PaymentState, order.FulfillmentState, order.ErrorCode)
	return CustomerOrderDTO{
		ID:                order.ID,
		SKU:               order.SKU,
		Name:              order.Name,
		Kind:              "customer",
		PaymentState:      paymentState,
		FulfillmentState:  fulfillmentState,
		PriceMinor:        order.PriceMinor,
		DiscountMinor:     order.DiscountMinor,
		PeriodDays:        order.PeriodDays,
		TargetMasked:      order.TargetMasked,
		CredentialMode:    normalizeCredentialMode(order.CredentialMode),
		InputRequired:     order.InputRequired,
		ErrorCode:         errorCode,
		RefundRequestedAt: order.RefundRequestedAt,
		CreatedAt:         order.CreatedAt,
		UpdatedAt:         order.UpdatedAt,
		Events:            customerEventsDTO(order.Events),
	}
}

func AdminOrderDTOFrom(order Order) AdminOrderDTO {
	paymentState, fulfillmentState, errorCode := normalizeOrderStates(order.PaymentState, order.FulfillmentState, order.ErrorCode)
	kind := "customer"
	if order.Kind == "validation" {
		kind = "validation"
	}
	return AdminOrderDTO{
		ID:                order.ID,
		SKU:               order.SKU,
		Name:              order.Name,
		Kind:              kind,
		PaymentState:      paymentState,
		FulfillmentState:  fulfillmentState,
		PriceMinor:        order.PriceMinor,
		DiscountMinor:     order.DiscountMinor,
		PeriodDays:        order.PeriodDays,
		TargetMasked:      order.TargetMasked,
		CredentialMode:    normalizeCredentialMode(order.CredentialMode),
		InputRequired:     order.InputRequired,
		ErrorCode:         errorCode,
		RefundRequestedAt: order.RefundRequestedAt,
		CreatedAt:         order.CreatedAt,
		UpdatedAt:         order.UpdatedAt,
		Events:            customerEventsDTO(order.Events),
	}
}

func AdminOverviewDTOFrom(overview AdminOverview) AdminOverviewDTO {
	products := make([]AdminProductDTO, 0, len(overview.Products))
	for _, product := range overview.Products {
		products = append(products, AdminProductDTO{
			SKU:                product.SKU,
			Name:               product.Name,
			PriceMinor:         product.PriceMinor,
			Currency:           product.Currency,
			PeriodDays:         product.PeriodDays,
			CredentialMode:     normalizeCredentialMode(product.CredentialMode),
			ForSale:            product.ForSale,
			Available:          product.Available,
			ETAMinutes:         product.ETAMinutes,
			Channel:            normalizeAdminChannel(product.Channel),
			Paused:             product.Paused,
			VerifiedAt:         product.VerifiedAt,
			InventoryCheckedAt: product.InventoryCheckedAt,
			PollSeconds:        product.PollSeconds,
			WaitSeconds:        product.WaitSeconds,
		})
	}
	orders := make([]AdminOrderDTO, 0, len(overview.Orders))
	for _, order := range overview.Orders {
		orders = append(orders, AdminOrderDTOFrom(order))
	}
	return AdminOverviewDTO{
		Products: products,
		Orders:   orders,
		Stats: AdminStatsDTO{
			Total:       overview.Stats.Total,
			Success:     overview.Stats.Success,
			Review:      overview.Stats.Review,
			Queued:      overview.Stats.Queued,
			MeanSeconds: overview.Stats.MeanSeconds,
		},
		Ledger:          overview.Ledger,
		RuntimeReady:    overview.RuntimeReady,
		PaymentsEnabled: overview.PaymentsEnabled,
		ConsentVersion:  ConsentVersion,
	}
}

func customerProductDTO(product Product) CustomerProductDTO {
	return CustomerProductDTO{
		SKU:            product.SKU,
		Name:           product.Name,
		PriceMinor:     product.PriceMinor,
		Currency:       product.Currency,
		PeriodDays:     product.PeriodDays,
		CredentialMode: normalizeCredentialMode(product.CredentialMode),
		ForSale:        product.ForSale,
		Available:      product.Available,
		ETAMinutes:     product.ETAMinutes,
	}
}

func customerEventsDTO(events []Event) []CustomerOrderEventDTO {
	out := make([]CustomerOrderEventDTO, 0, len(events))
	for _, event := range events {
		state, errorCode := normalizeCustomerState(event.State, event.ErrorCode)
		out = append(out, CustomerOrderEventDTO{
			Action:    normalizeEventAction(event.Action),
			State:     state,
			ErrorCode: errorCode,
			CreatedAt: event.CreatedAt,
		})
	}
	return out
}

func normalizeCustomerState(state, errorCode string) (string, string) {
	if !isCustomerState(state) {
		return "review_required", "REVIEW_REQUIRED"
	}
	if !isCustomerErrorCode(errorCode) {
		return "review_required", "REVIEW_REQUIRED"
	}
	return state, errorCode
}

func normalizeOrderStates(paymentState, fulfillmentState, errorCode string) (string, string, string) {
	if !isCustomerPaymentState(paymentState) {
		return "pending", "review_required", "REVIEW_REQUIRED"
	}
	fulfillmentState, errorCode = normalizeCustomerState(fulfillmentState, errorCode)
	return paymentState, fulfillmentState, errorCode
}

func isCustomerState(state string) bool {
	switch state {
	case "awaiting_input", "queued", "submitted", "processing", "review_required", "succeeded", "failed", "canceled", "pending", "refund_pending", "refunded", "closed":
		return true
	default:
		return false
	}
}

func isCustomerErrorCode(errorCode string) bool {
	switch errorCode {
	case "", "CREDENTIAL_EXPIRED", "ACCOUNT_NOT_ELIGIBLE", "OUT_OF_STOCK", "PROCESSING", "REVIEW_REQUIRED", "FULFILLMENT_FAILED", "NOT_SUPPORTED", "CHANNEL_UNAVAILABLE", "PAGE_CHANGED", "LOGIN_REQUIRED", "CAPTCHA_REQUIRED", "PRODUCT_MISMATCH":
		return true
	default:
		return false
	}
}

func isCustomerPaymentState(state string) bool {
	switch state {
	case "created", "pending", "paid", "closed", "refund_pending", "refunded":
		return true
	default:
		return false
	}
}

func normalizeCredentialMode(mode string) string {
	if mode == "session" {
		return "session"
	}
	return "account_id"
}

func normalizeAdminChannel(channel string) string {
	switch channel {
	case "gpt", "gptpro":
		return channel
	default:
		return "unknown"
	}
}

func normalizeEventAction(action string) string {
	switch action {
	case "order_created", "credential_received", "submit_intent", "input_required", "fulfillment_updated", "payment_pending", "payment_confirmed", "refund_prepared", "refund_completed", "refund_requested", "order_expired", "product_verified", "review_query", "review_confirm_success", "review_confirm_not_submitted", "review_retry", "review_cancel":
		return action
	default:
		return "review_required"
	}
}
