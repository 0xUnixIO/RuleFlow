package app

import "testing"

func TestAddCountryEmoji(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "prefers leftmost match over rule order",
			in:   "nya-SG-ISIF[HK]-COP-BAGE-01[1.0x]",
			want: "🇸🇬 nya-SG-ISIF[HK]-COP-BAGE-01[1.0x]",
		},
		{
			name: "matches chinese keyword",
			in:   "东京-01",
			want: "🇯🇵 东京-01",
		},
		{
			name: "matches chinese keyword followed by latin suffix",
			in:   "iku-日本W09",
			want: "🇯🇵 iku-日本W09",
		},
		{
			name: "matches case insensitive latin keyword",
			in:   "SeAttle-01",
			want: "🇺🇸 SeAttle-01",
		},
		{
			name: "does not double prefix existing emoji",
			in:   "🇭🇰 HK-01",
			want: "🇭🇰 HK-01",
		},
		{
			name: "keeps original name when no keyword matches",
			in:   "relay-node-01",
			want: "relay-node-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := addCountryEmoji(tc.in); got != tc.want {
				t.Fatalf("addCountryEmoji(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWordIndex(t *testing.T) {
	tests := []struct {
		name string
		text string
		word string
		idx  int
		ok   bool
	}{
		{
			name: "skips embedded letter matches",
			text: "bage-ru-just-ss-01",
			word: "us",
			idx:  -1,
			ok:   false,
		},
		{
			name: "accepts digits around keyword",
			text: "us1-hybrid",
			word: "us",
			idx:  0,
			ok:   true,
		},
		{
			name: "accepts symbol boundaries",
			text: "node[hk]-01",
			word: "hk",
			idx:  5,
			ok:   true,
		},
		{
			name: "rejects trailing letter boundary",
			text: "hkg-01",
			word: "hk",
			idx:  -1,
			ok:   false,
		},
		{
			name: "finds later valid occurrence after invalid one",
			text: "hkg hk-01",
			word: "hk",
			idx:  4,
			ok:   true,
		},
		{
			name: "accepts chinese keyword before latin suffix",
			text: "iku-日本w09",
			word: "日本",
			idx:  4,
			ok:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idx, ok := wordIndex(tc.text, tc.word)
			if idx != tc.idx || ok != tc.ok {
				t.Fatalf("wordIndex(%q, %q) = (%d, %v), want (%d, %v)", tc.text, tc.word, idx, ok, tc.idx, tc.ok)
			}
		})
	}
}

func TestStartsWithEmoji(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty string", in: "", want: false},
		{name: "emoji prefix", in: "🇸🇬 SG-01", want: true},
		{name: "ascii prefix", in: "SG-01", want: false},
		{name: "punctuation is not treated as emoji", in: "-SG-01", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := startsWithEmoji(tc.in); got != tc.want {
				t.Fatalf("startsWithEmoji(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestEnsureNodeName(t *testing.T) {
	t.Run("uses explicit name when different from server", func(t *testing.T) {
		node := &ProxyNode{
			Name:     "Tokyo-Edge",
			Server:   "1.1.1.1",
			Protocol: "vmess",
		}
		if got := ensureNodeName(node, 0); got != "🇯🇵 Tokyo-Edge" {
			t.Fatalf("ensureNodeName() = %q, want %q", got, "🇯🇵 Tokyo-Edge")
		}
	})

	t.Run("falls back to protocol index when name is empty", func(t *testing.T) {
		node := &ProxyNode{
			Server:   "1.1.1.1",
			Protocol: "trojan",
		}
		if got := ensureNodeName(node, 1); got != "TROJAN-2" {
			t.Fatalf("ensureNodeName() = %q, want %q", got, "TROJAN-2")
		}
	})
}
