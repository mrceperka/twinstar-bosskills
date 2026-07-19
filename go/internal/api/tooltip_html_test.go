package api

import (
	"strings"
	"testing"
)

func TestSanitizeTooltipHTML(t *testing.T) {
	in := `<div class="item" style="color:red" onclick="steal()"><strong>Sword</strong><script>alert(1)</script><a href="javascript:alert(2)" onmouseover="steal()">bad</a><a href="https://example.com/item">good</a><form action="/steal"><input name="token"></form></div>`
	got := SanitizeTooltipHTML(in)

	for _, forbidden := range []string{"onclick", "onmouseover", "<script", "alert(1)", "javascript:", "<form", "<input"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("sanitized tooltip contains %q: %s", forbidden, got)
		}
	}
	for _, wanted := range []string{`class="item"`, `style="color: red"`, "<strong>Sword</strong>", `<a>bad</a>`, `href="https://example.com/item"`} {
		if !strings.Contains(got, wanted) {
			t.Errorf("sanitized tooltip missing %q: %s", wanted, got)
		}
	}
}

func TestSanitizeTooltipHTMLMalformedInput(t *testing.T) {
	got := SanitizeTooltipHTML(`<div><b>Epic<script>bad`)
	if strings.Contains(got, "bad") || !strings.Contains(got, "<b>Epic</b>") {
		t.Fatalf("unexpected sanitized tooltip: %s", got)
	}
}

func TestSanitizeTooltipHTMLKeepsPresentationAndSocketImage(t *testing.T) {
	in := `<b style="color: #a335ee; position: fixed">Ring</b><table width="100%" style="border-spacing: 0;"><tr><td style="padding: 0; text-align: right">Finger</td></tr></table><span style="padding-left:26px; background:url(https://twinstar-api.twinstar-wow.com/img/socket/socket_blue.gif) no-repeat left center; background-size: 14px 14px; color: #9d9d9d">Blue Socket</span><img src="https://twinstar-api.twinstar-wow.com/img/socket/socket_blue.gif" onerror="steal()">`
	got := SanitizeTooltipHTML(in)

	for _, wanted := range []string{"color: #a335ee", `width="100%"`, "border-spacing: 0", "text-align: right", "socket_blue.gif", "background-size: 14px 14px", "padding-left: 26px", "<img src="} {
		if !strings.Contains(got, wanted) {
			t.Errorf("sanitized tooltip missing %q: %s", wanted, got)
		}
	}
	for _, forbidden := range []string{"position:", "onerror"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("sanitized tooltip contains %q: %s", forbidden, got)
		}
	}
}
