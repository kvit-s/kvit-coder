package prompt

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/tools"
)

func TestCapabilitiesString(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []string
		want         string
	}{
		{
			name:         "empty capabilities",
			capabilities: []string{},
			want:         "various tools",
		},
		{
			name:         "single capability",
			capabilities: []string{"reading files"},
			want:         "reading files",
		},
		{
			name:         "two capabilities",
			capabilities: []string{"reading files", "editing files"},
			want:         "reading files, editing files",
		},
		{
			name:         "multiple capabilities",
			capabilities: []string{"reading files", "editing files", "searching code", "running shell commands"},
			want:         "reading files, editing files, searching code, running shell commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &PromptContext{
				Capabilities: tt.capabilities,
			}
			got := ctx.CapabilitiesString()
			if got != tt.want {
				t.Errorf("CapabilitiesString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		sep  string
		want string
	}{
		{
			name: "empty",
			strs: []string{},
			sep:  ", ",
			want: "",
		},
		{
			name: "single",
			strs: []string{"one"},
			sep:  ", ",
			want: "one",
		},
		{
			name: "multiple",
			strs: []string{"one", "two", "three"},
			sep:  ", ",
			want: "one, two, three",
		},
		{
			name: "different separator",
			strs: []string{"a", "b", "c"},
			sep:  " | ",
			want: "a | b | c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.want {
				t.Errorf("joinStrings() = %q, want %q", got, tt.want)
			}
		})
	}
}

// mockRegistry implements RegistryInterface for testing
type mockRegistry struct {
	enabled    map[string]bool
	categories []string
	tools      map[string][]tools.Tool
}

func (m *mockRegistry) IsEnabled(name string) bool {
	return m.enabled[name]
}

func (m *mockRegistry) GenerateToolPrompt() string {
	return "mock tool prompt"
}

func (m *mockRegistry) ToolsInCategory(category string) []tools.Tool {
	return m.tools[category]
}

func (m *mockRegistry) EnabledCategories() []string {
	return m.categories
}

func TestBuildCapabilitiesList(t *testing.T) {
	tests := []struct {
		name    string
		enabled map[string]bool
		want    []string
	}{
		{
			name:    "no tools enabled",
			enabled: map[string]bool{},
			want:    nil,
		},
		{
			name: "read only",
			enabled: map[string]bool{
				"Read": true,
			},
			want: []string{"reading files"},
		},
		{
			name: "all core tools",
			enabled: map[string]bool{
				"Read":   true,
				"Edit":   true,
				"Search": true,
				"Shell":  true,
			},
			want: []string{"reading files", "editing files", "searching code", "running shell commands"},
		},
		{
			name: "with plan and checkpoint",
			enabled: map[string]bool{
				"Read":            true,
				"Plan.create":     true,
				"Checkpoint.list": true,
			},
			want: []string{"reading files", "making and tracking plans", "managing checkpoints"},
		},
		{
			name: "with tasks",
			enabled: map[string]bool{
				"Tasks.Start": true,
			},
			want: []string{"managing tasks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockRegistry{enabled: tt.enabled}
			got := buildCapabilitiesList(registry)

			if len(got) != len(tt.want) {
				t.Errorf("buildCapabilitiesList() returned %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("buildCapabilitiesList()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestDetermineEnabledCategories(t *testing.T) {
	tests := []struct {
		name    string
		enabled map[string]bool
		want    []string
	}{
		{
			name:    "no tools",
			enabled: map[string]bool{},
			want:    nil,
		},
		{
			name: "filesystem only",
			enabled: map[string]bool{
				"Read": true,
			},
			want: []string{"filesystem"},
		},
		{
			name: "shell only",
			enabled: map[string]bool{
				"Shell": true,
			},
			want: []string{"shell"},
		},
		{
			name: "multiple categories",
			enabled: map[string]bool{
				"Read":            true,
				"Shell":           true,
				"Plan.create":     true,
				"Checkpoint.list": true,
			},
			want: []string{"filesystem", "shell", "plan", "checkpoint"},
		},
		{
			name: "advanced shell counts",
			enabled: map[string]bool{
				"Shell.advanced": true,
			},
			want: []string{"shell"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockRegistry{enabled: tt.enabled}
			got := determineEnabledCategories(registry)

			if len(got) != len(tt.want) {
				t.Errorf("determineEnabledCategories() returned %d items, want %d: got %v", len(got), len(tt.want), got)
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("determineEnabledCategories()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
