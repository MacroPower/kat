package ui

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
	"github.com/muesli/reflow/truncate"
	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/version"
)

const (
	stashIndent                = 1
	stashViewItemHeight        = 3 // Height of stash entry, including gap.
	stashViewTopPadding        = 5 // Logo, status bar, gaps.
	stashViewBottomPadding     = 3 // Pagination and gaps, but not help.
	stashViewHorizontalPadding = 6
)

var (
	stashingStatusMessage = statusMessage{"Stashing...", normalStatusMessage}

	dividerDot = darkGrayFg.SetString(" • ")
	dividerBar = darkGrayFg.SetString(" │ ")

	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ECFD65")).
			Background(fuchsia).
			Bold(true)

	stashSpinnerStyle = lipgloss.NewStyle().
				Foreground(gray)
	stashInputPromptStyle = lipgloss.NewStyle().
				Foreground(yellowGreen).
				MarginRight(1)
	stashInputCursorStyle = lipgloss.NewStyle().
				Foreground(fuchsia).
				MarginRight(1)
)

// MSG.

type (
	filteredYAMLMsg []*yaml
	fetchedYAMLMsg  *yaml
)

// MODEL.

// stashViewState is the high-level state of the file listing.
type stashViewState int

const (
	stashStateReady stashViewState = iota
	stashStateLoadingDocument
	stashStateShowingError
)

// The types of documents we are currently showing to the user.
type sectionKey int

const (
	documentsSection = iota
	filterSection
)

// section contains definitions and state information for displaying a tab and
// its contents in the file listing view.
type section struct {
	paginator paginator.Model
	key       sectionKey
	cursor    int
}

// map sections to their associated types.
var sections = map[sectionKey]section{}

// filterState is the current filtering state in the file listing.
type filterState int

const (
	unfiltered    filterState = iota // No filter set.
	filtering                        // User is actively setting a filter.
	filterApplied                    // A filter is applied and user is not editing filter.
)

// statusMessageType adds some context to the status message being sent.
type statusMessageType int

// Types of status messages.
const (
	normalStatusMessage statusMessageType = iota
	subtleStatusMessage
	errorStatusMessage
)

// statusMessage is an ephemeral note displayed in the UI.
type statusMessage struct {
	message string
	status  statusMessageType
}

func initSections() {
	sections = map[sectionKey]section{
		documentsSection: {
			key:       documentsSection,
			paginator: newStashPaginator(),
		},
		filterSection: {
			key:       filterSection,
			paginator: newStashPaginator(),
		},
	}
}

// String returns a styled version of the status message appropriate for the
// given context.
func (s statusMessage) String() string {
	switch s.status {
	case subtleStatusMessage:
		return dimGreenFg(s.message)
	case errorStatusMessage:
		return redFg(s.message)
	default:
		return greenFg(s.message)
	}
}

type stashModel struct {
	err                error
	statusMessageTimer *time.Timer
	common             *commonModel
	statusMessage      statusMessage

	// The master set of yaml documents we're working with.
	yamls []*yaml

	// YAML documents we're currently displaying. Filtering, toggles and so
	// on will alter this slice so we can show what is relevant. For that
	// reason, this field should be considered ephemeral.
	filteredYAMLs []*yaml

	// Available document sections we can cycle through. We use a slice, rather
	// than a map, because order is important.
	sections    []section
	filterInput textinput.Model
	spinner     spinner.Model
	viewState   stashViewState

	// Index of the section we're currently looking at.
	sectionIndex int
	filterState  filterState

	// Page we're fetching stash items from on the server, which is different
	// from the local pagination. Generally, the server will return more items
	// than we can display at a time so we can paginate locally without having
	// to fetch every time.
	serverPage int64

	showStatusMessage bool

	// Tracks if files were loaded.
	loaded       bool
	showFullHelp bool
}

func (m stashModel) loadingDone() bool {
	return m.loaded
}

func (m stashModel) currentSection() *section {
	return &m.sections[m.sectionIndex]
}

func (m stashModel) paginator() *paginator.Model {
	return &m.currentSection().paginator
}

func (m *stashModel) setPaginator(p paginator.Model) {
	m.currentSection().paginator = p
}

func (m stashModel) cursor() int {
	return m.currentSection().cursor
}

func (m *stashModel) setCursor(i int) {
	m.currentSection().cursor = i
}

// Whether or not the spinner should be spinning.
func (m stashModel) shouldSpin() bool {
	loading := !m.loadingDone()
	openingDocument := m.viewState == stashStateLoadingDocument

	return loading || openingDocument
}

func (m *stashModel) setSize(width, height int) {
	m.common.width = width
	m.common.height = height

	m.filterInput.Width = width - stashViewHorizontalPadding*2 - ansi.PrintableRuneWidth(
		m.filterInput.Prompt,
	)

	m.updatePagination()
}

func (m *stashModel) resetFiltering() {
	m.filterState = unfiltered
	m.filterInput.Reset()
	m.filteredYAMLs = nil

	sortYAMLs(m.yamls)

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
func (m stashModel) filterApplied() bool {
	return m.filterState != unfiltered
}

// Should we be updating the filter?
func (m stashModel) shouldUpdateFilter() bool {
	// If we're in the middle of setting a note don't update the filter so that
	// the focus won't jump around.
	return m.filterApplied()
}

// Update pagination according to the amount of yamls for the current
// state.
func (m *stashModel) updatePagination() {
	_, helpHeight := m.helpView()

	availableHeight := m.common.height -
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
func (m stashModel) yamlIndex() int {
	return m.paginator().Page*m.paginator().PerPage + m.cursor()
}

// Return the current selected yaml in the stash.
func (m stashModel) selectedYAML() *yaml {
	i := m.yamlIndex()

	mds := m.getVisibleYAMLs()
	if i < 0 || len(mds) == 0 || len(mds) <= i {
		return nil
	}

	return mds[i]
}

// Adds yaml documents to the model.
func (m *stashModel) addYAMLs(mds ...*yaml) {
	if len(mds) == 0 {
		return
	}

	m.yamls = append(m.yamls, mds...)
	if !m.filterApplied() {
		sortYAMLs(m.yamls)
	}

	m.updatePagination()
}

// Returns the yamls that should be currently shown.
func (m stashModel) getVisibleYAMLs() []*yaml {
	if m.filterState == filtering || m.currentSection().key == filterSection {
		return m.filteredYAMLs
	}

	return m.yamls
}

// Command for opening a yaml document in the pager. Note that this also
// alters the model.
func (m *stashModel) openYAML(md *yaml) tea.Cmd {
	m.viewState = stashStateLoadingDocument
	cmd := loadLocalYAML(md)

	return tea.Batch(cmd, m.spinner.Tick)
}

func (m *stashModel) hideStatusMessage() {
	m.showStatusMessage = false
	m.statusMessage = statusMessage{}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
}

func (m *stashModel) moveCursorUp() {
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

func (m *stashModel) moveCursorDown() {
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

func newStashModel(common *commonModel) stashModel {
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = stashSpinnerStyle

	si := textinput.New()
	si.Prompt = "Find:"
	si.PromptStyle = stashInputPromptStyle
	si.Cursor.Style = stashInputCursorStyle
	si.Focus()

	s := []section{
		sections[documentsSection],
	}

	m := stashModel{
		common:      common,
		spinner:     sp,
		filterInput: si,
		serverPage:  1,
		sections:    s,
	}

	return m
}

func newStashPaginator() paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = brightGrayFg("•")
	p.InactiveDot = darkGrayFg.Render("•")

	return p
}

// UPDATE.

func (m stashModel) update(msg tea.Msg) (stashModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case errMsg:
		m.err = msg

	case commandRunFinished:
		// We're finished searching for local files.
		m.loaded = true

	case filteredYAMLMsg:
		m.filteredYAMLs = msg
		m.setCursor(0)

		return m, nil

	case spinner.TickMsg:
		if m.shouldSpin() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case statusMessageTimeoutMsg:
		if applicationContext(msg) == stashContext {
			m.hideStatusMessage()
		}
	}

	if m.filterState == filtering {
		cmds = append(cmds, m.handleFiltering(msg))

		return m, tea.Batch(cmds...)
	}

	// Updates per the current state.
	switch m.viewState {
	case stashStateReady:
		cmds = append(cmds, m.handleDocumentBrowsing(msg))
	case stashStateShowingError:
		// Any key exists the error view.
		if _, ok := msg.(tea.KeyMsg); ok {
			m.viewState = stashStateReady
		}
	}

	return m, tea.Batch(cmds...)
}

// Updates for when a user is browsing the yaml listing.
func (m *stashModel) handleDocumentBrowsing(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	numDocs := len(m.getVisibleYAMLs())

	switch msg := msg.(type) {
	// Handle keys.
	case tea.KeyMsg:
		switch msg.String() {
		case "k", "ctrl+k", "up":
			m.moveCursorUp()

		case "j", "ctrl+j", "down":
			m.moveCursorDown()

		// Go to the very start.
		case "home", "g":
			m.paginator().Page = 0
			m.setCursor(0)

		// Go to the very end.
		case "end", "G":
			m.paginator().Page = m.paginator().TotalPages - 1
			m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

		// Clear filter (if applicable).
		case keyEsc:
			if m.filterApplied() {
				m.resetFiltering()
			}

		// Next section.
		case "tab", "L":
			if len(m.sections) == 0 || m.filterState == filtering {
				break
			}
			m.sectionIndex++
			if m.sectionIndex >= len(m.sections) {
				m.sectionIndex = 0
			}
			m.updatePagination()

		// Previous section.
		case "shift+tab", "H":
			if len(m.sections) == 0 || m.filterState == filtering {
				break
			}
			m.sectionIndex--
			if m.sectionIndex < 0 {
				m.sectionIndex = len(m.sections) - 1
			}
			m.updatePagination()

		case "F":
			m.loaded = false

			return runCommand(*m.common)

		// Open document.
		case keyEnter:
			m.hideStatusMessage()

			if numDocs == 0 {
				break
			}

			// Load the document from the server. We'll handle the message
			// that comes back in the main update function.
			md := m.selectedYAML()
			cmds = append(cmds, m.openYAML(md))

		// Filter your notes.
		case "/":
			m.hideStatusMessage()

			// Build values we'll filter against.
			for _, md := range m.yamls {
				md.buildFilterValue()
			}

			m.filteredYAMLs = m.yamls

			m.paginator().Page = 0
			m.setCursor(0)
			m.filterState = filtering
			m.filterInput.CursorEnd()
			m.filterInput.Focus()

			return textinput.Blink

		// Toggle full help.
		case "?":
			m.showFullHelp = !m.showFullHelp
			m.updatePagination()

		// Show errors.
		case "!":
			if m.err != nil && m.viewState == stashStateReady {
				m.viewState = stashStateShowingError

				return nil
			}
		}
	}

	// Update paginator. Pagination key handling is done here, but it could
	// also be moved up to this level, in which case we'd use model methods
	// like model.PageUp().
	newPaginatorModel, cmd := m.paginator().Update(msg)
	m.setPaginator(newPaginatorModel)
	cmds = append(cmds, cmd)

	// Extra paginator keystrokes.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "b", "u":
			m.paginator().PrevPage()
		case "f", "d":
			m.paginator().NextPage()
		}
	}

	// Keep the index in bounds when paginating.
	itemsOnPage := m.paginator().ItemsOnPage(len(m.getVisibleYAMLs()))
	if m.cursor() > itemsOnPage-1 {
		m.setCursor(max(0, itemsOnPage-1))
	}

	return tea.Batch(cmds...)
}

// Updates for when a user is in the filter editing interface.
func (m *stashModel) handleFiltering(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle keys.
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case keyEsc:
			// Cancel filtering.
			m.resetFiltering()
		case keyEnter, "tab", "shift+tab", "ctrl+k", "up", "ctrl+j", "down":
			m.hideStatusMessage()

			if len(m.yamls) == 0 {
				break
			}

			h := m.getVisibleYAMLs()

			// If we've filtered down to nothing, clear the filter.
			if len(h) == 0 {
				m.viewState = stashStateReady
				m.resetFiltering()

				break
			}

			// When there's only one filtered yaml left we can just "open" it directly.
			if len(h) == 1 {
				m.viewState = stashStateReady
				m.resetFiltering()
				cmds = append(cmds, m.openYAML(h[0]))

				break
			}

			// Add new section if it's not present.
			if m.sections[len(m.sections)-1].key != filterSection {
				m.sections = append(m.sections, sections[filterSection])
			}
			m.sectionIndex = len(m.sections) - 1

			m.filterInput.Blur()

			m.filterState = filterApplied
			if m.filterInput.Value() == "" {
				m.resetFiltering()
			}
		}
	}

	// Update the filter text input component.
	newFilterInputModel, inputCmd := m.filterInput.Update(msg)
	currentFilterVal := m.filterInput.Value()
	newFilterVal := newFilterInputModel.Value()
	m.filterInput = newFilterInputModel
	cmds = append(cmds, inputCmd)

	// If the filtering input has changed, request updated filtering.
	if newFilterVal != currentFilterVal {
		cmds = append(cmds, filterYAMLs(*m))
	}

	// Update pagination.
	m.updatePagination()

	return tea.Batch(cmds...)
}

// VIEW.

func (m stashModel) view() string {
	var s string
	switch m.viewState {
	case stashStateShowingError:
		return errorView(m.err, false)
	case stashStateLoadingDocument:
		s += " " + m.spinner.View() + " Loading document..."
	case stashStateReady:
		loadingIndicator := " "
		if m.shouldSpin() {
			loadingIndicator = m.spinner.View()
		}

		// Only draw the normal header if we're not using the header area for
		// something else (like a note or delete prompt).
		header := m.headerView()

		// Rules for the logo, filter and status message.
		logoOrFilter := " "
		if m.filterState == filtering {
			if m.showStatusMessage {
				logoOrFilter += m.statusMessage.String()
			} else {
				logoOrFilter += m.filterInput.View()
			}
		} else {
			logoOrFilter += katLogoView()
			if m.showStatusMessage {
				logoOrFilter += "  " + m.statusMessage.String()
			}
		}
		logoOrFilter = truncate.StringWithTail(logoOrFilter, uint(max(0, m.common.width-1)), ellipsis) //nolint:gosec // Uses max.

		help, helpHeight := m.helpView()

		populatedView := m.populatedView()
		populatedViewHeight := strings.Count(populatedView, "\n") + 2

		// We need to fill any empty height with newlines so the footer reaches
		// the bottom.
		availHeight := m.common.height -
			stashViewTopPadding -
			populatedViewHeight -
			helpHeight -
			stashViewBottomPadding
		blankLines := strings.Repeat("\n", max(0, availHeight))

		var pagination string
		if m.paginator().TotalPages > 1 {
			pagination = m.paginator().View()

			// If the dot pagination is wider than the width of the window
			// use the arabic paginator.
			if ansi.PrintableRuneWidth(pagination) > m.common.width-stashViewHorizontalPadding {
				// Copy the paginator since m.paginator() returns a pointer to
				// the active paginator and we don't want to mutate it. In
				// normal cases, where the paginator is not a pointer, we could
				// safely change the model parameters for rendering here as the
				// current model is discarded after reuturning from a View().
				// One could argue, in fact, that using pointers in
				// a functional framework is an antipattern and our use of
				// pointers in our model should be refactored away.
				p := *(m.paginator())
				p.Type = paginator.Arabic
				pagination = paginationStyle.Render(p.View())
			}
		}

		s += fmt.Sprintf(
			"%s%s\n\n  %s\n\n%s\n\n%s  %s\n\n%s",
			loadingIndicator,
			logoOrFilter,
			header,
			populatedView,
			blankLines,
			pagination,
			help,
		)
	}

	return "\n" + indent(s, stashIndent)
}

func katLogoView() string {
	return logoStyle.Render(fmt.Sprintf(" kat %s ", version.Revision))
}

func (m stashModel) headerView() string {
	localCount := len(m.yamls)
	sections := []string{}

	// Filter results.
	if m.filterState == filtering {
		if localCount == 0 {
			return grayFg("Nothing found.")
		}
		if localCount > 0 {
			sections = append(sections, fmt.Sprintf("%d local", localCount))
		}

		for i := range sections {
			sections[i] = grayFg(sections[i])
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
			s = selectedTabStyle.Render(s)
		} else {
			s = tabStyle.Render(s)
		}
		sections = append(sections, s)
	}

	return strings.Join(sections, dividerBar.String())
}

func (m stashModel) populatedView() string {
	mds := m.getVisibleYAMLs()

	var b strings.Builder

	// Empty states.
	if len(mds) == 0 {
		f := func(s string) {
			b.WriteString("  " + grayFg(s))
		}

		switch m.sections[m.sectionIndex].key {
		case documentsSection:
			if m.loadingDone() {
				f("No files found.")
			} else {
				f("Looking for local files...")
			}
		case filterSection:
			return ""
		}
	}

	if len(mds) > 0 {
		start, end := m.paginator().GetSliceBounds(len(mds))
		docs := mds[start:end]

		for i, md := range docs {
			stashItemView(&b, m, i, md)
			if i != len(docs)-1 {
				fmt.Fprintf(&b, "\n\n")
			}
		}
	}

	// If there aren't enough items to fill up this page (always the last page)
	// then we need to add some newlines to fill up the space where stash items
	// would have been.
	itemsOnPage := m.paginator().ItemsOnPage(len(mds))
	if itemsOnPage < m.paginator().PerPage {
		n := (m.paginator().PerPage - itemsOnPage) * stashViewItemHeight
		if len(mds) == 0 {
			n -= stashViewItemHeight - 1
		}
		for range n {
			fmt.Fprint(&b, "\n")
		}
	}

	return b.String()
}

// COMMANDS.

func loadLocalYAML(md *yaml) tea.Cmd {
	return func() tea.Msg {
		return fetchedYAMLMsg(md)
	}
}

func filterYAMLs(m stashModel) tea.Cmd {
	return func() tea.Msg {
		if m.filterInput.Value() == "" || !m.filterApplied() {
			return filteredYAMLMsg(m.yamls) // Return everything.
		}

		targets := []string{}
		mds := m.yamls

		for _, t := range mds {
			targets = append(targets, t.filterValue)
		}

		ranks := fuzzy.Find(m.filterInput.Value(), targets)
		sort.Stable(ranks)

		filtered := []*yaml{}
		for _, r := range ranks {
			filtered = append(filtered, mds[r.Index])
		}

		return filteredYAMLMsg(filtered)
	}
}
