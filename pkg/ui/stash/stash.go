package stash

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/keys"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

const (
	stashIndent                = 1
	stashViewItemHeight        = 3 // Height of stash entry, including gap.
	stashViewTopPadding        = 5 // Logo, status bar, gaps.
	stashViewBottomPadding     = 5 // Pagination and gaps, but not help.
	stashViewHorizontalPadding = 6
)

var (
	stashingStatusMessage = StatusMessage{"Stashing...", NormalStatusMessage}

	dividerDot = styles.DarkGrayFg.SetString(" • ")
	dividerBar = styles.DarkGrayFg.SetString(" │ ")

	stashSpinnerStyle = lipgloss.NewStyle().
				Foreground(styles.Gray)
	stashInputPromptStyle = lipgloss.NewStyle().
				Foreground(styles.YellowGreen).
				MarginRight(1)
	stashInputCursorStyle = lipgloss.NewStyle().
				Foreground(styles.Fuchsia).
				MarginRight(1)
)

// MSG.

type (
	FilteredYAMLMsg []*yamldoc.YAMLDocument
	FetchedYAMLMsg  *yamldoc.YAMLDocument
)

// MODEL.

// StashViewState is the high-level state of the file listing.
type StashViewState int

const (
	StashStateReady StashViewState = iota
	StashStateLoadingDocument
	StashStateShowingError
)

// The types of documents we are currently showing to the user.
type sectionKey int

const (
	documentsSection = iota
	filterSection
)

// Section contains definitions and state information for displaying a tab and
// its contents in the file listing view.
type Section struct {
	paginator paginator.Model
	key       sectionKey
	cursor    int
}

// map sections to their associated types.
var sections = map[sectionKey]Section{
	documentsSection: {
		key:       documentsSection,
		paginator: newStashPaginator(),
	},
	filterSection: {
		key:       filterSection,
		paginator: newStashPaginator(),
	},
}

// FilterState is the current filtering state in the file listing.
type FilterState int

const (
	Unfiltered    FilterState = iota // No filter set.
	Filtering                        // User is actively setting a filter.
	FilterApplied                    // A filter is applied and user is not editing filter.
)

// StatusMessageType adds some context to the status message being sent.
type StatusMessageType int

// Types of status messages.
const (
	NormalStatusMessage StatusMessageType = iota
	SubtleStatusMessage
	ErrorStatusMessage
)

// StatusMessage is an ephemeral note displayed in the UI.
type StatusMessage struct {
	message string
	status  StatusMessageType
}

// String returns a styled version of the status message appropriate for the
// given context.
func (s StatusMessage) String() string {
	switch s.status {
	case SubtleStatusMessage:
		return styles.DimGreenFg(s.message)
	case ErrorStatusMessage:
		return styles.RedFg(s.message)
	default:
		return styles.GreenFg(s.message)
	}
}

type StashModel struct {
	err                error
	statusMessageTimer *time.Timer
	common             *common.CommonModel
	statusMessage      StatusMessage
	helpRenderer       *statusbar.HelpRenderer
	statusBarRenderer  *statusbar.StatusBarRenderer

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
	Spinner     spinner.Model
	ViewState   StashViewState

	// Index of the section we're currently looking at.
	sectionIndex int
	FilterState  FilterState

	// Page we're fetching stash items from on the server, which is different
	// from the local pagination. Generally, the server will return more items
	// than we can display at a time so we can paginate locally without having
	// to fetch every time.
	serverPage int64

	showStatusMessage bool

	// Tracks if files were loaded.
	loaded     bool
	ShowHelp   bool
	helpHeight int
}

func (m StashModel) loadingDone() bool {
	return m.loaded
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

// Whether or not the spinner should be spinning.
func (m StashModel) ShouldSpin() bool {
	loading := !m.loadingDone()
	openingDocument := m.ViewState == StashStateLoadingDocument

	return loading || openingDocument
}

func (m *StashModel) SetSize(width, height int) {
	m.common.Width = width
	m.common.Height = height

	// Update status bar renderer width
	m.statusBarRenderer = statusbar.NewStatusBarRenderer(width)

	// Calculate help height if needed
	if m.ShowHelp && m.helpHeight == 0 {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
	}

	m.filterInput.Width = width - stashViewHorizontalPadding*2 - ansi.PrintableRuneWidth(
		m.filterInput.Prompt,
	)

	m.updatePagination()
}

func (m *StashModel) toggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.common.Width, m.common.Height)
}

func (m *StashModel) resetFiltering() {
	m.FilterState = Unfiltered
	m.filterInput.Reset()
	m.filteredYAMLs = nil

	sortYAMLs(m.YAMLs)

	// If the filtered section is present (it's always at the end) slice it out
	// of the sections slice to remove it from the UI.
	if m.sections[len(m.sections)-1].key == filterSection {
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

// Is a filter currently being applied?
func (m StashModel) FilterApplied() bool {
	return m.FilterState != Unfiltered
}

// Should we be updating the filter?
func (m StashModel) ShouldUpdateFilter() bool {
	// If we're in the middle of setting a note don't update the filter so that
	// the focus won't jump around.
	return m.FilterApplied()
}

// Update pagination according to the amount of yamls for the current
// state.
func (m *StashModel) updatePagination() {
	helpHeight := 0
	if m.ShowHelp {
		helpHeight = m.helpHeight
	}

	availableHeight := m.common.Height -
		stashViewTopPadding -
		helpHeight -
		stashViewBottomPadding

	m.paginator().PerPage = max(1, availableHeight/stashViewItemHeight)

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

// Returns the yamls that should be currently shown.
func (m StashModel) getVisibleYAMLs() []*yamldoc.YAMLDocument {
	if m.FilterState == Filtering || m.currentSection().key == filterSection {
		return m.filteredYAMLs
	}

	return m.YAMLs
}

// Command for opening a yaml document in the pager. Note that this also
// alters the model.
func (m *StashModel) openYAML(md *yamldoc.YAMLDocument) tea.Cmd {
	m.ViewState = StashStateLoadingDocument
	cmd := LoadYAML(md)

	return tea.Batch(cmd, m.Spinner.Tick)
}

func (m *StashModel) hideStatusMessage() {
	m.showStatusMessage = false
	m.statusMessage = StatusMessage{}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
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

// INIT.

func NewStashModel(cm *common.CommonModel) StashModel {
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = stashSpinnerStyle

	si := textinput.New()
	si.Prompt = "Find:"
	si.PromptStyle = stashInputPromptStyle
	si.Cursor.Style = stashInputCursorStyle
	si.Focus()

	s := []Section{
		sections[documentsSection],
	}

	// Initialize help renderer with key bindings like pager does
	kb := cm.Config.KeyBinds
	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(
		*kb.Common.Up,
		*kb.Common.Down,
		*kb.Stash.Find,
		*kb.Common.Escape,
	)
	kbr.AddColumn(
		*kb.Stash.Home,
		*kb.Stash.End,
		*kb.Stash.Open,
		*kb.Common.Reload,
		*kb.Common.Quit,
	)

	m := StashModel{
		common:            cm,
		Spinner:           sp,
		filterInput:       si,
		serverPage:        1,
		sections:          s,
		helpRenderer:      statusbar.NewHelpRenderer(kbr),
		statusBarRenderer: statusbar.NewStatusBarRenderer(cm.Width),
	}

	return m
}

func newStashPaginator() paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = styles.BrightGrayFg("•")
	p.InactiveDot = styles.DarkGrayFg.Render("•")

	return p
}

// UPDATE.

func (m StashModel) Update(msg tea.Msg) (StashModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case common.ErrMsg:
		m.err = msg

	case common.CommandRunFinished:
		// We're finished searching for local files.
		m.loaded = true

	case FilteredYAMLMsg:
		m.filteredYAMLs = msg
		m.setCursor(0)

		return m, nil

	case spinner.TickMsg:
		if m.ShouldSpin() {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case common.StatusMessageTimeoutMsg:
		if common.ApplicationContext(msg) == common.StashContext {
			m.hideStatusMessage()
		}
	}

	if m.FilterState == Filtering {
		cmds = append(cmds, m.handleFiltering(msg))

		return m, tea.Batch(cmds...)
	}

	// Updates per the current state.
	switch m.ViewState {
	case StashStateReady:
		cmds = append(cmds, m.handleDocumentBrowsing(msg))
	case StashStateShowingError:
		// Any key exists the error view.
		if _, ok := msg.(tea.KeyMsg); ok {
			m.ViewState = StashStateReady
		}
	}

	return m, tea.Batch(cmds...)
}

// Updates for when a user is browsing the yaml listing.
func (m *StashModel) handleDocumentBrowsing(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Use the new stash key handler for document browsing.
		stashHandler := NewStashKeyHandler()
		browseCmds := stashHandler.HandleDocumentBrowsing(m, msg)
		cmds = append(cmds, browseCmds...)
	}

	// Handle pagination using the new pagination handler.
	paginationHandler := NewPaginationKeyHandler()
	paginationCmds := paginationHandler.HandlePaginationKeys(m, msg)
	cmds = append(cmds, paginationCmds...)

	return tea.Batch(cmds...)
}

// Updates for when a user is in the filter editing interface.
func (m *StashModel) handleFiltering(msg tea.Msg) tea.Cmd {
	// Use the new FilterHandler for cleaner filtering logic.
	filterHandler := NewFilterHandler()
	cmds := filterHandler.HandleFilteringMode(m, msg)

	return tea.Batch(cmds...)
}

// VIEW.

func (m StashModel) View() string {
	var s string
	switch m.ViewState {
	case StashStateShowingError:
		return common.ErrorView(m.err, false)
	case StashStateLoadingDocument:
		s += " " + m.Spinner.View() + " Loading document..."
	case StashStateReady:
		// Use ViewBuilder for composable view construction.
		vb := NewViewBuilder()

		// Build the main sections using the rendering utilities.
		loadingIndicator := " "
		if m.ShouldSpin() {
			loadingIndicator = m.Spinner.View()
		}

		// Create header renderer for logo/filter section.
		headerRenderer := NewHeaderRenderer(m.common.Width, m.common.Height)
		logoOrFilter := headerRenderer.RenderLogoOrFilter(
			m.FilterState,
			m.filterInput.View(),
			func() string {
				if m.showStatusMessage {
					return m.statusMessage.String()
				}

				return ""
			}(),
		)

		// Get regular header content.
		header := m.headerView()

		// Add status bar using statusbar renderer
		statusBar := m.statusBarView()

		// Get help content using statusbar renderer like pager does
		var help string
		var helpHeight int
		if m.ShowHelp {
			help = m.helpRenderer.Render(m.common.Width)
			helpHeight = m.helpRenderer.CalculateHelpHeight() + 1
		}

		// Use DocumentListRenderer for the populated view.
		listRenderer := NewDocumentListRenderer(m.common.Width, m.common.Height)
		populatedView := listRenderer.RenderDocumentList(m.getVisibleYAMLs(), m)
		populatedViewHeight := strings.Count(populatedView, "\n")

		// Calculate layout using LayoutCalculator.
		calc := NewLayoutCalculator(m.common.Width, m.common.Height)
		availHeight := calc.CalculateAvailableHeight(stashViewTopPadding, stashViewBottomPadding, helpHeight, populatedViewHeight)
		blankLines := fillVerticalSpace(availHeight)

		// Use PaginationRenderer for pagination controls.
		var pagination string
		if m.paginator().TotalPages > 1 {
			pagRenderer := NewPaginationRenderer(m.common.Width)
			pagination = pagRenderer.RenderPagination(m.paginator(), m.paginator().TotalPages)
		}

		// Build the final view using ViewBuilder.
		s = vb.
			AddSection(loadingIndicator + logoOrFilter).
			AddEmptySection().
			AddSection(padHorizontal(header, 2, 0)).
			AddEmptySection().
			AddSection(common.Indent(populatedView, stashIndent)).
			AddSection(blankLines).
			AddSection(padHorizontal(pagination, 2, 0)).
			AddEmptySection().
			AddSection(statusBar).
			AddSection(help).
			Build()
	}

	return "\n" + s
}

func (m StashModel) headerView() string {
	localCount := len(m.YAMLs)
	sections := []string{}

	// Filter results.
	if m.FilterState == Filtering {
		if localCount == 0 {
			return styles.GrayFg("Nothing found.")
		}
		if localCount > 0 {
			sections = append(sections, fmt.Sprintf("%d local", localCount))
		}

		for i := range sections {
			sections[i] = styles.GrayFg(sections[i])
		}

		return strings.Join(sections, dividerDot.String())
	}

	// Tabs.
	for i := range len(m.sections) {
		var s string

		switch m.sections[i].key {
		case documentsSection:
			s = fmt.Sprintf("%d documents", localCount)

		case filterSection:
			s = fmt.Sprintf("%d “%s”", len(m.filteredYAMLs), m.filterInput.Value())
		}

		if m.sectionIndex == i && len(m.sections) > 1 {
			s = styles.SelectedTabStyle.Render(s)
		} else {
			s = styles.TabStyle.Render(s)
		}
		sections = append(sections, s)
	}

	return strings.Join(sections, dividerBar.String())
}

func (m StashModel) statusBarView() string {
	// Determine what to show as the title/message
	var title string
	var statusMsg string

	if m.showStatusMessage {
		statusMsg = m.statusMessage.String()
		// When showing status message, use current section as title
		switch m.currentSection().key {
		case documentsSection:
			title = fmt.Sprintf("%d documents", len(m.YAMLs))
		case filterSection:
			title = fmt.Sprintf("%d \"%s\"", len(m.filteredYAMLs), m.filterInput.Value())
		}
	} else {
		// When no status message, show current section info as title
		switch m.currentSection().key {
		case documentsSection:
			title = fmt.Sprintf("%d documents", len(m.YAMLs))
		case filterSection:
			title = fmt.Sprintf("%d \"%s\"", len(m.filteredYAMLs), m.filterInput.Value())
		}
	}

	// Calculate progress percentage based on pagination
	var progressPercent float64
	if m.paginator().TotalPages > 1 {
		progressPercent = float64(m.paginator().Page) / float64(m.paginator().TotalPages-1)
	}

	return m.statusBarRenderer.RenderWithScroll(title, statusMsg, progressPercent)
}

// COMMANDS.

// handleNavigationKeys handles navigation keys for document browsing.
func (m *StashModel) handleNavigationKeys(key string) {
	numDocs := len(m.getVisibleYAMLs())
	kb := m.common.Config.KeyBinds

	switch {
	case kb.Common.Up.Match(key):
		m.moveCursorUp()
	case kb.Common.Down.Match(key):
		m.moveCursorDown()
	case kb.Stash.Home.Match(key):
		m.paginator().Page = 0
		m.setCursor(0)
	case kb.Stash.End.Match(key):
		m.paginator().Page = m.paginator().TotalPages - 1
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)
	}
}

// handleDocumentKeys handles document-specific keys.
func (m *StashModel) handleDocumentKeys(key string) tea.Cmd {
	numDocs := len(m.getVisibleYAMLs())
	kb := m.common.Config.KeyBinds

	if kb.Stash.Open.Match(key) {
		m.hideStatusMessage()
		if numDocs == 0 {
			return nil
		}
		md := m.selectedYAML()

		return m.openYAML(md)
	}

	return nil
}

// handleFilterKeys handles filter-specific keys.
func (m *StashModel) handleFilterKeys(key string) tea.Cmd {
	kb := m.common.Config.KeyBinds

	if kb.Stash.Find.Match(key) {
		m.hideStatusMessage()

		return m.startFiltering()
	}

	return nil
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

func LoadYAML(md *yamldoc.YAMLDocument) tea.Cmd {
	return func() tea.Msg {
		return FetchedYAMLMsg(md)
	}
}

func FilterYAMLs(m StashModel) tea.Cmd {
	return func() tea.Msg {
		if m.filterInput.Value() == "" || !m.FilterApplied() {
			return FilteredYAMLMsg(m.YAMLs) // Return everything.
		}

		targets := []string{}
		mds := m.YAMLs

		for _, t := range mds {
			targets = append(targets, t.FilterValue)
		}

		ranks := fuzzy.Find(m.filterInput.Value(), targets)
		sort.Stable(ranks)

		filtered := []*yamldoc.YAMLDocument{}
		for _, r := range ranks {
			filtered = append(filtered, mds[r.Index])
		}

		return FilteredYAMLMsg(filtered)
	}
}
