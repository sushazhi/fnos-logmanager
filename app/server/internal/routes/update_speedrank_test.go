package routes

import (
	"testing"
)

func TestClassifyMirrorURL(t *testing.T) {
	cases := []struct {
		url  string
		want mirrorTier
	}{
		{"https://gh-proxy.com/https://github.com/o/r/releases/download/v1/x.fpk", tierMulti},
		{"https://v4.gh-proxy.org/https://github.com/o/r/releases/download/v1/x.fpk", tierNode},
		{"https://ghfast.top/https://github.com/o/r/releases/download/v1/x.fpk", tierVolatile},
		{"https://ghproxy.imciel.com/https://github.com/o/r/releases/download/v1/x.fpk", tierVolatile},
		{"https://github.com/o/r/releases/download/v1/x.fpk", tierDirect},
		{"https://objects.githubusercontent.com/xyz/x.fpk", tierDirect},
		{"https://gh.unheard-of.example/https://github.com/o/r/x.fpk", tierUnknown},
	}
	for _, c := range cases {
		if got := classifyMirrorURL(c.url); got != c.want {
			t.Errorf("classifyMirrorURL(%q) = %s, want %s", c.url, tierName(got), tierName(c.want))
		}
	}
}

func TestOrderCandidatesForSpeed(t *testing.T) {
	direct := "https://github.com/o/r/releases/download/v1/x.fpk"
	in := []string{
		direct,
		"https://ghfast.top/" + direct,
		"https://v4.gh-proxy.org/" + direct,
		"https://gh-proxy.com/" + direct,
		"https://gh-proxy.com/" + direct, // 重复候选应被去重
		"",
	}
	got := orderCandidatesForSpeed(in)
	want := []string{
		"https://gh-proxy.com/" + direct,
		"https://v4.gh-proxy.org/" + direct,
		"https://ghfast.top/" + direct,
		direct,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %v, want %v", i, got, want)
		}
	}
}

func TestOrderCandidatesStableWithinTier(t *testing.T) {
	a := "https://gh-proxy.com/https://github.com/o/r/a.fpk"
	b := "https://gh-proxy.com/https://github.com/o/r/b.fpk"
	got := orderCandidatesForSpeed([]string{b, a})
	if len(got) != 2 || got[0] != b || got[1] != a {
		t.Fatalf("same-tier order not stable: %v", got)
	}
}

func TestOrderCandidatesRejectsNothing(t *testing.T) {
	// 排序只做顺序调整，不做安全过滤：不安全地址由调用方校验后才会被请求。
	bad := "http://insecure.example/x.fpk"
	got := orderCandidatesForSpeed([]string{bad})
	if len(got) != 1 || got[0] != bad {
		t.Fatalf("orderCandidatesForSpeed should keep input intact, got %v", got)
	}
}

func TestMirrorAllowedHosts(t *testing.T) {
	for _, u := range []string{
		"https://gh-proxy.com/https://github.com/o/r/x.fpk",
		"https://v4.gh-proxy.org/https://github.com/o/r/x.fpk",
		"https://ghfast.top/https://github.com/o/r/x.fpk",
		"https://ghproxy.imciel.com/https://github.com/o/r/x.fpk",
		"https://github.com/o/r/x.fpk",
	} {
		if !isAllowedHost(u) || !isHTTPSURL(u) {
			t.Errorf("candidate %q should be allowed", u)
		}
	}
	for _, u := range []string{
		"https://gh.dpik.top/https://github.com/o/r/x.fpk", // 已淘汰
		"http://gh-proxy.com/https://github.com/o/r/x.fpk", // 非 HTTPS
		"https://evil.example/https://github.com/o/r/x.fpk",
	} {
		if isAllowedHost(u) && isHTTPSURL(u) {
			t.Errorf("candidate %q should be rejected", u)
		}
	}
}
