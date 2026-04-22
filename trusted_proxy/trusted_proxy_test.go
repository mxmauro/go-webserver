// See the LICENSE file for license details.

package trusted_proxy

import (
	"net"
	"testing"
)

// -----------------------------------------------------------------------------

func TestTrustedProxyIsIpTrusted(t *testing.T) {
	tp := NewTrustedProxy([]string{
		"10.0.0.1",
		"192.168.0.0/24",
		"invalid-cidr",
	})

	tests := []struct {
		ip       string
		expected bool
	}{
		{ip: "10.0.0.1", expected: true},
		{ip: "192.168.0.42", expected: true},
		{ip: "10.0.0.2", expected: false},
		{ip: "192.168.1.1", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.ip, func(t *testing.T) {
			if trusted := tp.IsIpTrusted(net.ParseIP(tc.ip)); trusted != tc.expected {
				t.Fatalf("unexpected trust result [got:%v / expected:%v]", trusted, tc.expected)
			}
		})
	}
}
