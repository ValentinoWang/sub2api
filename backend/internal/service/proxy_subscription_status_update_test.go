//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type proxyStatusAdminServiceStub struct {
	AdminService
	input *UpdateProxyInput
}

func (s *proxyStatusAdminServiceStub) UpdateProxy(_ context.Context, _ int64, input *UpdateProxyInput) (*Proxy, error) {
	s.input = input
	return &Proxy{}, nil
}

func TestProxySubscriptionStatusUpdateOnlyDeclaresStatus(t *testing.T) {
	admin := &proxyStatusAdminServiceStub{}
	svc := &ProxySubscriptionService{admin: admin}
	proxy := &Proxy{
		ID: 9, Name: "subscription node", Protocol: "socks5", Host: "old.example", Port: 1080,
		Username: "user", Password: "pass", Status: StatusActive, FallbackMode: FallbackModeDirect,
	}

	err := svc.setProxyStatus(context.Background(), proxy, "inactive")

	require.NoError(t, err)
	require.Equal(t, &UpdateProxyInput{Status: "inactive"}, admin.input)
}
