package api

import (
	"net/http/httptest"
	"testing"
)

func TestResolveConfigTarget(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback string
		want     string
		wantErr  bool
	}{
		{name: "默认 clash meta", want: "clash-mihomo"},
		{name: "显式 clash", raw: "clash", want: "clash-mihomo"},
		{name: "显式 clash-mihomo", raw: "clash-mihomo", want: "clash-mihomo"},
		{name: "显式 stash", raw: "stash", want: "stash"},
		{name: "显式 surge", raw: "surge", want: "surge"},
		{name: "显式 sing_box", raw: "sing_box", want: "sing-box"},
		{name: "使用模板 fallback", fallback: "sing_box", want: "sing-box"},
		{name: "默认 stash fallback", fallback: "stash", want: "stash"},
		{name: "非法 target", raw: "loon", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveConfigTarget(tt.raw, tt.fallback)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误，实际成功: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveConfigTarget() 返回错误: %v", err)
			}
			if got != tt.want {
				t.Fatalf("期望 target=%q，实际为 %q", tt.want, got)
			}
		})
	}
}

func TestValidateConvertTemplateParams(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		wantErr   bool
	}{
		{name: "template 使用雪花id", targetURL: "/convert?template=154025419991939"},
		{name: "template 非数字", targetURL: "/convert?template=my-template", wantErr: true},
		{name: "不支持 template_id", targetURL: "/convert?template_id=8", wantErr: true},
		{name: "不支持 template_name", targetURL: "/convert?template_name=tpl-a", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.targetURL, nil)
			err := validateConvertTemplateParams(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误，实际成功")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateConvertTemplateParams() 返回错误: %v", err)
			}
		})
	}
}

func TestParseConvertRequestParams(t *testing.T) {
	tests := []struct {
		name           string
		targetURL      string
		wantSubURL     string
		wantTemplateID int64
		wantTarget     string
		wantErr        bool
	}{
		{
			name:           "支持未编码订阅地址",
			targetURL:      "/convert?url=https://example.com/sub?target=clash&filename=demo&emoji=true&url=https%3A%2F%2Forigin.example.com%2Fsub%3Ftoken%3Dabc&template=154025419991939",
			wantSubURL:     "https://example.com/sub?target=clash&filename=demo&emoji=true&url=https%3A%2F%2Forigin.example.com%2Fsub%3Ftoken%3Dabc",
			wantTemplateID: 154025419991939,
		},
		{
			name:           "支持编码订阅地址和 target",
			targetURL:      "/convert?target=surge&url=https%3A%2F%2Fexample.com%2Fsub%3Fa%3D1%26b%3D2&template=9",
			wantSubURL:     "https://example.com/sub?a=1&b=2",
			wantTemplateID: 9,
			wantTarget:     "surge",
		},
		{
			name:      "非法 template",
			targetURL: "/convert?url=https://example.com/sub&template=abc",
			wantErr:   true,
		},
		{
			name:      "不支持 template_id",
			targetURL: "/convert?url=https://example.com/sub&template_id=9",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.targetURL, nil)
			got, err := parseConvertRequestParams(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误，实际成功")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConvertRequestParams() 返回错误: %v", err)
			}
			if got.subURL != tt.wantSubURL || got.templateID != tt.wantTemplateID || got.target != tt.wantTarget {
				t.Fatalf("期望 subURL=%q templateID=%d target=%q，实际为 subURL=%q templateID=%d target=%q",
					tt.wantSubURL, tt.wantTemplateID, tt.wantTarget,
					got.subURL, got.templateID, got.target)
			}
		})
	}
}

func TestBuildConvertLogToken(t *testing.T) {
	t.Run("默认", func(t *testing.T) {
		if got := buildConvertLogToken(convertRequestParams{}); got != "convert" {
			t.Fatalf("期望 token=%q，实际为 %q", "convert", got)
		}
	})

	t.Run("优先订阅地址", func(t *testing.T) {
		want := "https://example.com/sub?token=abc"
		if got := buildConvertLogToken(convertRequestParams{subURL: want, templateID: 7}); got != want {
			t.Fatalf("期望 token=%q，实际为 %q", want, got)
		}
	})

	t.Run("按模板ID", func(t *testing.T) {
		want := "convert:template_id=7"
		if got := buildConvertLogToken(convertRequestParams{templateID: 7}); got != want {
			t.Fatalf("期望 token=%q，实际为 %q", want, got)
		}
	})

	t.Run("超长订阅地址截断", func(t *testing.T) {
		got := buildConvertLogToken(convertRequestParams{
			subURL: "https://example.com/sub?token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		})
		if len(got) != 64 {
			t.Fatalf("期望截断后长度为 64，实际为 %d: %q", len(got), got)
		}
		if got[len(got)-3:] != "..." {
			t.Fatalf("期望截断后以省略号结尾，实际为 %q", got)
		}
	})
}
