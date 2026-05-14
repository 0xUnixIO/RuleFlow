package api

import (
	"testing"

	"github.com/ablate-ai/RuleFlow/database"
)

func TestBuildManualNodeFromRequestKeepsExistingNameOnReimport(t *testing.T) {
	h := &Handlers{}
	existing := &database.Node{
		ID:       1,
		Name:     "旧名称",
		Protocol: "ss",
		Server:   "old.example.com",
		Port:     1234,
		Config: map[string]interface{}{
			"cipher":   "aes-256-gcm",
			"password": "old",
		},
		Enabled: true,
		Tags:    []string{"keep"},
	}

	node, err := h.buildManualNodeFromRequest(manualNodeRequest{
		Name:     existing.Name,
		Protocol: "share",
		ShareURL: "ss://YWVzLTI1Ni1nY206OGI4Z3pnWkpVdXQ3NEtMV0k4ckNtSkpYS2hiNkplN1dqaHgxM0Eyc0tQOD0@72.234.229.126:38280?type=tcp#telegram%40wenwencc-f3lpezxl",
		Tags:     []string{"keep"},
		Enabled:  true,
	}, existing)
	if err != nil {
		t.Fatalf("buildManualNodeFromRequest() error = %v", err)
	}

	if node.Name != existing.Name {
		t.Fatalf("重新导入时名称被覆盖: got %q want %q", node.Name, existing.Name)
	}
	if node.Server != "72.234.229.126" {
		t.Fatalf("Server = %q, want %q", node.Server, "72.234.229.126")
	}
	if node.Port != 38280 {
		t.Fatalf("Port = %d, want %d", node.Port, 38280)
	}
}
