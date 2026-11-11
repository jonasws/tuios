package input

import (
	tea "charm.land/bubbletea/v2"
	"github.com/Gaurav-Gosain/tuios/internal/app"
	"github.com/Gaurav-Gosain/tuios/internal/config"
	"github.com/Gaurav-Gosain/tuios/internal/terminal"
)

const (
	terminalScrollUp       = "terminal_scroll_up"
	terminalScrollDown     = "terminal_scroll_down"
	terminalScrollPageUp   = "terminal_scroll_page_up"
	terminalScrollPageDown = "terminal_scroll_page_down"
	terminalPasteHost      = "terminal_paste_host"
)

// handleTerminalScrollBinding runs one of terminal mode's line or page scroll
// actions. It reports false when action is not a scroll action, or when the
// pane owns the input because it is on the alternate screen or uses mouse
// tracking. The caller can then forward the original key or mouse press.
func handleTerminalScrollBinding(action string, o *app.OS, focused *terminal.Window) bool {
	if !terminalScrollAction(action) || focused == nil || !keyScrollsPane(focused) {
		return false
	}

	o.NoteAction(action)
	if o.MultiCopy.Has(focused.ID) {
		for _, w := range o.MultiCopyWindows() {
			scrollTerminalPane(w, action)
		}
		return true
	}
	scrollTerminalPane(focused, action)
	return true
}

func terminalScrollAction(action string) bool {
	switch action {
	case terminalScrollUp, terminalScrollDown, terminalScrollPageUp, terminalScrollPageDown:
		return true
	}
	return false
}

func scrollTerminalPane(win *terminal.Window, action string) {
	up := action == terminalScrollUp || action == terminalScrollPageUp
	lines := 1
	if action == terminalScrollPageUp || action == terminalScrollPageDown {
		lines = max(1, win.ContentHeight())
	}

	if up {
		if !win.InCopyMode() && win.ScrollbackLen() > 0 {
			win.EnterCopyModeImplicit()
		}
		scrollCopyModeUpBy(win, lines)
		return
	}
	if win.InCopyMode() {
		scrollCopyModeDownBy(win, lines)
		leaveCopyModeAtBottom(win)
	}
}

// boundMouseButton reports whether button is one a key binding can name:
// Bubble Tea's "middle", "backward" and "forward". "left" and "right" are
// deliberately not: they are already keyboard key names, so looking every
// button up in the keyboard table would make an arrow binding also capture a
// click.
func boundMouseButton(button tea.MouseButton) bool {
	switch button {
	case tea.MouseMiddle, tea.MouseBackward, tea.MouseForward:
		return true
	}
	return false
}

// handleGlobalMouseBinding runs a [keybindings.global] binding on a backward or
// forward press. It runs in every mode, wherever the pointer is, and over a
// pane whose program tracks the mouse: few programs use these buttons, and a
// navigation binding that worked only over some programs would be useless.
// Middle is not a global key, so it never gets here.
func handleGlobalMouseBinding(msg tea.MouseClickMsg, o *app.OS) (*app.OS, tea.Cmd, bool) {
	if msg.Button != tea.MouseBackward && msg.Button != tea.MouseForward {
		return o, nil, false
	}
	// A new press starts over. A release that never came must not make this
	// press's release disappear.
	o.ClearMouseBindingPress(msg.Button)
	if o.KeybindRegistry == nil {
		return o, nil, false
	}
	action := o.KeybindRegistry.Action(config.TableGlobal, msg.String())
	if action == "" || !GetDispatcher().HasAction(action) {
		return o, nil, false
	}
	o.MarkMouseBindingPress(msg.Button)
	return dispatchAction(action, tea.KeyPressMsg{}, o)
}

// handleTerminalMouseBinding gives the middle, backward and forward buttons the
// same terminal-mode binding lookup as a key.
//
// A bare press over a program that tracks the mouse (vim with mouse=a,
// lazygit, a nested tmux) goes to that program, as it did before the binding
// existed. A binding with a modifier, such as ctrl+middle, runs over such a
// program too. A global binding on the same key wins; see
// handleGlobalMouseBinding.
func handleTerminalMouseBinding(msg tea.MouseClickMsg, o *app.OS, clicked int) (*app.OS, tea.Cmd, bool) {
	if !boundMouseButton(msg.Button) {
		return o, nil, false
	}
	// A new press starts over. A release that never came must not make this
	// press's release disappear.
	o.ClearMouseBindingPress(msg.Button)
	if o.Mode != app.TerminalMode ||
		o.KeybindRegistry == nil || clicked < 0 || clicked >= len(o.Windows) {
		return o, nil, false
	}
	win := o.Windows[clicked]
	if _, _, inContent := win.ScreenToTerminal(msg.X, msg.Y); !inContent {
		return o, nil, false
	}
	if msg.Mod == 0 && win.Terminal != nil && win.Terminal.HasMouseMode() {
		return o, nil, false
	}
	action := o.KeybindRegistry.Action(config.TableTerminalMode, msg.String())
	if !terminalMouseActionCanRun(action, win) {
		return o, nil, false
	}

	o.MarkMouseBindingPress(msg.Button)
	o.FocusWindowFromClick(clicked, msg.X, msg.Y)
	if terminalScrollAction(action) {
		handleTerminalScrollBinding(action, o, win)
		return o, nil, true
	}
	if action == terminalPasteHost {
		o.NoteAction(action)
		return o, o.RequestPaste(), true
	}
	next, cmd, handled := dispatchAction(action, tea.KeyPressMsg{}, o)
	return next, cmd, handled
}

// terminalMouseBindingConsumesRelease reports whether a binding took the press
// this release ends, and clears the mark. It is decided at the press, not by
// where the release lands: a release outside the pane's content still belongs
// to the binding, and a release over the content after a press the binding did
// not take still goes to the program.
func terminalMouseBindingConsumesRelease(msg tea.MouseReleaseMsg, o *app.OS) bool {
	if !boundMouseButton(msg.Button) {
		return false
	}
	return o.ConsumeMouseBindingRelease(msg.Button)
}

func terminalMouseActionCanRun(action string, win *terminal.Window) bool {
	switch {
	case action == terminalPasteHost:
		return true
	case terminalScrollAction(action):
		return keyScrollsPane(win)
	default:
		return GetDispatcher().HasAction(action)
	}
}
