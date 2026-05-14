package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigResponseMetaForTarget(t *testing.T) {
	tests := []struct {
		target          string
		wantFilename    string
		wantContentType string
	}{
		{target: "clash-mihomo", wantFilename: "clash-mihomo_config.yaml", wantContentType: "text/yaml; charset=utf-8"},
		{target: "stash", wantFilename: "stash_config.yaml", wantContentType: "text/yaml; charset=utf-8"},
		{target: "surge", wantFilename: "surge_config.conf", wantContentType: "text/plain; charset=utf-8"},
		{target: "sing-box", wantFilename: "sing_box_config.json", wantContentType: "application/json; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := configResponseMetaForTarget(tt.target)
			if got.filename != tt.wantFilename || got.contentType != tt.wantContentType {
				t.Fatalf("target=%s 时，期望 filename=%q contentType=%q，实际为 filename=%q contentType=%q",
					tt.target, tt.wantFilename, tt.wantContentType, got.filename, got.contentType)
			}
		})
	}
}

func TestWriteConfigResponseAppliesHeadersAndFinalizer(t *testing.T) {
	req := httptest.NewRequest("GET", "https://sub.example.com/config?token=abc", nil)
	rec := httptest.NewRecorder()

	writeConfigResponse(rec, req, "surge", "[General]\nloglevel = notify", 3)

	res := rec.Result()
	if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := res.Header.Get("Content-Disposition"); got != `inline; filename="surge_config.conf"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := res.Header.Get("X-Node-Count"); got != "3" {
		t.Fatalf("X-Node-Count = %q, want 3", got)
	}

	body := rec.Body.String()
	if !strings.HasPrefix(body, "#!MANAGED-CONFIG https://sub.example.com/config?token=abc") {
		t.Fatalf("Surge 响应应自动注入 managed header，实际内容为:\n%s", body)
	}
	if !strings.Contains(body, "[General]\nloglevel = notify") {
		t.Fatalf("响应正文缺少配置内容，实际为:\n%s", body)
	}
}
