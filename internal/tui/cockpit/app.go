package cockpit

import (
	"context"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// screen identifies which view the cockpit is currently routing
// keys and rendering for. Adding a screen is two changes: a new
// constant here plus a case in Update / View. Keeping this an enum
// (rather than a stack of sub-models) is enough for the v2.1
// shape; promote later if a real overlay system is needed.
type screen int

const (
	// screenDashboard is the default split-pane view: tree on the
	// left, branch detail on the right. The skeleton renders a
	// loading placeholder until the first snapshot arrives.
	screenDashboard screen = iota
	// screenConflict is the in-TUI conflict resolver, entered when
	// any in-process action returns a *service.PausedError. Wired
	// in todo `conflict_screen`.
	screenConflict
	// screenDiff is the per-branch diff viewer. Wired in todo
	// `diff_viewer`.
	screenDiff
	// screenPalette is the Ctrl+P / Cmd+K command picker. Wired in
	// todo `palette`.
	screenPalette
	// screenHelp is the bindings overlay. Wired in todo
	// `help_overlay`.
	screenHelp
)

// Model is the root tea.Model. It owns the snapshot loaded from the
// service layer, the active screen, terminal dimensions, and the
// keymap. Per-screen state will live in dedicated structs nested
// here as later todos land.
type Model struct {
	ctx  context.Context
	svc  *service.Service
	keys Keymap

	screen   screen
	width    int
	height   int
	snapshot *service.DashboardSnapshot
	// loadErr surfaces the most recent snapshot-load failure so the
	// dashboard can render a banner instead of going dark. Cleared
	// on the next successful load.
	loadErr error
	// quitting is set by the Quit binding so View can short-circuit
	// to an empty string and let the alt-screen tear-down land on a
	// clean terminal.
	quitting bool

	// cursor is the currently-selected row in the dashboard's tree
	// pane, indexing into snapshot.CheckoutItems. Bounded by Update
	// so render code can read it without re-checking len().
	cursor int
	// firstLoad is cleared after the first snapshot arrives. Used
	// to position the cursor on the current branch on boot without
	// stomping the user's selection across subsequent refreshes.
	firstLoad bool
	// details and detailErrs cache the result of Service.Show per
	// branch so cursor movement doesn't re-hit gh / git. Both reset
	// on refresh so the user can force a fresh detail read.
	details    map[string]*service.BranchView
	detailErrs map[string]error

	// lastAction holds the outcome of the most recent local action
	// dispatched from the dashboard so the status line can render
	// it. Cleared by an explicit refresh keypress so a stale "✓"
	// doesn't outlive the user's mental model of the stack state.
	lastAction *actionResult
	// statusRefreshing tracks the live GitHub status refresh that
	// updates SQLite in the background. The dashboard keeps rendering
	// the last cached status while this is true.
	statusRefreshing bool
	// statusAutoRefreshed prevents the initial background refresh
	// from re-triggering after every snapshot repaint.
	statusAutoRefreshed bool
	// statusErr is the most recent live-status refresh failure. It is
	// intentionally separate from loadErr so a GitHub hiccup does not
	// hide the local stack view.
	statusErr error

	// conflictCursor is the index into snapshot.Paused.ConflictPaths
	// for the conflict resolver's edit-target selection. Reset to 0
	// when entering the conflict screen so a stale cursor from a
	// previously-resolved rebase doesn't leak across sessions.
	conflictCursor int

	// pendingPausedRoute is set by network-action handlers and
	// consumed (one-shot) by routeAfterSnapshot. It exists because
	// shell-out subcommands can leave the on-disk state paused
	// (e.g. `sm sync` triggers a restack that conflicts), and the
	// only way the cockpit learns about it is the next snapshot
	// reload. Without this flag the user would land back on the
	// dashboard staring at a paused banner instead of straight
	// into the resolver.
	pendingPausedRoute bool

	// diff* fields back the diff viewer screen. They live on the
	// root Model rather than a nested struct because Update is a
	// value receiver — copying state across calls is the
	// bubbletea idiom and a flat layout keeps that copy cheap.
	//
	// diffBranch is the branch the viewer was entered from.
	// diffCommits is captured at entry so an async snapshot
	// refresh that overwrites m.details doesn't pull commits out
	// from under the user mid-scroll. diffCache / diffErrs map
	// SHA → diff content / error so commit stepping is
	// memoised.
	diffBranch       string
	diffCommits      []service.CommitView
	diffCommitCursor int
	diffScroll       int
	diffCache        map[string]string
	diffErrs         map[string]error

	// palette* fields back the command palette overlay-screen.
	// prevScreen is what closePalette returns to so the palette
	// is non-disruptive: opening from the dashboard returns to
	// the dashboard, opening from anywhere else returns there.
	// paletteQuery is the running filter string and
	// paletteCursor is the selected row in the filtered list.
	prevScreen    screen
	paletteQuery  string
	paletteCursor int
}

// New constructs a Model wired to ctx and svc. ctx flows through to
// every snapshot load so an outer cancellation (signal handler in
// main.go) propagates into in-flight reads.
func New(ctx context.Context, svc *service.Service) Model {
	return Model{
		ctx:        ctx,
		svc:        svc,
		keys:       DefaultKeymap(),
		screen:     screenDashboard,
		firstLoad:  true,
		details:    map[string]*service.BranchView{},
		detailErrs: map[string]error{},
		diffCache:  map[string]string{},
		diffErrs:   map[string]error{},
	}
}

// snapshotMsg is delivered when an asynchronous Snapshot load
// completes. err non-nil means the load failed; snap is nil in that
// case.
type snapshotMsg struct {
	snap *service.DashboardSnapshot
	err  error
}

// detailMsg is delivered when an asynchronous Service.Show load
// completes for a single branch. The branch field lets Update
// route the result to the right cache entry even when the user has
// since moved the cursor (so a slow detail read doesn't overwrite
// the freshly-cached one for a different branch).
type detailMsg struct {
	branch string
	view   *service.BranchView
	err    error
}

// statusRefreshMsg is delivered after a live GitHub status refresh.
// The command fetches remote state and writes SQLite; Update folds the
// returned rows into the current snapshot so badges change in place.
type statusRefreshMsg struct {
	report service.StatusReport
	err    error
}

// loadSnapshotCmd kicks off a background read of the dashboard
// payload. It captures the model's ctx so an outer cancellation
// short-circuits the in-flight call.
func loadSnapshotCmd(ctx context.Context, svc *service.Service) tea.Cmd {
	return func() tea.Msg {
		snap, err := svc.Snapshot(ctx, service.LogOptions{})
		return snapshotMsg{snap: snap, err: err}
	}
}

// loadDetailCmd fetches one branch's BranchView via Service.Show.
// branch is captured in the closure so the resulting detailMsg
// always carries the right key for the cache.
func loadDetailCmd(ctx context.Context, svc *service.Service, branch string) tea.Cmd {
	return func() tea.Msg {
		view, err := svc.Show(ctx, branch)
		return detailMsg{branch: branch, view: view, err: err}
	}
}

func refreshGitHubStatusCmd(ctx context.Context, svc *service.Service) tea.Cmd {
	return func() tea.Msg {
		report, err := svc.Status(ctx)
		return statusRefreshMsg{report: report, err: err}
	}
}

// Init kicks off the first snapshot load so the dashboard renders
// real data on first paint instead of the loading placeholder.
func (m Model) Init() tea.Cmd {
	return loadSnapshotCmd(m.ctx, m.svc)
}

// Update routes messages to per-screen handlers. The skeleton only
// implements the dashboard handler; later todos add cases as new
// screens land.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case snapshotMsg:
		if msg.err != nil {
			m.loadErr = msg.err
			// Keep the previous snapshot visible on refresh errors.
			if msg.snap == nil {
				return m, nil
			}
		} else {
			m.loadErr = nil
		}
		m.snapshot = msg.snap
		if msg.snap != nil {
			// routeAfterSnapshot reads firstLoad to decide on the
			// first-load-into-paused jump, so it has to run before
			// positionCursorAfterSnapshot clears the flag.
			m.routeAfterSnapshot(msg.snap)
			m.positionCursorAfterSnapshot(msg.snap)
		}
		detailCmd := m.detailCmdForSelection()
		var statusCmd tea.Cmd
		if !m.statusAutoRefreshed && snapshotHasPR(msg.snap) {
			m.statusAutoRefreshed = true
			m, statusCmd = m.startStatusRefresh()
		}
		return m, tea.Batch(detailCmd, statusCmd)
	case detailMsg:
		if msg.err != nil {
			m.detailErrs[msg.branch] = msg.err
			delete(m.details, msg.branch)
		} else {
			m.details[msg.branch] = msg.view
			delete(m.detailErrs, msg.branch)
		}
		return m, nil
	case statusRefreshMsg:
		m.statusRefreshing = false
		if msg.err != nil {
			m.statusErr = msg.err
			return m, nil
		}
		m.statusErr = nil
		if m.snapshot != nil {
			mergeStatusReport(m.snapshot, msg.report)
		}
		return m, nil
	case diffMsg:
		if msg.err != nil {
			m.diffErrs[msg.sha] = msg.err
			delete(m.diffCache, msg.sha)
		} else {
			m.diffCache[msg.sha] = msg.content
			delete(m.diffErrs, msg.sha)
		}
		return m, nil
	case actionResult:
		// Always record the outcome so the status line can render
		// it even if the action failed.
		res := msg
		m.lastAction = &res
		switch {
		case res.Paused != nil:
			// Route the user into the conflict resolver
			// immediately. Snapshot reload populates Paused on the
			// snapshot, which the resolver reads for its file
			// list and pending-queue display.
			m.screen = screenConflict
			m.conflictCursor = 0
			m.details = map[string]*service.BranchView{}
			m.detailErrs = map[string]error{}
			return m, loadSnapshotCmd(m.ctx, m.svc)
		case res.Err != nil:
			// Plain failure: keep the current snapshot so the user
			// can see what state things are still in. No reload.
			return m, nil
		default:
			// Success: invalidate caches and pull fresh data so
			// the dashboard reflects the mutation immediately.
			// routeAfterSnapshot handles bouncing back to the
			// dashboard if a continue/abort cleared the paused
			// state.
			m.details = map[string]*service.BranchView{}
			m.detailErrs = map[string]error{}
			return m, loadSnapshotCmd(m.ctx, m.svc)
		}
	case networkFinishedMsg:
		// Translate the subprocess exit into a status-line entry
		// so the user can see "submit ✓" or "submit failed: ..."
		// without scrolling back through the post-exec terminal
		// output. Then arm the paused-route flag and reload —
		// the subprocess may have left paused state behind.
		m.lastAction = &actionResult{Verb: msg.Verb, Err: msg.Err}
		m.pendingPausedRoute = true
		m.details = map[string]*service.BranchView{}
		m.detailErrs = map[string]error{}
		return m, loadSnapshotCmd(m.ctx, m.svc)
	case editorFinishedMsg:
		// Editor exits don't change service state, but the user
		// almost certainly wrote something — refresh so any new
		// stage/unstage state reflects in the conflict-paths list
		// (git counts a fully-resolved file as no-longer-unmerged
		// once it's added). The error becomes a status line entry
		// so a missing $EDITOR is visible.
		if msg.err != nil {
			m.lastAction = &actionResult{
				Verb:   m.keys.ConflictEdit.Help,
				Branch: msg.path,
				Err:    msg.err,
			}
			return m, nil
		}
		return m, loadSnapshotCmd(m.ctx, m.svc)
	case tea.KeyMsg:
		// Global keys are gated on the palette screen because the
		// palette captures every keystroke as part of its query —
		// firing Quit on `q` or Refresh on `R` while the user is
		// typing would be a UX disaster. ctrl+c is the one
		// universal escape hatch and is always honoured so a
		// runaway palette can never trap the user.
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		if m.screen != screenPalette {
			if m.keys.Quit.Matches(msg) {
				m.quitting = true
				return m, tea.Quit
			}
			if m.keys.Refresh.Matches(msg) {
				// Force a fresh detail read on the next
				// snapshot so the user sees the impact of
				// whatever change prompted the refresh. Also
				// drop the stale action banner — a manual
				// refresh is the user saying "start over".
				m.details = map[string]*service.BranchView{}
				m.detailErrs = map[string]error{}
				m.lastAction = nil
				return m, loadSnapshotCmd(m.ctx, m.svc)
			}
			if m.keys.PaletteOpen.Matches(msg) {
				return m.openPalette(), nil
			}
			if m.keys.Help.Matches(msg) && m.screen != screenHelp {
				return m.openHelp(), nil
			}
		}
		// Per-screen routing. Each screen owns its own update
		// helper so the switch here stays a one-liner per screen.
		switch m.screen {
		case screenDashboard:
			return m.updateDashboard(msg)
		case screenConflict:
			return m.updateConflict(msg)
		case screenDiff:
			return m.updateDiff(msg)
		case screenPalette:
			return m.updatePalette(msg)
		case screenHelp:
			return m.updateHelp(msg)
		}
	}
	return m, nil
}

// routeAfterSnapshot decides which screen to land on after a fresh
// snapshot arrives. Two transitions are automated:
//
//   - First load with a paused rebase → enter the conflict
//     resolver immediately. Without this the user has to discover
//     that the dashboard's paused banner is actionable.
//   - Resolver has nothing left to resolve → bounce back to the
//     dashboard. Triggered by a successful continue/abort or an
//     external `sm continue` between snapshots, this keeps the
//     user from being stranded on a now-stale screen.
//
// All other transitions are user-driven (Back from the resolver,
// action-paused dispatch into the resolver) so the cockpit doesn't
// surprise the user mid-workflow. Must be called before
// positionCursorAfterSnapshot because the first-load case reads
// the firstLoad flag.
func (m *Model) routeAfterSnapshot(snap *service.DashboardSnapshot) {
	// pendingPausedRoute is one-shot so a stray flag doesn't keep
	// hijacking the user across unrelated refreshes. Consumed
	// regardless of which branch below fires.
	postNetwork := m.pendingPausedRoute
	m.pendingPausedRoute = false

	switch {
	case m.screen == screenConflict && snap.Paused == nil:
		m.screen = screenDashboard
		m.conflictCursor = 0
	case m.firstLoad && snap.Paused != nil:
		m.screen = screenConflict
		m.conflictCursor = 0
	case postNetwork && snap.Paused != nil && m.screen != screenConflict:
		// A shell-out (sync / land / submit) just finished and
		// left paused state — drop the user into the resolver
		// instead of stranding them on the dashboard. Mirrors the
		// in-process actionResult.Paused → resolver path.
		m.screen = screenConflict
		m.conflictCursor = 0
	case m.screen == screenConflict && snap.Paused != nil:
		// Cursor clamp for the in-resolver refresh case (e.g.
		// after an edit-finished refresh that resolved one of
		// several conflict paths). Keeps the caret inside bounds
		// instead of pointing at a path git no longer reports.
		if n := len(snap.Paused.ConflictPaths); n == 0 {
			m.conflictCursor = 0
		} else if m.conflictCursor >= n {
			m.conflictCursor = n - 1
		}
	}
}

// positionCursorAfterSnapshot keeps the cursor pointing at something
// reasonable across snapshot churn:
//   - On the very first snapshot, jump to the row matching the
//     current branch so the user lands where they were in git.
//   - On subsequent snapshots, preserve the cursor index but clamp
//     it to the new bounds in case items shrunk underneath it.
func (m *Model) positionCursorAfterSnapshot(snap *service.DashboardSnapshot) {
	n := len(snap.CheckoutItems)
	if n == 0 {
		m.cursor = 0
		m.firstLoad = false
		return
	}
	if m.firstLoad {
		for i, it := range snap.CheckoutItems {
			if it.IsCurrent {
				m.cursor = i
				break
			}
		}
		m.firstLoad = false
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// selectedBranch returns the branch name of the currently-cursored
// row, or "" when there is no snapshot or no items. Callers route
// through this so detail fetches can stay no-ops on an empty stack.
func (m Model) selectedBranch() string {
	if m.snapshot == nil {
		return ""
	}
	items := m.snapshot.CheckoutItems
	if len(items) == 0 || m.cursor < 0 || m.cursor >= len(items) {
		return ""
	}
	return items[m.cursor].Branch
}

// detailCmdForSelection returns a load command for the branch under
// the cursor when its detail isn't already cached (success or
// failure). Returns nil when nothing needs fetching so callers can
// safely chain it into return values.
func (m Model) detailCmdForSelection() tea.Cmd {
	branch := m.selectedBranch()
	if branch == "" {
		return nil
	}
	if _, ok := m.details[branch]; ok {
		return nil
	}
	if _, ok := m.detailErrs[branch]; ok {
		return nil
	}
	return loadDetailCmd(m.ctx, m.svc, branch)
}

func (m Model) startStatusRefresh() (Model, tea.Cmd) {
	if m.svc == nil || m.statusRefreshing {
		return m, nil
	}
	m.statusRefreshing = true
	m.statusErr = nil
	return m, refreshGitHubStatusCmd(m.ctx, m.svc)
}

func snapshotHasPR(snap *service.DashboardSnapshot) bool {
	if snap == nil {
		return false
	}
	for _, it := range snap.CheckoutItems {
		if it.PR > 0 {
			return true
		}
	}
	return false
}

func mergeStatusReport(snap *service.DashboardSnapshot, report service.StatusReport) {
	if snap.GitHubStatus == nil {
		snap.GitHubStatus = map[string]service.StatusBranch{}
	}
	for _, st := range report.Branches {
		if st.Branch == "" || st.Err != "" {
			continue
		}
		snap.GitHubStatus[st.Branch] = st
	}
}

// View renders the active screen. Returning an empty string while
// quitting lets bubbletea's alt-screen exit on a clean terminal.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	switch m.screen {
	case screenDashboard:
		return m.viewDashboard()
	case screenConflict:
		return m.viewConflict()
	case screenDiff:
		return m.viewDiff()
	case screenPalette:
		return m.viewPalette()
	case screenHelp:
		return m.viewHelp()
	}
	// Unhandled screens during incremental development are caught
	// here so a missing case surfaces as a visible diagnostic
	// instead of an empty pane.
	return fmt.Sprintf("(cockpit: screen %d not yet implemented)\n", m.screen)
}

// Run launches the cockpit and blocks until the user quits or ctx
// is cancelled. It uses the alt screen so the cockpit doesn't
// pollute the user's terminal scrollback.
func Run(ctx context.Context, svc *service.Service) error {
	p := tea.NewProgram(
		New(ctx, svc),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
