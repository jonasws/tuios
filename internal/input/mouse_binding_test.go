package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Gaurav-Gosain/tuios/internal/app"
	"github.com/Gaurav-Gosain/tuios/internal/config"
)

func mouseBindingConfig(t *testing.T, toml string) *config.UserConfig {
	t.Helper()
	cfg, err := config.ParseUserConfig([]byte(toml))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// buttonPress sends a press then a release of button over the pane's content
// and reports whether the program in the pane was sent anything.
func buttonPress(t *testing.T, o *app.OS, pty *capturePty, button tea.MouseButton, mod tea.KeyMod) (sent int) {
	t.Helper()
	x, y := contentCell(o.Windows[0])
	pty.got = nil
	HandleInput(tea.MouseClickMsg{Button: button, X: x, Y: y, Mod: mod}, o)
	HandleInput(tea.MouseReleaseMsg{Button: button, X: x, Y: y, Mod: mod}, o)
	return len(pty.got)
}

func trackMouse(o *app.OS, pty *capturePty) {
	win := o.Windows[0]
	win.DaemonMode = true
	win.DaemonWriteFunc = func(b []byte) error { _, err := pty.Write(b); return err }
	win.Terminal.Write([]byte("\x1b[?1002h\x1b[?1006h"))
}

// A global binding runs in either mode, over a pane that tracks the mouse, and
// its release does not reach the program.
func TestGlobalMouseBindingRunsInEveryMode(t *testing.T) {
	cfg := mouseBindingConfig(t, "[keybindings.global]\ntoggle_tiling = [\"backward\"]\n")
	for _, mode := range []app.Mode{app.TerminalMode, app.WindowManagementMode} {
		o, pty := osWithFocusedPane(t, cfg, mode)
		trackMouse(o, pty)
		if !o.Windows[0].Terminal.HasMouseMode() {
			t.Fatal("the fixture pane did not take mouse tracking")
		}
		before := o.AutoTiling
		if sent := buttonPress(t, o, pty, tea.MouseBackward, 0); sent != 0 {
			t.Errorf("mode %v: %q reached the program", mode, pty.got)
		}
		if o.AutoTiling == before {
			t.Errorf("mode %v: the global binding did not run", mode)
		}
	}
}

func TestGlobalMouseBindingWithModifier(t *testing.T) {
	cfg := mouseBindingConfig(t, "[keybindings.global]\ntoggle_tiling = [\"ctrl+forward\"]\n")
	o, pty := osWithFocusedPane(t, cfg, app.TerminalMode)
	trackMouse(o, pty)
	before := o.AutoTiling
	buttonPress(t, o, pty, tea.MouseForward, 0)
	if o.AutoTiling != before {
		t.Error("a bare forward press ran the ctrl+forward binding")
	}
	buttonPress(t, o, pty, tea.MouseForward, tea.ModCtrl)
	if o.AutoTiling == before {
		t.Error("ctrl+forward did not run its binding")
	}
}

func TestCommandKeyOnForwardRuns(t *testing.T) {
	cfg := mouseBindingConfig(t, "[[keybindings.command]]\nkey = \"forward\"\ntype = \"shell\"\ncommand = \"true\"\nname = \"fwd\"\n")
	o, _ := osWithFocusedPane(t, cfg, app.TerminalMode)
	x, y := contentCell(o.Windows[0])
	_, cmd := HandleInput(tea.MouseClickMsg{Button: tea.MouseForward, X: x, Y: y}, o)
	if cmd == nil {
		t.Error("the command key on forward did not run its command")
	}
}

func TestUnboundBackwardStillReachesTheProgram(t *testing.T) {
	o, pty := osWithFocusedPane(t, config.DefaultConfig(), app.TerminalMode)
	trackMouse(o, pty)
	if sent := buttonPress(t, o, pty, tea.MouseBackward, 0); sent == 0 {
		t.Error("an unbound backward press was not forwarded to the mouse-tracking pane")
	}
}

// A bare terminal_mode binding yields to a program that tracks the mouse, and
// one with a modifier runs over it.
func TestTerminalModeMouseBindingYieldsToTheProgram(t *testing.T) {
	cfg := mouseBindingConfig(t, "[keybindings.terminal_mode]\ntoggle_tiling = [\"forward\", \"ctrl+forward\"]\n")
	o, pty := osWithFocusedPane(t, cfg, app.TerminalMode)
	trackMouse(o, pty)

	before := o.AutoTiling
	if sent := buttonPress(t, o, pty, tea.MouseForward, 0); sent == 0 {
		t.Error("a bare forward press was not forwarded to the mouse-tracking pane")
	}
	if o.AutoTiling != before {
		t.Error("a bare forward press ran its binding over a mouse-tracking pane")
	}

	if sent := buttonPress(t, o, pty, tea.MouseForward, tea.ModCtrl); sent != 0 {
		t.Errorf("ctrl+forward reached the program: %q", pty.got)
	}
	if o.AutoTiling == before {
		t.Error("ctrl+forward did not run its binding")
	}
}

// A binding's hold on a press is kept per button.
func TestMouseBindingReleaseMarkIsPerButton(t *testing.T) {
	cfg := mouseBindingConfig(t, "[keybindings.global]\ntoggle_tiling = [\"backward\"]\n")
	o, _ := osWithFocusedPane(t, cfg, app.TerminalMode)
	x, y := contentCell(o.Windows[0])

	HandleInput(tea.MouseClickMsg{Button: tea.MouseBackward, X: x, Y: y}, o)
	// Another button's release does not consume backward's mark.
	if o.ConsumeMouseBindingRelease(tea.MouseForward) {
		t.Error("forward consumed the mark of backward")
	}
	if !o.ConsumeMouseBindingRelease(tea.MouseBackward) {
		t.Error("backward's mark was lost")
	}
}
