package pager

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
)

type KeyBinds struct {
	Copy *keys.KeyBind `json:"copy,omitempty"`

	// Navigation.
	Home         *keys.KeyBind `json:"home,omitempty"`
	End          *keys.KeyBind `json:"end,omitempty"`
	PageUp       *keys.KeyBind `json:"pageUp,omitempty"`
	PageDown     *keys.KeyBind `json:"pageDown,omitempty"`
	HalfPageUp   *keys.KeyBind `json:"halfPageUp,omitempty"`
	HalfPageDown *keys.KeyBind `json:"halfPageDown,omitempty"`

	// Search.
	Search    *keys.KeyBind `json:"search,omitempty"`
	NextMatch *keys.KeyBind `json:"nextMatch,omitempty"`
	PrevMatch *keys.KeyBind `json:"prevMatch,omitempty"`
}

func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Copy,
		keys.NewBind("copy contents",
			keys.New("c"),
		))
	keys.SetDefaultBind(&kb.Home,
		keys.NewBind("go to top",
			keys.New("home"),
			keys.New("g"),
		))
	keys.SetDefaultBind(&kb.End,
		keys.NewBind("go to bottom",
			keys.New("end"),
			keys.New("G"),
		))
	keys.SetDefaultBind(&kb.PageUp,
		keys.NewBind("page up",
			keys.New("pgup"),
			keys.New("b"),
		))
	keys.SetDefaultBind(&kb.PageDown,
		keys.NewBind("page down",
			keys.New("pgdown", keys.WithAlias("pgdn")),
			keys.New("f"),
		))
	keys.SetDefaultBind(&kb.HalfPageUp,
		keys.NewBind("½ page up",
			keys.New("u"),
		))
	keys.SetDefaultBind(&kb.HalfPageDown,
		keys.NewBind("½ page down",
			keys.New("d"),
		))
	keys.SetDefaultBind(&kb.Search,
		keys.NewBind("search content",
			keys.New("/"),
		))
	keys.SetDefaultBind(&kb.NextMatch,
		keys.NewBind("next match",
			keys.New("n"),
		))
	keys.SetDefaultBind(&kb.PrevMatch,
		keys.NewBind("previous match",
			keys.New("N"),
		))
}

func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Copy,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
		*kb.HalfPageUp,
		*kb.HalfPageDown,
		*kb.Search,
		*kb.NextMatch,
		*kb.PrevMatch,
	}
}

// KeyHandler provides key handling for pager view.
type KeyHandler struct {
	kb  *KeyBinds
	ckb *common.KeyBinds
}

// NewKeyHandler creates a new pager key handler.
func NewKeyHandler(kb *KeyBinds, ckb *common.KeyBinds) *KeyHandler {
	return &KeyHandler{
		kb:  kb,
		ckb: ckb,
	}
}

// HandlePagerKeys handles key events for pager view.
func (h *KeyHandler) HandlePagerKeys(m PagerModel, msg tea.KeyMsg) (PagerModel, tea.Cmd) {
	var cmd tea.Cmd

	key := msg.String()

	switch {
	case h.kb.Home.Match(key):
		m.GoToTop()

	case h.kb.End.Match(key):
		m.GoToBottom()

	case h.kb.PageUp.Match(key):
		m.PageUp()

	case h.kb.PageDown.Match(key):
		m.PageDown()

	case h.kb.HalfPageDown.Match(key):
		m.HalfPageDown()

	case h.kb.HalfPageUp.Match(key):
		m.HalfPageUp()

	case h.ckb.Up.Match(key):
		m.MoveUp()

	case h.ckb.Down.Match(key):
		m.MoveDown()

	case h.ckb.Help.Match(key):
		m.ToggleHelp()

	case h.kb.Search.Match(key):
		cmd = m.StartSearch()

	case h.kb.NextMatch.Match(key):
		cmd = m.NextMatch()

	case h.kb.PrevMatch.Match(key):
		cmd = m.PrevMatch()

	case h.kb.Copy.Match(key):
		cmd = m.CopyContent()
	}

	return m, cmd
}
