package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFinalizeConfigContentAddsManagedHeaderForSurge(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost:8080/config?token=abc", nil)
	content := "[General]\nloglevel = notify"

	got := finalizeConfigContent(req, "surge", content)

	want := "#!MANAGED-CONFIG http://localhost:8080/config?token=abc interval=43200 strict=false\n" + content
	if got != want {
		t.Fatalf("期望内容为:\n%s\n\n实际内容为:\n%s", want, got)
	}
}

func TestFinalizeConfigContentUsesForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/config?token=abc", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "sub.example.com")

	got := buildSurgeManagedConfigLine(req)
	want := "#!MANAGED-CONFIG https://sub.example.com/config?token=abc interval=43200 strict=false"
	if got != want {
		t.Fatalf("期望 managed-config 地址为 %q，实际为 %q", want, got)
	}
}

func TestFinalizeConfigContentUsesEnvBaseURLFirst(t *testing.T) {
	t.Setenv(publicBaseURLEnv, "https://public.example.com")

	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/config?token=abc", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "sub.example.com")

	got := buildSurgeManagedConfigLine(req)
	want := "#!MANAGED-CONFIG https://public.example.com/config?token=abc interval=43200 strict=false"
	if got != want {
		t.Fatalf("期望环境变量优先生效，实际为 %q", got)
	}
}

func TestFinalizeConfigContentFallsBackWhenEnvBaseURLInvalid(t *testing.T) {
	t.Setenv(publicBaseURLEnv, "://bad-url")

	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/config?token=abc", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "sub.example.com")

	got := buildSurgeManagedConfigLine(req)
	want := "#!MANAGED-CONFIG https://sub.example.com/config?token=abc interval=43200 strict=false"
	if got != want {
		t.Fatalf("期望非法环境变量回退到请求地址，实际为 %q", got)
	}
}

func TestFinalizeConfigContentDoesNotDuplicateManagedHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost:8080/config?token=abc", nil)
	content := buildSurgeManagedConfigLine(req) + "\n[General]\nloglevel = notify"

	got := finalizeConfigContent(req, "surge", content)

	if got != content {
		t.Fatalf("不应重复插入 MANAGED-CONFIG 头，实际内容为:\n%s", got)
	}
}

func TestFinalizeConfigContentReplacesExistingManagedHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost:8080/config?token=abc", nil)
	content := "#!MANAGED-CONFIG https://old.example.com/config?token=old interval=43200 strict=false\n[General]\nloglevel = notify"

	got := finalizeConfigContent(req, "surge", content)
	want := buildSurgeManagedConfigLine(req) + "\n[General]\nloglevel = notify"
	if got != want {
		t.Fatalf("期望替换旧的 MANAGED-CONFIG 头，实际内容为:\n%s", got)
	}
}

func TestFinalizeConfigContentLeavesNonSurgeUntouched(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost:8080/config?token=abc", nil)
	content := "proxies:\n  - name: test"

	got := finalizeConfigContent(req, "clash", content)

	if got != content {
		t.Fatalf("非 Surge 配置不应被修改，实际内容为:\n%s", got)
	}
}

func TestReplaceTemplateRuntimePlaceholdersExpandsRuleSetPath(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/subscribe?token=abc", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "sub.example.com")

	content := `url: /rulesets/ai_ai?target=clash-classical`
	got := replaceTemplateRuntimePlaceholders(req, content)

	want := `url: https://sub.example.com/rulesets/ai_ai?target=clash-classical`
	if got != want {
		t.Fatalf("期望为 %q，实际为 %q", want, got)
	}
}

func TestReplaceTemplateRuntimePlaceholdersUsesPublicBaseURLFirst(t *testing.T) {
	t.Setenv(publicBaseURLEnv, "https://public.example.com")

	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/subscribe?token=abc", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "sub.example.com")

	content := `url: /rulesets/ai_ai?target=sing-box`
	got := replaceTemplateRuntimePlaceholders(req, content)

	want := `url: https://public.example.com/rulesets/ai_ai?target=sing-box`
	if got != want {
		t.Fatalf("期望优先使用 PUBLIC_BASE_URL，实际为 %q", got)
	}
}

func TestReplaceTemplateRuntimePlaceholdersLeavesAbsoluteURLUntouched(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/subscribe?token=abc", nil)
	content := `url: https://ruleset.skk.moe/sing-box/non_ip/ai.json`
	got := replaceTemplateRuntimePlaceholders(req, content)
	if !strings.Contains(got, "https://ruleset.skk.moe/sing-box/non_ip/ai.json") {
		t.Fatalf("绝对 URL 不应被改写，实际为 %q", got)
	}
}
