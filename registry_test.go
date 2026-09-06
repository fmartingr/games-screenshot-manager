package gamesscreenshotmanager

import "testing"

// Two providers that want one program are grouped under it, so the doctor
// looks for it once.
func TestProviderRegistry_RequirementsGroupsOneProgram(t *testing.T) {
	config := &Config{}
	config.Defaults()
	config.Providers.GuildWars2.Enabled = Ptr(true)
	config.Providers.PlayStation4.Enabled = Ptr(true)

	registry, err := NewProviderRegistry(config, NewGameManager(), NewFileManager(*config))
	if err != nil {
		t.Fatalf("NewProviderRegistry() returned an error: %v", err)
	}

	requirements := registry.Requirements()

	uses, found := requirements["exiftool"]
	if !found {
		t.Fatalf("Requirements() holds no exiftool, got %v", requirements)
	}

	if len(uses) != 2 {
		t.Fatalf("exiftool holds %d uses, want 2", len(uses))
	}

	for _, use := range uses {
		if use.Requirement.Package != "exiftool" {
			t.Errorf("package = %q, want %q", use.Requirement.Package, "exiftool")
		}
	}
}

// A provider that needs no external program declares none.
func TestProviderRegistry_RequirementsIsEmptyForMinecraft(t *testing.T) {
	config := &Config{}
	config.Defaults()
	config.Providers.Minecraft.Enabled = Ptr(true)

	registry, err := NewProviderRegistry(config, NewGameManager(), NewFileManager(*config))
	if err != nil {
		t.Fatalf("NewProviderRegistry() returned an error: %v", err)
	}

	if requirements := registry.Requirements(); len(requirements) != 0 {
		t.Errorf("Requirements() = %v, want none", requirements)
	}
}
