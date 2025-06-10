package stash

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/keys"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

const (
	stashIndent                = 1
	stashViewItemHeight        = 2 // Height of stash entry, including gap.
	stashViewTopPadding        = 2 // Padding at the top of the stash view.
	stashViewBottomPadding     = 6 // Pagination and gaps, but not help.
	stashViewHorizontalPadding = 6
)

var (
	dividerDot = styles.DarkGrayFg.SetString(" • ")
	dividerBar = styles.DarkGrayFg.SetString(" │ ")

	stashInputPromptStyle = lipgloss.NewStyle().
				Foreground(styles.YellowGreen).
				MarginRight(1)
	stashInputCursorStyle = lipgloss.NewStyle().
				Foreground(styles.Fuchsia).
				MarginRight(1)
)

type (
	FilteredYAMLMsg []*yamldoc.YAMLDocument
	FetchedYAMLMsg  *yamldoc.YAMLDocument
)

func LoadYAML(md *yamldoc.YAMLDocument) tea.Cmd {
	return func() tea.Msg {
		return FetchedYAMLMsg(md)
	}
}

// ViewState is the high-level state of the file listing.
type ViewState int

const (
	StateReady ViewState = iota
)

// The types of documents we are currently showing to the user.
type SectionKey int

const (
	SectionDocuments = iota
	SectionFilter
)

// Section contains definitions and state information for displaying a tab and
// its contents in the file listing view.
type Section struct {
	paginator paginator.Model
	key       SectionKey
	cursor    int
}

// FilterState is the current filtering state in the file listing.
type FilterState int

const (
	Unfiltered    FilterState = iota // No filter set.
	Filtering                        // User is actively setting a filter.
	FilterApplied                    // A filter is applied and user is not editing filter.
)

type StashModel struct {
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	docRenderer  *DocumentListRenderer

	// The master set of yaml documents we're working with.
	YAMLs []*yamldoc.YAMLDocument

	// YAML documents we're currently displaying. Filtering, toggles and so
	// on will alter this slice so we can show what is relevant. For that
	// reason, this field should be considered ephemeral.
	filteredYAMLs []*yamldoc.YAMLDocument

	// Available document sections we can cycle through. We use a slice, rather
	// than a map, because order is important.
	sections    []Section
	filterInput textinput.Model
	ViewState   ViewState

	// Index of the section we're currently looking at.
	sectionIndex int
	FilterState  FilterState

	// Page we're fetching stash items from on the server, which is different
	// from the local pagination. Generally, the server will return more items
	// than we can display at a time so we can paginate locally without having
	// to fetch every time.
	serverPage int64

	// Tracks if files were loaded.
	loaded     bool
	ShowHelp   bool
	helpHeight int
}

func NewStashModel(cm *common.CommonModel) StashModel {
	si := textinput.New()
	si.Prompt = "Find:"
	si.PromptStyle = stashInputPromptStyle
	si.Cursor.Style = stashInputCursorStyle
	si.Focus()

	s := []Section{
		{
			key:       SectionDocuments,
			paginator: newStashPaginator(),
		},
	}

	// Initialize help renderer with key bindings like pager does.
	kb := cm.Config.KeyBinds
	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(
		*kb.Common.Up,
		*kb.Common.Down,
		*kb.Common.Left,
		*kb.Common.Right,
		*kb.Stash.Home,
		*kb.Stash.End,
	)
	kbr.AddColumn(
		*kb.Common.Reload,
		*kb.Stash.Open,
		*kb.Stash.Find,
	)
	kbr.AddColumn(
		*kb.Common.Escape,
		*kb.Common.Error,
		*kb.Common.Help,
		*kb.Common.Quit,
	)

	m := StashModel{
		cm:           cm,
		filterInput:  si,
		serverPage:   1,
		sections:     s,
		helpRenderer: statusbar.NewHelpRenderer(kbr),
		docRenderer:  NewDocumentListRenderer(stashIndent, cm.Config.Compact),
	}

	return m
}

func (m StashModel) Update(msg tea.Msg) (StashModel, tea.Cmd) {
	var cmds []tea.Cmd

	isFiltering := m.FilterState == Filtering

	if isFiltering {
		var cmd tea.Cmd
		filterHandler := NewFilterHandler()
		m, cmd = filterHandler.HandleFilteringMode(m, msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isFiltering {
			// Don't re-handle filter keys.
			break
		}
		var cmd tea.Cmd
		stashHandler := NewStashKeyHandler()
		m, cmd = stashHandler.HandleDocumentBrowsing(m, msg)
		cmds = append(cmds, cmd)

	case FilteredYAMLMsg:
		m.filteredYAMLs = msg
		m.setCursor(0)

		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m StashModel) View() string {
	top := lipgloss.JoinVertical(
		lipgloss.Top,
		m.headerView(),
		m.documentListView(),
	)
	availableHeight := m.cm.Height - lipgloss.Height(top) + 1
	bottom := lipgloss.PlaceVertical(
		availableHeight,
		lipgloss.Bottom,
		lipgloss.JoinVertical(
			lipgloss.Top,
			lipgloss.PlaceHorizontal(
				m.cm.Width,
				lipgloss.Left,
				m.paginationView(),
			),
			m.statusBarView(),
			m.helpView(),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Top, top, bottom)
}

// Adds yaml documents to the model.
func (m *StashModel) AddYAMLs(mds ...*yamldoc.YAMLDocument) {
	if len(mds) == 0 {
		return
	}

	m.YAMLs = append(m.YAMLs, mds...)
	if !m.FilterApplied() {
		sortYAMLs(m.YAMLs)
	}

	m.updatePagination()
}

// Whether or not the spinner should be spinning.
func (m StashModel) IsLoading() bool {
	return !m.cm.Loaded
}

func (m StashModel) FilterApplied() bool {
	return m.FilterState != Unfiltered
}

func (m *StashModel) SetSize(width, height int) {
	m.cm.Width = width
	m.cm.Height = height

	// Calculate help height if needed.
	if m.ShowHelp && m.helpHeight == 0 {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
	}

	m.docRenderer.SetSize(width, height)

	m.filterInput.Width = width - stashViewHorizontalPadding*2 - ansi.PrintableRuneWidth(
		m.filterInput.Prompt,
	)

	m.updatePagination()
}

func (m StashModel) currentSection() *Section {
	return &m.sections[m.sectionIndex]
}

func (m StashModel) paginator() *paginator.Model {
	return &m.currentSection().paginator
}

func (m *StashModel) setPaginator(p paginator.Model) {
	m.currentSection().paginator = p
}

func (m StashModel) cursor() int {
	return m.currentSection().cursor
}

func (m *StashModel) setCursor(i int) {
	m.currentSection().cursor = i
}

func (m *StashModel) toggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.cm.Width, m.cm.Height)
}

func (m *StashModel) ResetFiltering() {
	m.FilterState = Unfiltered
	m.filterInput.Reset()
	m.filteredYAMLs = nil
	m.ViewState = StateReady

	sortYAMLs(m.YAMLs)

	// If the filtered section is present (it's always at the end) slice it out
	// of the sections slice to remove it from the UI.
	if m.sections[len(m.sections)-1].key == SectionFilter {
		m.sections = m.sections[:len(m.sections)-1]
	}

	// If the current section is out of bounds (it would be if we cut down the
	// slice above) then return to the first section.
	if m.sectionIndex > len(m.sections)-1 {
		m.sectionIndex = 0
	}

	// Update pagination after we've switched sections.
	m.updatePagination()
}

// Update pagination according to the amount of yamls for the current
// state.
func (m *StashModel) updatePagination() {
	helpHeight := 0
	if m.ShowHelp {
		helpHeight = m.helpHeight
	}

	// TODO: Why does this need to be set this way?
	availableHeight := m.cm.Height -
		helpHeight -
		stashViewBottomPadding

	m.paginator().PerPage = max(1, availableHeight/m.docRenderer.GetItemHeight())

	if pages := len(m.getVisibleYAMLs()); pages < 1 {
		m.paginator().SetTotalPages(1)
	} else {
		m.paginator().SetTotalPages(pages)
	}

	// Make sure the page stays in bounds.
	if m.paginator().Page >= m.paginator().TotalPages-1 {
		m.paginator().Page = max(0, m.paginator().TotalPages-1)
	}
}

// YAMLIndex returns the index of the currently selected yaml item.
func (m StashModel) yamlIndex() int {
	return m.paginator().Page*m.paginator().PerPage + m.cursor()
}

// Return the current selected yaml in the stash.
func (m StashModel) selectedYAML() *yamldoc.YAMLDocument {
	i := m.yamlIndex()

	mds := m.getVisibleYAMLs()
	if i < 0 || len(mds) == 0 || len(mds) <= i {
		return nil
	}

	return mds[i]
}

// Returns the yamls that should be currently shown.
func (m StashModel) getVisibleYAMLs() []*yamldoc.YAMLDocument {
	if m.FilterState == Filtering || m.currentSection().key == SectionFilter {
		return m.filteredYAMLs
	}

	return m.YAMLs
}

// Command for opening a yaml document in the pager. Note that this also
// alters the model.
func (m *StashModel) openYAML(md *yamldoc.YAMLDocument) tea.Cmd {
	cmd := LoadYAML(md)

	return cmd
}

func (m *StashModel) moveCursorUp() {
	m.setCursor(m.cursor() - 1)
	if m.cursor() < 0 && m.paginator().Page == 0 {
		// Stop.
		m.setCursor(0)

		return
	}

	if m.cursor() >= 0 {
		return
	}

	// Go to previous page.
	m.paginator().PrevPage()

	m.setCursor(m.paginator().ItemsOnPage(len(m.getVisibleYAMLs())) - 1)
}

func (m *StashModel) moveCursorDown() {
	itemsOnPage := m.paginator().ItemsOnPage(len(m.getVisibleYAMLs()))

	m.setCursor(m.cursor() + 1)
	if m.cursor() < itemsOnPage {
		return
	}

	if !m.paginator().OnLastPage() {
		m.paginator().NextPage()
		m.setCursor(0)

		return
	}

	// During filtering the cursor position can exceed the number of
	// itemsOnPage. It's more intuitive to start the cursor at the
	// topmost position when moving it down in this scenario.
	if m.cursor() > itemsOnPage {
		m.setCursor(0)

		return
	}
	m.setCursor(itemsOnPage - 1)
}

func (m *StashModel) enforcePaginationBounds() {
	itemsOnPage := m.paginator().ItemsOnPage(len(m.getVisibleYAMLs()))
	if m.cursor() > itemsOnPage-1 {
		m.setCursor(max(0, itemsOnPage-1))
	}
}

func (m StashModel) documentListView() string {
	return m.docRenderer.RenderDocumentList(m.getVisibleYAMLs(), m)
}

func (m StashModel) paginationView() string {
	pagination := "\n"
	if m.paginator().TotalPages > 1 {
		pagination = NewPaginationRenderer(m.cm.Width).
			RenderPagination(m.paginator(), m.paginator().TotalPages)
	}

	return pagination
}

func (m StashModel) helpView() string {
	var help string
	if m.ShowHelp {
		help = m.helpRenderer.Render(m.cm.Width)
	}

	return help
}

func (m StashModel) headerView() string {
	sections, divider := m.getHeaderSections()
	header := strings.Join(sections, divider.String())

	header = lipgloss.NewStyle().
		Padding(stashViewTopPadding, stashIndent+2, 1).
		Render(header)

	return header
}

func (m StashModel) getHeaderSections() ([]string, lipgloss.Style) {
	localCount := len(m.YAMLs)
	sections := []string{}

	// Filter results.
	if m.FilterState == Filtering {
		sections = append(sections, m.filterInput.View())

		for i := range sections {
			sections[i] = styles.GrayFg(sections[i])
		}

		return sections, dividerDot
	}

	// Tabs.
	for i := range len(m.sections) {
		var s string

		switch m.sections[i].key {
		case SectionDocuments:
			s = fmt.Sprintf("%d manifests", localCount)

		case SectionFilter:
			s = fmt.Sprintf("%d “%s”", len(m.filteredYAMLs), m.filterInput.Value())
		}

		if m.sectionIndex == i && len(m.sections) > 1 {
			s = styles.SelectedTabStyle.Render(s)
		} else {
			s = styles.TabStyle.Render(s)
		}
		sections = append(sections, s)
	}

	return sections, dividerBar
}

func (m StashModel) statusBarView() string {
	// Determine what to show as the title/message.
	title := m.cm.Cmd.String()

	// Show progress based on pagination.
	progress := fmt.Sprintf("%d/%d", m.paginator().Page+1, m.paginator().TotalPages)

	return m.cm.GetStatusBar().RenderWithNote(title, progress)
}

// startFiltering initializes the filtering mode.
func (m *StashModel) startFiltering() tea.Cmd {
	// Build values we'll filter against.
	for _, md := range m.YAMLs {
		md.BuildFilterValue()
	}

	m.filteredYAMLs = m.YAMLs
	m.paginator().Page = 0
	m.setCursor(0)
	m.FilterState = Filtering
	m.filterInput.CursorEnd()
	m.filterInput.Focus()

	return textinput.Blink
}
