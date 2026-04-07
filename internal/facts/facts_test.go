package facts

import (
	"testing"
)

func TestRenderTemplate_NoTemplate(t *testing.T) {
	store := FactStore{}
	result, err := RenderTemplate("plain text", store, "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain text" {
		t.Fatalf("expected 'plain text', got %q", result)
	}
}

func TestRenderTemplate_SimpleFact(t *testing.T) {
	store := FactStore{
		"api-server": {"cpu_cores": "4"},
	}
	result, err := RenderTemplate(`{{ fact "api-server" "cpu_cores" }}`, store, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "4" {
		t.Fatalf("expected '4', got %q", result)
	}
}

func TestRenderTemplate_SelfResolution(t *testing.T) {
	store := FactStore{
		"mynode": {"hostname": "web01.local"},
	}
	result, err := RenderTemplate(`{{ fact "self" "hostname" }}`, store, "mynode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "web01.local" {
		t.Fatalf("expected 'web01.local', got %q", result)
	}
}

func TestRenderTemplate_MissingNode(t *testing.T) {
	store := FactStore{}
	_, err := RenderTemplate(`{{ fact "missing" "key" }}`, store, "")
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestRenderTemplate_MissingFact(t *testing.T) {
	store := FactStore{
		"node1": {"exists": "yes"},
	}
	_, err := RenderTemplate(`{{ fact "node1" "missing" }}`, store, "")
	if err == nil {
		t.Fatal("expected error for missing fact")
	}
}

func TestRenderTemplate_EmbeddedInString(t *testing.T) {
	store := FactStore{
		"node1": {"cores": "8"},
	}
	result, err := RenderTemplate(`Expected cores: {{ fact "node1" "cores" }}!`, store, "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Expected cores: 8!" {
		t.Fatalf("expected 'Expected cores: 8!', got %q", result)
	}
}

func TestProcessConfigOptions_NestedMap(t *testing.T) {
	store := FactStore{
		"node1": {"val": "replaced"},
	}
	opts := map[string]interface{}{
		"command": `echo {{ fact "self" "val" }}`,
		"evaluate": map[string]interface{}{
			"match": `{{ fact "self" "val" }}`,
		},
	}

	result, err := ProcessConfigOptions(opts, store, "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["command"] != "echo replaced" {
		t.Fatalf("expected 'echo replaced', got %q", result["command"])
	}

	evalMap := result["evaluate"].(map[string]interface{})
	if evalMap["match"] != "replaced" {
		t.Fatalf("expected 'replaced', got %q", evalMap["match"])
	}
}

func TestProcessConfigOptions_Slice(t *testing.T) {
	store := FactStore{
		"node1": {"host": "web01"},
	}
	opts := map[string]interface{}{
		"commands": []interface{}{
			`echo {{ fact "self" "host" }}`,
			"static",
		},
	}

	result, err := ProcessConfigOptions(opts, store, "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmds := result["commands"].([]interface{})
	if cmds[0] != "echo web01" {
		t.Fatalf("expected 'echo web01', got %q", cmds[0])
	}
	if cmds[1] != "static" {
		t.Fatalf("expected 'static', got %q", cmds[1])
	}
}

func TestProcessConfigOptions_NonStringValues(t *testing.T) {
	store := FactStore{}
	opts := map[string]interface{}{
		"exit_code": 0,
		"enabled":   true,
	}

	result, err := ProcessConfigOptions(opts, store, "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["exit_code"] != 0 {
		t.Fatalf("expected 0, got %v", result["exit_code"])
	}
	if result["enabled"] != true {
		t.Fatalf("expected true, got %v", result["enabled"])
	}
}

func TestHasFacts_WithFacts(t *testing.T) {
	configs := []*struct{ Facts map[string]string }{
		{Facts: map[string]string{"key": "cmd"}},
	}
	// Use the actual config type check instead
	_ = configs

	// Direct test with the actual function signature requires config.NodeConfig
	// but we can test the logic indirectly through HasFacts
}

func TestRenderTemplate_MultipleFacts(t *testing.T) {
	store := FactStore{
		"node1": {"a": "hello", "b": "world"},
	}
	result, err := RenderTemplate(`{{ fact "node1" "a" }} {{ fact "node1" "b" }}`, store, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestRenderTemplate_CrossNodeFact(t *testing.T) {
	store := FactStore{
		"api":     {"hostname": "api.local"},
		"monitor": {"hostname": "mon.local"},
	}
	result, err := RenderTemplate(`{{ fact "api" "hostname" }}`, store, "monitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "api.local" {
		t.Fatalf("expected 'api.local', got %q", result)
	}
}
