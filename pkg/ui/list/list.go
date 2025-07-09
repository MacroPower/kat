package list

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const (
	listIndent                = 1
	listViewTopPadding        = 1 // Padding at the top of the list view.
	listViewBottomPadding     = 6 // Pagination and gaps, but not help.
	listViewHorizontalPadding = 6
)

type (
	FilteredYAMLMsg []*yamls.Document
	FetchedYAMLMsg  *yamls.Document
)

func LoadYAML(md *yamls.Document) tea.Cmd {
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

type ListModel struct {
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	docRenderer  *DocumentListRenderer
	keyHandler   *KeyHandler

	// The master set of yaml documents we're working with.
	YAMLs []*yamls.Document

	// Available document sections we can cycle through. We use a slice, rather
	// than a map, because order is important.
	sections []Section

	// YAML documents we're currently displaying. Filtering, toggles and so
	// on will alter this slice so we can show what is relevant. For that
	// reason, this field should be considered ephemeral.
	filteredYAMLs []*yamls.Document

	filterInput textinput.Model
	ViewState   ViewState

	// Index of the section we're currently looking at.
	sectionIndex int

	FilterState FilterState
	helpHeight  int
	ShowHelp    bool
	compact     bool
}

type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
	Compact     bool
}

func NewModel(c Config) ListModel {
	si := textinput.New()
	si.Prompt = "Find:"
	si.PromptStyle = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	si.Cursor.Style = c.CommonModel.Theme.CursorStyle.MarginRight(1)
	si.Focus()

	s := []Section{
		{
			key:       SectionDocuments,
			paginator: newListPaginator(c.CommonModel.Theme),
		},
	}

	// Initialize help renderer with key bindings like pager does.
	ckb := c.CommonModel.KeyBinds
	kb := c.KeyBinds
	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(
		*ckb.Up,
		*ckb.Down,
		*ckb.Left,
		*ckb.Right,
		*kb.PageUp,
		*kb.PageDown,
	)
	kbr.AddColumn(
		*ckb.Reload,
		*kb.Open,
		*kb.Find,
		*kb.Home,
		*kb.End,
	)
	kbr.AddColumn(
		*ckb.Escape,
		*ckb.Error,
		*ckb.Help,
		*ckb.Quit,
	)

	m := ListModel{
		cm:           c.CommonModel,
		filterInput:  si,
		sections:     s,
		helpRenderer: statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
		docRenderer:  NewDocumentListRenderer(c.CommonModel.Theme, listIndent, c.Compact),
		keyHandler:   NewKeyHandler(kb, ckb, c.CommonModel.Theme),
		compact:      c.Compact,
	}

	return m
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	var cmds []tea.Cmd

	isFiltering := m.FilterState == Filtering

	if isFiltering {
		var cmd tea.Cmd

		m, cmd = m.keyHandler.HandleFilteringMode(m, msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isFiltering {
			// Don't re-handle filter keys.
			break
		}

		var cmd tea.Cmd

		m, cmd = m.keyHandler.HandleDocumentBrowsing(m, msg)
		cmds = append(cmds, cmd)

	case FilteredYAMLMsg:
		m.filteredYAMLs = msg
		m.setCursor(0)

		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m ListModel) View() string {
	top := lipgloss.JoinVertical(
		lipgloss.Top,
		m.headerView(),
		m.documentListView(),
	)
	availableHeight := m.cm.Height - lipgloss.Height(top)
	if !m.ShowHelp {
		availableHeight++
	}

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
func (m *ListModel) AddYAMLs(yaml ...*yamls.Document) {
	if len(yaml) == 0 {
		return
	}

	m.YAMLs = append(m.YAMLs, yaml...)
	if !m.FilterApplied() {
		sortYAMLs(m.YAMLs)
	}

	m.updatePagination()
}

// Whether or not the spinner should be spinning.
func (m ListModel) IsLoading() bool {
	return !m.cm.Loaded
}

func (m ListModel) FilterApplied() bool {
	return m.FilterState != Unfiltered
}

func (m *ListModel) SetSize(width, height int) {
	m.cm.Width = width
	m.cm.Height = height

	// Calculate help height if needed.
	if m.ShowHelp && m.helpHeight == 0 {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
	}

	m.filterInput.Width = width - listViewHorizontalPadding*2 - ansi.PrintableRuneWidth(
		m.filterInput.Prompt,
	)

	m.updatePagination()
}

func (m ListModel) currentSection() *Section {
	return &m.sections[m.sectionIndex]
}

func (m ListModel) paginator() *paginator.Model {
	return &m.currentSection().paginator
}

func (m *ListModel) setPaginator(p paginator.Model) {
	m.currentSection().paginator = p
}

func (m ListModel) cursor() int {
	return m.currentSection().cursor
}

func (m *ListModel) setCursor(i int) {
	m.currentSection().cursor = i
}

func (m *ListModel) toggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.cm.Width, m.cm.Height)
}

func (m *ListModel) ResetFiltering() {
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
func (m *ListModel) updatePagination() {
	helpHeight := 0
	if m.ShowHelp {
		helpHeight = m.helpHeight + 1
	}

	// TODO: Why does this need to be set this way?
	availableHeight := m.cm.Height -
		helpHeight -
		listViewTopPadding -
		listViewBottomPadding

	if !m.compact {
		availableHeight++
	}

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
func (m ListModel) yamlIndex() int {
	return m.paginator().Page*m.paginator().PerPage + m.cursor()
}

// Return the current selected yaml in the list.
func (m ListModel) selectedYAML() *yamls.Document {
	i := m.yamlIndex()

	mds := m.getVisibleYAMLs()
	if i < 0 || len(mds) == 0 || len(mds) <= i {
		return nil
	}

	return mds[i]
}

// Returns the yamls that should be currently shown.
func (m ListModel) getVisibleYAMLs() []*yamls.Document {
	if m.FilterState == Filtering || m.currentSection().key == SectionFilter {
		return m.filteredYAMLs
	}

	return m.YAMLs
}

// Command for opening a yaml document in the pager. Note that this also
// alters the model.
func (m *ListModel) openYAML(md *yamls.Document) tea.Cmd {
	cmd := LoadYAML(md)

	return cmd
}

func (m *ListModel) itemsOnPage() int {
	return m.paginator().ItemsOnPage(len(m.getVisibleYAMLs()))
}

func (m *ListModel) moveCursorUp() {
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

	m.setCursor(m.itemsOnPage() - 1)
}

func (m *ListModel) moveCursorDown() {
	itemsOnPage := m.itemsOnPage()

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

func (m *ListModel) enforcePaginationBounds() {
	itemsOnPage := m.itemsOnPage()
	if m.cursor() > itemsOnPage-1 {
		m.setCursor(max(0, itemsOnPage-1))
	}
}

func (m ListModel) documentListView() string {
	return m.docRenderer.RenderDocumentList(m.getVisibleYAMLs(), m)
}

func (m ListModel) paginationView() string {
	pagination := "\n"
	if m.paginator().TotalPages > 1 {
		pagination = NewPaginationRenderer(m.cm.Theme, m.cm.Width).
			RenderPagination(m.paginator(), m.paginator().TotalPages)
	}

	return pagination
}

func (m ListModel) helpView() string {
	var help string
	if m.ShowHelp {
		help = m.helpRenderer.Render(m.cm.Width)
	}

	return help
}

func (m ListModel) headerView() string {
	sections, divider := m.getHeaderSections()
	header := strings.Join(sections, divider.String())

	header = lipgloss.NewStyle().
		Padding(listViewTopPadding, listIndent+2, 1).
		Render(header)

	return header
}

func (m ListModel) getHeaderSections() ([]string, lipgloss.Style) {
	localCount := len(m.YAMLs)
	sections := []string{}

	var (
		dividerDot = m.cm.Theme.SubtleStyle.SetString(" • ")
		dividerBar = m.cm.Theme.SubtleStyle.SetString(" │ ")
	)

	// Filter results.
	if m.FilterState == Filtering {
		sections = append(sections, m.filterInput.View())

		for i := range sections {
			sections[i] = m.cm.Theme.GenericTextStyle.Render(sections[i])
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
			s = m.cm.Theme.SelectedStyle.Render(s)
		} else {
			s = m.cm.Theme.SubtleStyle.Render(s)
		}

		sections = append(sections, s)
	}

	return sections, dividerBar
}

func (m ListModel) statusBarView() string {
	// Determine what to show as the title/message.
	title := m.cm.Cmd.String()

	// Show progress based on pagination.
	progress := fmt.Sprintf("%d/%d", m.paginator().Page+1, m.paginator().TotalPages)

	return m.cm.GetStatusBar().RenderWithNote(title, progress)
}

// startFiltering initializes the filtering mode.
func (m *ListModel) startFiltering() tea.Cmd {
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
