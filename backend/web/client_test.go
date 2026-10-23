package web

import (
	"net"
	"testing"
)

func TestClientKind(t *testing.T) {
	const chrome = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/129 Mobile Safari/537.36"
	cases := []struct {
		ua, fetchSite, want string
	}{
		{"ConexiuniClujMobile/1.4.0", "", "android/1.4.0"},
		{"ConexiuniClujWear/1.0.2", "", "wear/1.0.2"},
		{"ConexiuniClujMobile/<junk>", "", "android"},
		{chrome, "same-origin", "web"},
		{chrome, "", ""},
		{chrome, "cross-site", ""},
		{chrome, "none", ""},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "same-origin", ""},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 HeadlessChrome/129 Safari/537.36", "same-origin", ""},
		{"python-requests/2.32", "", ""},
		{"ESP32HTTPClient", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := ClientKind(c.ua, c.fetchSite); got != c.want {
			t.Errorf("ClientKind(%q, %q) = %q, want %q", c.ua, c.fetchSite, got, c.want)
		}
	}
}

func TestIPv6ClientsCountPerSlash64(t *testing.T) {
	same := []string{"2a02:2f0c:8a12:3400::1", "2a02:2f0c:8a12:3400:abcd:1234:5678:9abc"}
	if ClientKey(same[0]) != ClientKey(same[1]) {
		t.Errorf("one /64 gives two clients: %s, %s", ClientKey(same[0]), ClientKey(same[1]))
	}
	if ClientKey("2a02:2f0c:8a12:3401::1") == ClientKey(same[0]) {
		t.Error("two /64s give one client")
	}
	if got := ClientKey("188.24.64.220"); got != "188.24.64.220" {
		t.Errorf("IPv4 changed to %s", got)
	}
}

func TestIPv6NetworksArePerSlash48(t *testing.T) {
	if NetworkKey("2a02:2f0c:8a12:3400::1") != NetworkKey("2a02:2f0c:8a12:ffff::1") {
		t.Error("one /48 gives two networks")
	}
	if NetworkKey("2a02:2f0c:8a13::1") == NetworkKey("2a02:2f0c:8a12::1") {
		t.Error("two /48s give one network")
	}
	if got := NetworkKey("188.24.64.220"); got != "188.24.64.220" {
		t.Errorf("IPv4 changed to %s", got)
	}
}

func TestClientIPTrustsOnlyTheEntryNginxAdded(t *testing.T) {
	loopback := net.ParseIP("127.0.0.1")
	cases := []struct {
		name         string
		peer         net.IP
		forwardedFor string
		want         string
	}{
		{"through nginx", loopback, "203.0.113.7", "203.0.113.7"},
		{"made-up entries before nginx's", loopback, "1.2.3.4, 198.51.100.9, 203.0.113.7", "203.0.113.7"},
		{"ipv6 client", loopback, "2001:db8::1", "2001:db8::1"},
		{"nothing forwarded", loopback, "", "127.0.0.1"},
		{"garbage forwarded", loopback, "not-an-ip", "127.0.0.1"},
		{"straight to the backend", net.ParseIP("198.51.100.20"), "203.0.113.7", "198.51.100.20"},
	}
	for _, tc := range cases {
		if got := ClientIPFrom(tc.peer, tc.forwardedFor); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
