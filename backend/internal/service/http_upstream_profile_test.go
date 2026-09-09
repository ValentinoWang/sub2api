package service

import (
	"context"
	"testing"
)

func TestWithHTTPUpstreamProfile_DefaultKeepsContext(t *testing.T) {
	ctx := context.Background()
	got := WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileDefault)
	if got != ctx {
		t.Fatal("default profile should not wrap context")
	}
}

func TestWithHTTPUpstreamProfile_OpenAI(t *testing.T) {
	ctx := WithHTTPUpstreamProfile(context.TODO(), HTTPUpstreamProfileOpenAI)
	if profile := HTTPUpstreamProfileFromContext(ctx); profile != HTTPUpstreamProfileOpenAI {
		t.Fatalf("expected profile %q, got %q", HTTPUpstreamProfileOpenAI, profile)
	}
}

func TestWithHTTPUpstreamProfile_LongStream(t *testing.T) {
	ctx := WithHTTPUpstreamProfile(context.TODO(), HTTPUpstreamProfileLongStream)
	if profile := HTTPUpstreamProfileFromContext(ctx); profile != HTTPUpstreamProfileLongStream {
		t.Fatalf("expected profile %q, got %q", HTTPUpstreamProfileLongStream, profile)
	}
}

func TestWithLongStreamHTTPUpstreamProfileIsRequestScoped(t *testing.T) {
	base := context.Background()
	if got := withLongStreamHTTPUpstreamProfile(base, false); got != base {
		t.Fatal("non-stream request must retain its original context")
	}
	stream := withLongStreamHTTPUpstreamProfile(base, true)
	if profile := HTTPUpstreamProfileFromContext(stream); profile != HTTPUpstreamProfileLongStream {
		t.Fatalf("expected stream profile %q, got %q", HTTPUpstreamProfileLongStream, profile)
	}
	if profile := HTTPUpstreamProfileFromContext(base); profile != HTTPUpstreamProfileDefault {
		t.Fatalf("profile leaked to sibling request: %q", profile)
	}
}

func TestWithHTTPUpstreamRedirectsDisabled(t *testing.T) {
	//nolint:staticcheck // Exercises the defensive nil-context fallback.
	ctx := WithHTTPUpstreamRedirectsDisabled(nil)
	if !HTTPUpstreamRedirectsDisabled(ctx) {
		t.Fatal("expected redirects to be disabled")
	}
	if HTTPUpstreamRedirectsDisabled(context.Background()) {
		t.Fatal("redirects should remain enabled by default")
	}
}

func TestWithHTTPUpstreamPublicHostsOnly(t *testing.T) {
	//nolint:staticcheck // Exercises the defensive nil-context fallback.
	ctx := WithHTTPUpstreamPublicHostsOnly(nil)
	if !HTTPUpstreamPublicHostsOnly(ctx) {
		t.Fatal("expected public-hosts-only marker to be set")
	}
	if HTTPUpstreamPublicHostsOnly(context.Background()) {
		t.Fatal("marker must be absent by default")
	}
	if HTTPUpstreamRedirectsDisabled(ctx) {
		t.Fatal("public-hosts-only must not disable redirects")
	}
}
