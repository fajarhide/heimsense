package config

import "testing"

func TestValidate(t *testing.T) {
	cases := []struct {
		name      string
		addr      string
		proxyKey  string
		wantError bool
	}{
		{"loopback ip, no key", "127.0.0.1:8080", "", false},
		{"loopback name, no key", "localhost:8080", "", false},
		{"ipv6 loopback, no key", "[::1]:8080", "", false},
		{"all interfaces, no key", ":8080", "", true},
		{"all interfaces explicit, no key", "0.0.0.0:8080", "", true},
		{"routable address, no key", "192.0.2.10:8080", "", true},
		{"all interfaces, with key", ":8080", "secret", false},
		{"routable address, with key", "0.0.0.0:8080", "secret", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{ListenAddr: tc.addr, ProxyAPIKey: tc.proxyKey}
			err := cfg.Validate()

			if tc.wantError && err == nil {
				t.Errorf("Validate() = nil, want an error for %q without a proxy key", tc.addr)
			}
			if !tc.wantError && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}
