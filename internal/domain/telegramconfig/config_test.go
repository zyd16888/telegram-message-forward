package telegramconfig

import (
	"errors"
	"testing"
)

func TestNormalizeProxyType(t *testing.T) {
	tests := map[string]string{
		"socks5":  ProxyTypeSOCKS5,
		" HTTP ":  ProxyTypeHTTP,
		"https":   ProxyTypeHTTPS,
		" HTTPS ": ProxyTypeHTTPS,
	}
	for input, want := range tests {
		got, err := NormalizeProxyType(input)
		if err != nil {
			t.Fatalf("NormalizeProxyType(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeProxyType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeProxyTypeRejectsUnsupportedType(t *testing.T) {
	_, err := NormalizeProxyType("ss")
	if !errors.Is(err, ErrUnsupportedProxyType) {
		t.Fatalf("NormalizeProxyType(ss) error = %v", err)
	}
}
