package api

import "testing"

func TestTemplateValidationNodes(t *testing.T) {
	nodes := templateValidationNodes()
	if len(nodes) != 1 {
		t.Fatalf("期望 1 个示例节点，实际为 %d", len(nodes))
	}
	if nodes[0].Protocol == "" || nodes[0].Server == "" || nodes[0].Port == 0 {
		t.Fatalf("示例节点字段不完整: %#v", nodes[0])
	}
}

func TestResolveConfigTargetForTemplateValidation(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{name: "clash", target: "clash", want: "clash-mihomo"},
		{name: "clash-mihomo", target: "clash-mihomo", want: "clash-mihomo"},
		{name: "stash", target: "stash", want: "stash"},
		{name: "surge", target: "surge", want: "surge"},
		{name: "sing_box", target: "sing_box", want: "sing-box"},
		{name: "loon", target: "loon", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveConfigTarget(tt.target, "")
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
