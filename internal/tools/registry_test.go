package tools

import (
	"os"
	"testing"
	"time"
)

func TestRegistry_EnableAndGet(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	registry.Enable(tool)

	retrieved := registry.Get("Shell")
	if retrieved == nil {
		t.Fatal("Expected to retrieve enabled tool")
	}

	if retrieved.Name() != "Shell" {
		t.Errorf("Expected tool name 'Shell', got '%s'", retrieved.Name())
	}
}

func TestRegistry_GetNonExistent(t *testing.T) {
	registry := NewRegistry()

	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for non-existent tool")
	}
}

func TestRegistry_Specs(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool)

	specs := registry.Specs()
	if len(specs) != 1 {
		t.Errorf("Expected 1 spec, got %d", len(specs))
	}

	if specs[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", specs[0].Type)
	}

	if specs[0].Function.Name != "Shell" {
		t.Errorf("Expected function name 'Shell', got '%s'", specs[0].Function.Name)
	}
}

func TestRegistry_All(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool1 := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool1)

	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(all))
	}
}
