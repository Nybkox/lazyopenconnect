package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit       key.Binding
	FocusPane1 key.Binding
	FocusPane2 key.Binding
	FocusPane3 key.Binding
	FocusPane4 key.Binding
	FocusPane5 key.Binding
	TabFocus   key.Binding

	Up         key.Binding
	Down       key.Binding
	Connect    key.Binding
	Disconnect key.Binding
	Cleanup    key.Binding
	Edit       key.Binding
	Delete     key.Binding
	New        key.Binding
	Settings   key.Binding

	ScrollUp       key.Binding
	ScrollDown     key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	ScrollToTop    key.Binding
	ScrollToBottom key.Binding

	Submit key.Binding

	Reset key.Binding

	Cancel key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:       key.NewBinding(key.WithKeys("ctrl+c")),
		FocusPane1: key.NewBinding(key.WithKeys("1")),
		FocusPane2: key.NewBinding(key.WithKeys("2")),
		FocusPane3: key.NewBinding(key.WithKeys("3")),
		FocusPane4: key.NewBinding(key.WithKeys("4")),
		FocusPane5: key.NewBinding(key.WithKeys("5")),
		TabFocus:   key.NewBinding(key.WithKeys("tab")),

		Up:         key.NewBinding(key.WithKeys("k", "up")),
		Down:       key.NewBinding(key.WithKeys("j", "down")),
		Connect:    key.NewBinding(key.WithKeys("enter")),
		Disconnect: key.NewBinding(key.WithKeys("d")),
		Cleanup:    key.NewBinding(key.WithKeys("c")),
		Edit:       key.NewBinding(key.WithKeys("e")),
		Delete:     key.NewBinding(key.WithKeys("x")),
		New:        key.NewBinding(key.WithKeys("n")),
		Settings:   key.NewBinding(key.WithKeys("s")),

		ScrollUp:       key.NewBinding(key.WithKeys("k", "up")),
		ScrollDown:     key.NewBinding(key.WithKeys("j", "down")),
		PageUp:         key.NewBinding(key.WithKeys("ctrl+u")),
		PageDown:       key.NewBinding(key.WithKeys("ctrl+d")),
		ScrollToTop:    key.NewBinding(key.WithKeys("g")),
		ScrollToBottom: key.NewBinding(key.WithKeys("G")),

		Submit: key.NewBinding(key.WithKeys("enter")),

		Reset: key.NewBinding(key.WithKeys("r")),

		Cancel: key.NewBinding(key.WithKeys("esc")),
	}
}
