package config

import (
	"strings"
	"testing"
)

func TestValidateSectionKeyMouseButtons(t *testing.T) {
	kn := NewKeyNormalizer()
	for _, button := range []string{"backward", "forward"} {
		for _, key := range []string{button, "ctrl+" + button, "alt+" + button, "shift+ctrl+" + button, "ctrl+alt+shift+" + button} {
			for _, section := range []string{SectionGlobal, SectionTerminalMode} {
				if ok, msg := kn.ValidateSectionKey(section, key); !ok {
					t.Errorf("%q in %s rejected: %s", key, section, msg)
				}
			}
			for _, section := range []string{SectionWindowManagement, SectionPrefixMode, SectionWorkspaces, SectionLayout} {
				ok, msg := kn.ValidateSectionKey(section, key)
				if ok {
					t.Errorf("%q in %s accepted", key, section)
				}
				if !strings.Contains(msg, "[keybindings.global]") || !strings.Contains(msg, "[keybindings.terminal_mode]") {
					t.Errorf("%q in %s: message %q does not name the allowed tables", key, section, msg)
				}
			}
		}
		for _, key := range []string{"super+" + button, "hyper+" + button, "ctrl+ctrl+" + button, "ctrl+shift+ctrl+" + button} {
			if ok, _ := kn.ValidateSectionKey(SectionGlobal, key); ok {
				t.Errorf("%q accepted", key)
			}
		}
	}
}

func TestValidateSectionKeyMiddleUnchanged(t *testing.T) {
	kn := NewKeyNormalizer()
	for _, key := range []string{"middle", "ctrl+middle", "alt+shift+middle"} {
		if ok, msg := kn.ValidateSectionKey(SectionTerminalMode, key); !ok {
			t.Errorf("%q in terminal_mode rejected: %s", key, msg)
		}
		if ok, _ := kn.ValidateSectionKey(SectionGlobal, key); ok {
			t.Errorf("%q in global accepted", key)
		}
	}
	if ok, _ := kn.ValidateSectionKey(SectionTerminalMode, "super+middle"); ok {
		t.Error("super+middle accepted")
	}
}

func TestCommandKeyOnMouseButton(t *testing.T) {
	cfg, err := ParseUserConfig([]byte(`
[[keybindings.command]]
key = "backward"
type = "shell"
command = "true"
name = "back"

[[keybindings.command]]
key = "ctrl+forward"
type = "shell"
command = "true"
name = "fwd"

[[keybindings.command]]
key = "prefix+backward"
type = "shell"
command = "true"
name = "prefixed"
`))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range cfg.Keybindings.Commands() {
		names = append(names, c.ResolvedName())
	}
	if got := strings.Join(names, ","); got != "back,fwd" {
		t.Errorf("valid commands = %q, want back,fwd", got)
	}
	r := NewKeybindRegistry(cfg)
	if got := r.Action(TableGlobal, "backward"); got != "command:back" {
		t.Errorf("global backward runs %q, want command:back", got)
	}
	if got := r.Action(TableGlobal, "ctrl+forward"); got != "command:fwd" {
		t.Errorf("global ctrl+forward runs %q, want command:fwd", got)
	}
}

func TestMouseButtonBindingsAreReportedWithoutProblems(t *testing.T) {
	r := registryFor(t, func(cfg *UserConfig) {
		cfg.Keybindings.Global["next_workspace"] = []string{"forward"}
		cfg.Keybindings.TerminalMode["prev_workspace"] = []string{"ctrl+backward"}
	})
	if probs := r.KeyProblems(); len(probs) != 0 {
		t.Errorf("KeyProblems = %+v, want none", probs)
	}
	got := map[string]string{}
	for _, s := range r.TerminalModeSwallowed() {
		got[s.Key] = s.Origin
	}
	if got["forward"] != "global" || got["ctrl+backward"] != "terminal_mode" {
		t.Errorf("swallowed origins = %v", got)
	}

	bad := registryFor(t, func(cfg *UserConfig) {
		cfg.Keybindings.WindowManagement["new_window"] = []string{"forward"}
	})
	if probs := bad.KeyProblems(); len(probs) != 1 {
		t.Errorf("forward in window_management: KeyProblems = %+v, want one", probs)
	}
}
