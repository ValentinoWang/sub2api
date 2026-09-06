package service

import "github.com/Wei-Shaw/sub2api/internal/membership"

// SetMembershipEngine attaches the independent fulfillment domain at startup.
func (s *PaymentService) SetMembershipEngine(engine *membership.Engine) { s.membership = engine }
