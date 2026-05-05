package metric

import "testing"

func TestNewRegistryRegistersRuntimeCollectors(t *testing.T) {
	registry := NewRegistry()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}

	for _, name := range []string{"go_goroutines", "process_cpu_seconds_total", "go_build_info"} {
		if !names[name] {
			t.Fatalf("metric family %q not found", name)
		}
	}
}
