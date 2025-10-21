package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	cursor "github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wordwrap"

	"github.com/kong/kongctl/internal/iostreams"
	kai "github.com/kong/kongctl/internal/kai"
	"github.com/kong/kongctl/internal/kai/render"
	"github.com/kong/kongctl/internal/kai/storage"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/theme"
)

// Options configure the interactive chat experience.
type Options struct {
	BaseURL            string
	Token              string
	UseColor           bool
	SessionNameFactory func() string
	SessionID          string
	SessionHistory     *kai.SessionHistory
	LookupControlPlane func(context.Context, string) (string, error)
	InitialTasks       []kai.TaskDetails
	Version            string
	Theme              theme.Palette
}

type message struct {
	sender   string
	content  string
	duration time.Duration
}

type exchange struct {
	prompt   string
	response string
}

type taskEntry struct {
	id           string
	taskType     string
	confirmation string
	context      []kai.ChatContext
	status       string
	metadata     map[string]any
	toolError    string
	updates      []string
	statusNote   string
}

type taskStatus struct {
	visible    bool
	taskID     string
	taskLabel  string
	status     string
	message    string
	cancelable bool
	finished   bool
}

type taskActionResultMsg struct {
	taskID  string
	details *kai.TaskDetails
	action  kai.TaskAction
	err     error
}

type taskStatusStartedMsg struct {
	taskID string
	stream *kai.TaskStatusStream
	cancel context.CancelFunc
}

type taskStatusEventMsg struct {
	event kai.TaskStatusEvent
}

type taskStatusFinishedMsg struct{}

type taskStatusErrorMsg struct {
	err error
}

type activeTasksLoadedMsg struct {
	tasks []kai.TaskDetails
	err   error
}

type taskAnalyzeStartedMsg struct {
	taskID string
	stream *kai.Stream
	cancel context.CancelFunc
}

type taskAnalyzeEventMsg struct {
	event kai.ChatEvent
}

type taskAnalyzeFinishedMsg struct{}

type taskAnalyzeErrorMsg struct {
	err error
}

type contextEntry struct {
	entity kai.ChatContext
	name   string
}

type slashCommand struct {
	name        string
	description string
}

type themeStyles struct {
	statusStyle        lipgloss.Style
	thinkingStyle      lipgloss.Style
	questionStyle      lipgloss.Style
	bannerAccent       lipgloss.AdaptiveColor
	promptAccent       lipgloss.AdaptiveColor
	promptPlaceholder  lipgloss.AdaptiveColor
	promptHelperStyle  lipgloss.Style
	promptThinking     lipgloss.AdaptiveColor
	promptSuccess      lipgloss.AdaptiveColor
	promptError        lipgloss.AdaptiveColor
	promptBorderStyle  lipgloss.Style
	bannerHeadingStyle lipgloss.Style
	taskBorder         lipgloss.AdaptiveColor
	taskStatusBarStyle lipgloss.Style
}

const (
	userSpeaker         = "You"
	agentSpeaker        = "Kai"
	toolSpeaker         = "Tool"
	taskSpeaker         = "Task"
	defaultPrompt       = "Ask a question... Ctrl+C to exit"
	promptSymbol        = "› "
	promptMinHeight     = 1
	promptMaxHeight     = 8
	defaultPromptWidth  = 60
	maxTaskDetailLines  = 6
	taskAnalysisMarker  = "TASK_ANALYSIS::"
	clearTaskStatusHelp = "Use /clear-task-status to clear the task status bar"
)

func buildThemeStyles(p theme.Palette) themeStyles {
	return themeStyles{
		statusStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorSuccessText)).
			Background(p.Adaptive(theme.ColorSuccess)).
			Padding(0),
		thinkingStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorHighlight)),
		questionStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorAccent)),
		bannerAccent:      p.Adaptive(theme.ColorPrimary),
		promptAccent:      p.Adaptive(theme.ColorAccent),
		promptPlaceholder: p.Adaptive(theme.ColorTextMuted),
		promptHelperStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorTextMuted)),
		promptThinking: p.Adaptive(theme.ColorHighlight),
		promptSuccess:  p.Adaptive(theme.ColorSuccess),
		promptError:    p.Adaptive(theme.ColorDanger),
		promptBorderStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorBorder)),
		bannerHeadingStyle: lipgloss.NewStyle().
			Foreground(p.Adaptive(theme.ColorTextPrimary)).
			Bold(true),
		taskBorder: p.Adaptive(theme.ColorInfo),
		taskStatusBarStyle: lipgloss.NewStyle().
			Background(p.Adaptive(theme.ColorInfo)).
			Foreground(p.Adaptive(theme.ColorInfoText)).
			Padding(0, 1, 0, 0),
	}
}

var defaultSlashCommands = []slashCommand{
	{name: "/context-control-plane", description: "Load a control plane into the Kai context"},
	{name: "/context-clear", description: "Remove all resource contexts"},
	{name: "/clear", description: "Clear chat history"},
	{name: "/clear-task-status", description: "Hide the current task status bar"},
	{name: "/quit", description: "Exit the Kai chat"},
}

type model struct {
	ctx  context.Context
	opts Options

	palette theme.Palette
	styles  themeStyles

	input            textarea.Model
	spinner          spinner.Model
	spinnerActive    bool
	messages         []message
	history          []exchange
	pendingPrompt    string
	pendingContexts  []kai.ChatContext
	reconnecting     bool
	sessionID        string
	sessionName      string
	sessionCreatedAt time.Time
	recorder         *storage.SessionRecorder
	resumed          bool
	knownHistoryIDs  map[string]struct{}

	streaming      bool
	stream         *kai.Stream
	streamCancel   context.CancelFunc
	pendingBuilder strings.Builder
	streamErr      error
	responseStart  time.Time

	contextEntries           []contextEntry
	commandList              []slashCommand
	filteredCommands         []slashCommand
	lookupControlPlane       func(context.Context, string) (string, error)
	contextLoading           bool
	width                    int
	initialTasks             []kai.TaskDetails
	tasks                    map[string]*taskEntry
	pendingTask              *taskEntry
	awaitingTaskDecision     bool
	taskActionInFlight       bool
	activeTask               *taskEntry
	pendingAnalysisTask      *taskEntry
	awaitingAnalysisDecision bool
	taskStatus               string
	taskStatusStream         *kai.TaskStatusStream
	taskStatusCancel         context.CancelFunc
	analyzeStream            *kai.Stream
	analyzeCancel            context.CancelFunc
	analyzeBuilder           strings.Builder
	sessionLimitReached      bool
	taskStatusBar            taskStatus
}

type chatStreamStartedMsg struct {
	stream *kai.Stream
	cancel context.CancelFunc
}

type chatEventMsg struct {
	event kai.ChatEvent
}

type chatStreamFinishedMsg struct{}

type chatStreamErrorMsg struct {
	err error
}

type chatRecoveryHistoryMsg struct {
	history  *kai.SessionHistory
	prompt   string
	contexts []kai.ChatContext
	err      error
}

type (
	readyMsg          struct{}
	sessionCreatedMsg struct {
		id        string
		name      string
		createdAt time.Time
	}
)

type sessionErrorMsg struct {
	err error
}

type contextAddedMsg struct {
	entry contextEntry
}

type contextErrorMsg struct {
	err error
}

// Run launches the interactive chat session.
func Run(ctx context.Context, streams *iostreams.IOStreams, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}

	logger := kai.ContextLogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelInfo, "kai session start",
			slog.String("session_id", strings.TrimSpace(opts.SessionID)),
			slog.Bool("has_history", opts.SessionHistory != nil))
	}

	m := newModel(ctx, opts)
	program := tea.NewProgram(m,
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
		tea.WithoutSignalHandler(),
	)

	finalModel, err := program.Run()

	if logger != nil {
		sessionID := strings.TrimSpace(m.sessionID)
		if fm, ok := finalModel.(*model); ok && strings.TrimSpace(fm.sessionID) != "" {
			sessionID = strings.TrimSpace(fm.sessionID)
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "kai session end",
			slog.String("session_id", sessionID),
			slog.Bool("had_error", err != nil))
	}

	return err
}

func newModel(ctx context.Context, opts Options) *model {
	pal := opts.Theme
	if strings.TrimSpace(pal.Name) == "" {
		pal = theme.FromContext(ctx)
	}
	if strings.TrimSpace(pal.Name) == "" {
		pal = theme.Current()
	}
	styles := buildThemeStyles(pal)

	input := textarea.New()
	input.Placeholder = defaultPrompt
	input.Prompt = promptSymbol
	input.ShowLineNumbers = false
	input.CharLimit = 0
	input.MaxHeight = promptMaxHeight
	input.SetHeight(promptMinHeight)
	input.Focus()
	input.Cursor.SetMode(cursor.CursorStatic)
	focusedStyle, blurredStyle := textarea.DefaultStyles()
	resetTextareaStyle := func(style *textarea.Style) {
		style.Base = lipgloss.NewStyle()
		style.CursorLine = lipgloss.NewStyle()
		style.CursorLineNumber = lipgloss.NewStyle()
		style.EndOfBuffer = lipgloss.NewStyle()
		style.LineNumber = lipgloss.NewStyle()
		style.Text = lipgloss.NewStyle()
		style.Placeholder = lipgloss.NewStyle().Foreground(styles.promptPlaceholder)
		style.Prompt = lipgloss.NewStyle().Foreground(styles.promptAccent)
	}
	resetTextareaStyle(&focusedStyle)
	resetTextareaStyle(&blurredStyle)
	input.FocusedStyle = focusedStyle
	input.BlurredStyle = blurredStyle
	input.Cursor.Style = lipgloss.NewStyle().Foreground(styles.promptAccent)
	promptWidth := lipgloss.Width(promptSymbol)
	input.SetPromptFunc(promptWidth, func(lineIdx int) string {
		if lineIdx == 0 {
			return promptSymbol
		}
		return strings.Repeat(" ", promptWidth)
	})
	input.SetWidth(defaultPromptWidth)

	sp := spinner.New()
	sp.Style = styles.thinkingStyle

	m := &model{
		ctx:                ctx,
		opts:               opts,
		palette:            pal,
		styles:             styles,
		input:              input,
		spinner:            sp,
		spinnerActive:      true,
		commandList:        append([]slashCommand(nil), defaultSlashCommands...),
		lookupControlPlane: opts.LookupControlPlane,
		tasks:              make(map[string]*taskEntry),
		initialTasks:       opts.InitialTasks,
		resumed:            opts.SessionHistory != nil,
		knownHistoryIDs:    make(map[string]struct{}),
	}

	if opts.SessionHistory != nil {
		if opts.SessionHistory.ID != "" {
			m.sessionID = opts.SessionHistory.ID
		}
		m.applyHistory(opts.SessionHistory)
	} else if opts.SessionID != "" {
		m.sessionID = opts.SessionID
	}

	if strings.TrimSpace(m.sessionID) != "" {
		lifecycle := ""
		if m.resumed {
			lifecycle = "session_resumed"
		}
		if created := m.ensureRecorder(m.sessionID, m.sessionName, m.sessionCreatedAt); created && lifecycle != "" {
			m.recordLifecycleEvent(lifecycle, map[string]any{
				"history_loaded": opts.SessionHistory != nil,
			})
		}
	}

	if logger := kai.ContextLogger(ctx); logger != nil && m.sessionID != "" {
		logger.LogAttrs(ctx, slog.LevelInfo, "kai session loaded",
			slog.String("session_id", m.sessionID),
			slog.Bool("has_history", opts.SessionHistory != nil))
	}

	m.refreshPlaceholder()
	m.adjustInputHeight()

	return m
}

func (m *model) applyHistory(history *kai.SessionHistory) {
	if history == nil {
		return
	}
	if history.Name != "" {
		m.sessionName = sanitizeSessionName(history.Name)
	}
	if !history.CreatedAt.IsZero() {
		m.sessionCreatedAt = history.CreatedAt
	}
	for _, item := range history.History {
		if item.ID != "" {
			m.knownHistoryIDs[item.ID] = struct{}{}
		}
		m.ingestContext(item)
		role := strings.ToLower(item.Role)
		content := item.Message

		var sender string
		switch role {
		case "human":
			sender = userSpeaker
			m.history = append(m.history, exchange{prompt: content})
		case "ai":
			if shouldSuppressAIMessage(item) {
				continue
			}
			sender = agentSpeaker
			if len(m.history) > 0 {
				last := &m.history[len(m.history)-1]
				if last.response == "" {
					last.response = content
				}
			}
		case "task":
			continue
		default:
			sender = agentSpeaker
		}
		if sender == toolSpeaker || sender == "" {
			continue
		}
		m.messages = append(m.messages, message{sender: sender, content: content, duration: 0})
	}
}

func (m *model) ingestHistoryDiff(history *kai.SessionHistory) bool {
	if history == nil {
		return false
	}
	if history.Name != "" {
		m.sessionName = sanitizeSessionName(history.Name)
	}
	if !history.CreatedAt.IsZero() {
		m.sessionCreatedAt = history.CreatedAt
	}
	agentAdded := false
	for _, item := range history.History {
		if item.ID != "" {
			if _, exists := m.knownHistoryIDs[item.ID]; exists {
				continue
			}
			m.knownHistoryIDs[item.ID] = struct{}{}
		}
		if m.ingestHistoryItem(item) {
			agentAdded = true
		}
	}
	return agentAdded
}

func (m *model) ingestHistoryItem(item kai.SessionHistoryItem) bool {
	m.ingestContext(item)
	role := strings.ToLower(strings.TrimSpace(item.Role))
	if role != "ai" {
		return false
	}
	if shouldSuppressAIMessage(item) {
		return false
	}
	response := strings.TrimSpace(item.Message)
	if response == "" {
		return false
	}
	if len(m.messages) > 0 {
		last := m.messages[len(m.messages)-1]
		if last.sender == agentSpeaker && strings.TrimSpace(last.content) == response {
			return false
		}
	}
	m.messages = append(m.messages, message{sender: agentSpeaker, content: response, duration: 0})
	if len(m.history) > 0 {
		last := &m.history[len(m.history)-1]
		if last.response == "" {
			last.response = response
		} else {
			m.history = append(m.history, exchange{prompt: "", response: response})
		}
	} else {
		m.history = append(m.history, exchange{prompt: "", response: response})
	}
	m.recordAgentResponse(response, 0)
	return true
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick, func() tea.Msg { return readyMsg{} }}
	if m.sessionID == "" {
		cmds = append(cmds, m.createSessionCmd())
	}
	if cmd := m.applyInitialTasks(m.initialTasks); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.initialTasks = nil
	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer m.refreshPlaceholder()
	switch msg := msg.(type) {
	case readyMsg:
		return m, nil
	case sessionCreatedMsg:
		m.sessionID = msg.id
		m.sessionName = sanitizeSessionName(msg.name)
		m.sessionCreatedAt = msg.createdAt
		if created := m.ensureRecorder(m.sessionID, m.sessionName, m.sessionCreatedAt); created {
			m.recordLifecycleEvent("session_created", map[string]any{
				"name": strings.TrimSpace(m.sessionName),
			})
		}
		m.messages = append(m.messages, message{
			sender:   agentSpeaker,
			content:  fmt.Sprintf("Session ready: %s", sanitizeSessionName(msg.name)),
			duration: 0,
		})
		if logger := kai.ContextLogger(m.ctx); logger != nil {
			logger.LogAttrs(m.ctx, slog.LevelInfo, "kai session created",
				slog.String("session_id", msg.id),
				slog.String("name", msg.name))
		}
		if cmd := m.loadActiveTasksCmd(); cmd != nil {
			return m, cmd
		}
		return m, nil
	case sessionErrorMsg:
		var limitErr *kai.SessionLimitError
		if errors.As(msg.err, &limitErr) {
			m.notifySessionLimit(limitErr.Detail)
			return m, nil
		}
		m.notifyError(msg.err)
		return m, tea.Quit
	case contextAddedMsg:
		m.contextLoading = false
		m.upsertContext(msg.entry)
		return m, nil
	case contextErrorMsg:
		m.contextLoading = false
		m.notifyError(msg.err)
		return m, nil
	case spinner.TickMsg:
		if !m.spinnerActive {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
			width := msg.Width - 4
			if width < 20 {
				width = 20
			}
			m.input.SetWidth(width)
			m.adjustInputHeight()
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.stopStream()
			return m, tea.Quit
		}
		key := strings.ToLower(msg.String())
		if key == "ctrl+t" {
			if cmd := m.cancelTask(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		if m.sessionLimitReached {
			return m, nil
		}
		if m.awaitingTaskDecision && !m.taskActionInFlight {
			return m, m.handleTaskDecisionKey(msg)
		}
		if m.awaitingAnalysisDecision && m.analyzeStream == nil {
			return m, m.handleAnalysisDecisionKey(msg)
		}
		if msg.Type == tea.KeyTab {
			if m.completeSuggestion() {
				return m, nil
			}
		}
		if msg.Type == tea.KeyEnter {
			if m.streaming || m.sessionID == "" || m.taskActionInFlight {
				return m, nil
			}
			if m.taskStatusBar.visible && m.taskStatusBar.finished {
				m.clearTaskStatusBar()
			}
			raw := m.input.Value()
			if cmd, handled := m.executeSlashCommand(raw); handled {
				m.input.SetValue("")
				m.input.CursorEnd()
				m.updateSuggestions()
				m.adjustInputHeight()
				return m, cmd
			}

			value := strings.TrimSpace(raw)
			if value == "" {
				return m, nil
			}

			m.messages = append(m.messages, message{sender: userSpeaker, content: value, duration: 0})
			m.pendingPrompt = value
			m.pendingBuilder.Reset()
			m.streaming = true
			m.input.SetValue("")
			m.adjustInputHeight()

			m.responseStart = time.Now()

			contexts := m.contextsForRequest()
			m.pendingContexts = append([]kai.ChatContext(nil), contexts...)
			m.recordUserMessage(value, contexts)

			prompt := buildPrompt(m.history, value)
			cmd := startStreamCmd(m.ctx, m.opts, m.sessionID, prompt, contexts)
			return m, cmd
		}

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.updateSuggestions()
		return m, cmd
	case chatStreamStartedMsg:
		m.stream = msg.stream
		m.streamCancel = msg.cancel
		return m, waitForEvent(msg.stream)
	case chatEventMsg:
		evt := msg.event
		if evt.Event == "task" {
			if details := parseTaskDetails(evt.Data); details != nil {
				if cmd := m.handleTaskProposal(details, "agent_event"); cmd != nil {
					return m, cmd
				}
			}
		}
		switch evt.Event {
		case "llm-response":
			if text, ok := eventText(evt); ok {
				m.pendingBuilder.WriteString(text)
			}
		case "error":
			msg := eventError(evt)
			if m.streamErr == nil {
				m.streamErr = fmt.Errorf("kai agent error: %s", msg)
			}
			m.recordAgentError(msg)
		}
		return m, waitForEvent(m.stream)
	case chatStreamFinishedMsg:
		response := strings.TrimSpace(m.pendingBuilder.String())
		var duration time.Duration
		if !m.responseStart.IsZero() {
			duration = time.Since(m.responseStart)
		}
		if m.streamErr != nil {
			m.notifyError(m.streamErr)
		} else if response != "" {
			m.messages = append(m.messages, message{sender: agentSpeaker, content: response, duration: duration})
			m.history = append(m.history, exchange{prompt: m.pendingPrompt, response: response})
		} else {
			m.history = append(m.history, exchange{prompt: m.pendingPrompt, response: ""})
		}
		if m.streamErr == nil {
			m.recordAgentResponse(response, duration)
		}

		m.cleanupAfterStream()
		return m, nil
	case chatStreamErrorMsg:
		m.recordStreamError(msg.err)
		if msg.err != nil && kai.IsTransientError(msg.err) && strings.TrimSpace(m.sessionID) != "" {
			prompt := m.pendingPrompt
			contexts := append([]kai.ChatContext(nil), m.pendingContexts...)
			m.cleanupAfterStream()
			m.pendingPrompt = prompt
			m.pendingContexts = contexts
			if prompt != "" {
				m.reconnecting = true
				m.notifyKaiMessage("Connection lost. Attempting to recover the latest response...")
				return m, recoverChatHistoryCmd(m.ctx, m.opts, m.sessionID, prompt, contexts)
			}
		}
		if !errors.Is(msg.err, context.Canceled) {
			m.notifyError(msg.err)
		}
		m.cleanupAfterStream()
		return m, nil
	case chatRecoveryHistoryMsg:
		m.reconnecting = false
		if msg.err != nil {
			if kai.IsTransientError(msg.err) {
				m.notifyKaiMessage("Still reconnecting... please retry shortly.")
			} else {
				m.notifyError(msg.err)
			}
			return m, nil
		}
		if msg.history != nil && msg.history.ID != "" && strings.TrimSpace(m.sessionID) == "" {
			m.sessionID = msg.history.ID
		}
		if added := m.ingestHistoryDiff(msg.history); added {
			m.pendingPrompt = ""
			m.pendingContexts = nil
			m.pendingBuilder.Reset()
			m.notifyKaiMessage("Recovered response from session history.")
			return m, nil
		}
		prompt := strings.TrimSpace(msg.prompt)
		if prompt == "" {
			return m, nil
		}
		m.pendingPrompt = prompt
		m.pendingContexts = append([]kai.ChatContext(nil), msg.contexts...)
		m.pendingBuilder.Reset()
		m.streamErr = nil
		m.streaming = true
		m.responseStart = time.Now()
		m.notifyKaiMessage("Reconnected. Replaying your last prompt...")
		return m, startStreamCmd(m.ctx, m.opts, m.sessionID, prompt, msg.contexts)
	case taskActionResultMsg:
		m.taskActionInFlight = false
		entry := m.upsertTask(msg.details)
		if entry == nil {
			if m.pendingTask != nil && m.pendingTask.id == msg.taskID {
				entry = m.pendingTask
			} else if m.activeTask != nil && m.activeTask.id == msg.taskID {
				entry = m.activeTask
			}
		}
		if msg.err != nil {
			m.recordTaskAction(entry, msg.action, msg.err)
			m.notifyError(msg.err)
			if msg.action == kai.TaskActionStart {
				m.awaitingTaskDecision = true
			}
			return m, nil
		}
		m.recordTaskAction(entry, msg.action, nil)
		switch msg.action {
		case kai.TaskActionStart:
			if entry.toolError != "" {
				m.pendingTask = nil
				m.awaitingTaskDecision = false
				m.activeTask = nil
				m.pendingAnalysisTask = nil
				m.awaitingAnalysisDecision = false
				m.taskStatus = formatTaskStatusLabel("failed")
				m.presentTaskFailure(entry)
				return m, nil
			}
			m.pendingTask = nil
			m.awaitingTaskDecision = false
			m.activeTask = entry
			entry.status = strings.ToLower(entry.status)
			if entry.statusNote == "" || entry.statusNote == "Pending approval" {
				entry.statusNote = "Approved"
			}
			m.taskStatus = formatTaskStatusLabel(entry.status)
			m.addTaskUpdate(entry, "Task approved and starting", false)
			if cmd := m.startTaskStatusStream(entry); cmd != nil {
				return m, cmd
			}
		case kai.TaskActionStop:
			if m.pendingTask != nil && m.pendingTask.id == entry.id {
				m.pendingTask = nil
				m.awaitingTaskDecision = false
			}
			if m.activeTask != nil && m.activeTask.id == entry.id {
				m.stopTaskStatusStream()
				m.activeTask = nil
			}
			if m.pendingAnalysisTask != nil && m.pendingAnalysisTask.id == entry.id {
				m.pendingAnalysisTask = nil
				m.awaitingAnalysisDecision = false
			}
			entry.status = strings.ToLower(entry.status)
			m.taskStatus = formatTaskStatusLabel(entry.status)
			m.addTaskUpdate(entry, "Task stopped", true)
		}
		return m, nil
	case taskStatusStartedMsg:
		m.taskStatusStream = msg.stream
		m.taskStatusCancel = msg.cancel
		if m.activeTask != nil {
			m.activeTask.status = "in_progress"
		}
		if m.taskStatus == "" {
			m.taskStatus = formatTaskStatusLabel("in_progress")
		}
		return m, waitForTaskStatus(msg.stream)
	case taskStatusEventMsg:
		status := strings.ToLower(msg.event.Status)
		entry := m.activeTask
		if entry == nil {
			entry = m.pendingTask
		}
		if entry != nil && status != "" {
			entry.status = status
		}
		finished := isTaskFinishedStatus(status)
		displayMsg := ""
		switch status {
		case "analyzable":
			if m.activeTask != nil {
				m.pendingAnalysisTask = m.activeTask
			}
			m.awaitingAnalysisDecision = true
			m.taskStatus = formatTaskStatusLabel(status)
			m.stopTaskStatusStream()
			if m.pendingAnalysisTask != nil {
				m.presentTaskSummary(m.pendingAnalysisTask)
			}
			displayMsg = "Awaiting analysis decision (Y to analyze, N to skip)"
		case "done":
			m.stopTaskStatusStream()
			m.activeTask = nil
			m.taskStatus = formatTaskStatusLabel(status)
			displayMsg = "Task completed"
			finished = true
		case "cancelled":
			m.stopTaskStatusStream()
			m.activeTask = nil
			m.taskStatus = formatTaskStatusLabel(status)
			displayMsg = "Task cancelled"
			finished = true
		case "stopped":
			m.stopTaskStatusStream()
			m.activeTask = nil
			m.taskStatus = formatTaskStatusLabel(status)
			displayMsg = "Task stopped"
			finished = true
		case "failed", "error":
			m.stopTaskStatusStream()
			m.activeTask = nil
			m.taskStatus = formatTaskStatusLabel(status)
			displayMsg = "Task failed"
			finished = true
		default:
			if status != "" {
				m.taskStatus = formatTaskStatusLabel(status)
				displayMsg = statusDisplayLabel(status)
				if strings.TrimSpace(displayMsg) == "" {
					displayMsg = friendlyStatus(status)
				}
				if strings.TrimSpace(displayMsg) == "" {
					displayMsg = humanizeLabel(status)
				}
			}
		}
		if msg.event.Message != "" && status != "analyzable" {
			displayMsg = msg.event.Message
		}
		if entry != nil && displayMsg != "" {
			if logger := kai.ContextLogger(m.ctx); logger != nil {
				logger.LogAttrs(m.ctx, slog.LevelInfo, "kai task update",
					slog.String("session_id", strings.TrimSpace(m.sessionID)),
					slog.String("task_id", entry.id),
					slog.String("status", entry.status),
					slog.String("message", displayMsg))
			}
			barStatus := status
			if barStatus == "" && entry != nil {
				barStatus = entry.status
			}
			cancelable := entry != nil && strings.ToLower(strings.TrimSpace(barStatus)) == "in_progress"
			m.updateTaskStatusBar(entry, barStatus, displayMsg, cancelable, finished)
		}
		m.recordTaskStatus(entry, status, displayMsg, finished, msg.event.Message)
		if m.taskStatusStream != nil {
			return m, waitForTaskStatus(m.taskStatusStream)
		}
		return m, nil
	case taskStatusFinishedMsg:
		m.stopTaskStatusStream()
		return m, nil
	case taskStatusErrorMsg:
		m.stopTaskStatusStream()
		if msg.err != nil && kai.IsTransientError(msg.err) && strings.TrimSpace(m.sessionID) != "" {
			m.notifyKaiMessage("Lost task status updates. Refreshing...")
			if cmd := m.loadActiveTasksCmd(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		m.notifyError(msg.err)
		entry := m.activeTask
		if entry == nil {
			entry = m.pendingTask
		}
		message := ""
		if msg.err != nil {
			message = msg.err.Error()
		}
		m.recordTaskStatus(entry, "error", "", true, message)
		return m, nil
	case activeTasksLoadedMsg:
		if msg.err != nil {
			m.notifyError(msg.err)
			return m, nil
		}
		if cmd := m.applyInitialTasks(msg.tasks); cmd != nil {
			return m, cmd
		}
		return m, nil
	case taskAnalyzeStartedMsg:
		if m.pendingAnalysisTask != nil {
			m.pendingAnalysisTask.status = "analyzing"
			state := "analyzing"
			m.updateTaskStatusBar(m.pendingAnalysisTask, state, "Under analysis...", false, false)
		}
		m.analyzeStream = msg.stream
		m.analyzeCancel = msg.cancel
		m.analyzeBuilder.Reset()
		m.taskStatus = formatTaskStatusLabel("analyzing")
		entry := m.pendingAnalysisTask
		if entry == nil {
			entry = m.activeTask
		}
		if entry != nil {
			m.recordTaskStatus(entry, "analyzing", "Under analysis...", false, "")
		}
		return m, waitForAnalyzeEvent(msg.stream)
	case taskAnalyzeEventMsg:
		evt := msg.event
		switch evt.Event {
		case "llm-response":
			if text, ok := eventText(evt); ok {
				m.analyzeBuilder.WriteString(text)
			}
		case "tool-response":
			if formatted := formatToolResponse(evt.Data); formatted != "" {
				m.messages = append(m.messages, message{sender: agentSpeaker, content: formatted, duration: 0})
			}
		case "error":
			msgText := eventError(evt)
			m.notifyErrorText(msgText)
			m.recordTaskStatus(m.pendingAnalysisTask, "analysis_error", msgText, true, msgText)
			m.stopAnalyzeStream()
			return m, nil
		}
		return m, waitForAnalyzeEvent(m.analyzeStream)
	case taskAnalyzeFinishedMsg:
		response := strings.TrimSpace(m.analyzeBuilder.String())
		entry := m.pendingAnalysisTask
		if entry != nil {
			entry.statusNote = "Analysis complete"
		}
		if response != "" {
			if entry != nil {
				m.appendTaskAnalysis(entry, response)
			} else {
				m.messages = append(m.messages, message{sender: agentSpeaker, content: response, duration: 0})
			}
		}
		m.stopAnalyzeStream()
		m.analyzeBuilder.Reset()
		if entry != nil {
			entry.status = "analysis_complete"
			m.updateTaskStatusBar(entry, "analysis_complete", "", false, true)
			m.recordTaskStatus(entry, entry.status, "", true, response)
		}
		m.pendingAnalysisTask = nil
		m.awaitingAnalysisDecision = false
		m.activeTask = nil
		m.taskStatus = ""
		return m, nil
	case taskAnalyzeErrorMsg:
		m.stopAnalyzeStream()
		if msg.err != nil && kai.IsTransientError(msg.err) && strings.TrimSpace(m.sessionID) != "" {
			if m.pendingAnalysisTask != nil {
				m.notifyKaiMessage("Analysis stream interrupted. Attempting to resume...")
				if cmd := m.startAnalyzeTask(m.pendingAnalysisTask); cmd != nil {
					return m, cmd
				}
			}
			return m, nil
		}
		errorMsg := ""
		if m.pendingAnalysisTask != nil {
			m.pendingAnalysisTask.status = "analysis_error"
			errorMsg = fmt.Sprintf("Analysis error: %v", msg.err)
			m.updateTaskStatusBar(m.pendingAnalysisTask, "analysis_error", errorMsg, false, true)
		}
		m.notifyError(msg.err)
		entry := m.pendingAnalysisTask
		if entry == nil {
			entry = m.activeTask
		}
		raw := ""
		if msg.err != nil {
			raw = msg.err.Error()
		}
		if errorMsg == "" {
			errorMsg = raw
		}
		m.recordTaskStatus(entry, "analysis_error", errorMsg, true, raw)
		return m, nil
	}

	if m.sessionLimitReached {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.adjustInputHeight()
	return m, cmd
}

func (m *model) View() string {
	var b strings.Builder

	if banner := m.renderBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n\n")
	}

	if m.sessionID == "" {
		if m.sessionLimitReached {
			b.WriteString("Session unavailable\n\n")
		} else {
			b.WriteString("Establishing session...\n\n")
		}
	}

	if m.pendingAnalysisTask != nil && m.awaitingAnalysisDecision {
		if prompt := m.renderAnalysisPrompt(m.pendingAnalysisTask); prompt != "" {
			b.WriteString(prompt)
			b.WriteString("\n\n")
		}
	}

	lastWasAgent := false
	for i, msg := range m.messages {
		switch msg.sender {
		case agentSpeaker:
			content := msg.content
			header := m.renderAgentBanner(false, msg.duration)
			if label, body, ok := splitTaskAnalysisContent(content); ok {
				content = body
				header = m.renderAgentAnalysisBanner(label)
			}
			if lastWasAgent {
				b.WriteString("\n")
			}
			b.WriteString(header)
			b.WriteString("\n")
			b.WriteString(m.renderMarkdown(content))
			lastWasAgent = true
		case taskSpeaker:
			entry := m.tasks[strings.TrimSpace(msg.content)]
			if lastWasAgent {
				b.WriteString("\n")
			}
			if entry != nil {
				b.WriteString(m.renderTaskCard(entry))
			} else {
				b.WriteString(m.renderTaskFallback(msg.content))
			}
			lastWasAgent = false
		default:
			lastWasAgent = false
			b.WriteString(m.renderUserLine(msg.content))
		}
		if i < len(m.messages)-1 {
			b.WriteString("\n\n")
		} else {
			b.WriteString("\n")
		}
	}

	if m.streaming || m.pendingBuilder.Len() > 0 {
		elapsed := time.Duration(0)
		if !m.responseStart.IsZero() {
			elapsed = time.Since(m.responseStart)
		}
		b.WriteString(m.renderAgentBanner(true, elapsed))
		b.WriteString("\n")
		switch {
		case m.pendingBuilder.Len() > 0:
			b.WriteString(m.renderMarkdown(m.pendingBuilder.String()))
		}
		b.WriteString("\n")
	}

	if suggestions := m.renderSuggestions(); suggestions != "" {
		b.WriteString(suggestions)
		b.WriteString("\n")
	}

	if len(m.messages) > 0 || m.streaming || m.pendingBuilder.Len() > 0 {
		b.WriteString("\n")
	}

	b.WriteString(m.renderPrompt())
	b.WriteString("\n")
	if bar := m.renderTaskStatusBar(); bar != "" {
		b.WriteString(bar)
		b.WriteString("\n")
	}
	b.WriteString(m.renderContextLine())
	b.WriteString("\n")

	return b.String()
}

func (m *model) renderBanner() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	const (
		minBannerWidth = 60
		minLeftWidth   = 20
		minRightWidth  = 28
		separatorWidth = 1
	)

	innerWidth := width - 2
	minInnerWidth := minLeftWidth + minRightWidth + separatorWidth
	if width < minBannerWidth || innerWidth < minInnerWidth {
		return m.renderBannerFallback(width)
	}

	leftWidth := (innerWidth - separatorWidth) * 35 / 100
	if leftWidth < minLeftWidth {
		leftWidth = minLeftWidth
	}
	rightWidth := innerWidth - separatorWidth - leftWidth
	if rightWidth < minRightWidth {
		adjust := minRightWidth - rightWidth
		leftWidth -= adjust
		rightWidth = minRightWidth
	}
	if leftWidth < minLeftWidth {
		return m.renderBannerFallback(width)
	}

	borderStyle := lipgloss.NewStyle()
	accent := lipgloss.NewStyle().Bold(true)
	if m.opts.UseColor {
		borderStyle = borderStyle.Foreground(m.styles.bannerAccent)
		accent = m.styles.bannerHeadingStyle
	}

	leftLines := m.bannerLeftPanel(leftWidth, accent)
	rightLines := m.bannerRightPanel(rightWidth, accent)

	rows := maxInt(len(leftLines), len(rightLines))
	leftLines = normalizeLines(leftLines, rows)
	rightLines = normalizeLines(rightLines, rows)

	version := strings.TrimSpace(m.opts.Version)
	if version == "" {
		version = "dev"
	}
	var topLine string
	var bottomLine string
	if m.opts.UseColor {
		nameSegment := m.styles.bannerHeadingStyle.Render(" " + strings.TrimSpace(meta.CLIName))
		versionSegment := m.styles.promptHelperStyle.Render(" (v" + version + ") ")
		kaiSegment := m.styles.bannerHeadingStyle.Render("Kai ")
		titleSegment := lipgloss.JoinHorizontal(lipgloss.Top, nameSegment, versionSegment, kaiSegment)
		titleWidth := lipgloss.Width(titleSegment)
		cutoff := innerWidth - 1
		if cutoff <= 0 {
			plain := fmt.Sprintf(" %s (v%s) Kai ", strings.TrimSpace(meta.CLIName), version)
			titleSegment = plain
			titleWidth = lipgloss.Width(titleSegment)
		} else if titleWidth > cutoff {
			plain := fmt.Sprintf(" %s (v%s) Kai ", strings.TrimSpace(meta.CLIName), version)
			if lipgloss.Width(plain) > cutoff {
				plain = truncate.String(plain, uint(cutoff))
			}
			titleSegment = plain
			titleWidth = lipgloss.Width(titleSegment)
		}
		dashCount := innerWidth - 1 - titleWidth
		if dashCount < 0 {
			dashCount = 0
		}
		cornerStyle := lipgloss.NewStyle().Foreground(m.styles.bannerAccent)
		dashStyle := lipgloss.NewStyle().Foreground(m.styles.bannerAccent)
		topLine = lipgloss.JoinHorizontal(lipgloss.Top,
			cornerStyle.Render("╭"),
			dashStyle.Render("─"),
			titleSegment,
			dashStyle.Render(strings.Repeat("─", dashCount)),
			cornerStyle.Render("╮"),
		)
		bottomLine = lipgloss.JoinHorizontal(lipgloss.Top,
			cornerStyle.Render("╰"),
			dashStyle.Render(strings.Repeat("─", innerWidth)),
			cornerStyle.Render("╯"),
		)
	} else {
		titleText := fmt.Sprintf(" %s (v%s) kai ", strings.TrimSpace(meta.CLIName), version)
		cutoff := innerWidth - 1
		if cutoff <= 0 {
			titleText = ""
		} else if lipgloss.Width(titleText) > cutoff {
			titleText = truncate.String(titleText, uint(cutoff))
		}
		padding := innerWidth - 1 - lipgloss.Width(titleText)
		if padding < 0 {
			padding = 0
		}
		topLine = "╭─" + titleText + strings.Repeat("─", padding) + "╮"
		bottomLine = "╰" + strings.Repeat("─", innerWidth) + "╯"
	}

	var body strings.Builder
	for i := 0; i < rows; i++ {
		if i > 0 {
			body.WriteString("\n")
		}
		left := padToWidth(leftLines[i], leftWidth)
		right := padToWidth(rightLines[i], rightWidth)
		body.WriteString(borderStyle.Render("│"))
		body.WriteString(left)
		body.WriteString(borderStyle.Render("│"))
		body.WriteString(right)
		body.WriteString(borderStyle.Render("│"))
	}

	var out strings.Builder
	if m.opts.UseColor {
		out.WriteString(topLine)
	} else {
		out.WriteString(borderStyle.Render(topLine))
	}
	out.WriteString("\n")
	out.WriteString(body.String())
	out.WriteString("\n")
	if m.opts.UseColor {
		out.WriteString(bottomLine)
	} else {
		out.WriteString(borderStyle.Render(bottomLine))
	}

	return out.String()
}

func (m *model) bannerLeftPanel(width int, accent lipgloss.Style) []string {
	lines := []string{""}

	heading := "Welcome to Kai"
	if strings.TrimSpace(m.sessionID) == "" {
		heading = "Preparing Kai..."
	}
	lines = append(lines, accent.Render(heading))
	lines = append(lines, "")
	lines = append(lines, m.bannerSessionLine())

	if host := m.bannerKonnectHost(); host != "" {
		lines = append(lines, fmt.Sprintf("Konnect host: %s", host))
	}

	if desc := wrapParagraph("Kai helps you explore and manage Kong Konnect.", width); len(desc) > 0 {
		lines = append(lines, "")
		lines = append(lines, desc...)
	}

	lines = append(lines, "")
	return lines
}

func (m *model) bannerRightPanel(width int, accent lipgloss.Style) []string {
	lines := []string{""}

	lines = append(lines, accent.Render("Usage tips"))

	lines = append(lines, wrapBullet(
		"Ask a question below to start a conversation with Kai.",
		width,
	)...)
	lines = append(lines, wrapBullet(
		"Use /context-control-plane <name> to load a Konnect control plane.",
		width,
	)...)
	lines = append(lines, wrapBullet(
		"Try /clear to reset the chat window.",
		width,
	)...)
	lines = append(lines, wrapBullet(
		"Use /quit to end the session.",
		width,
	)...)
	lines = append(lines, wrapBullet(
		"Kai surfaces guided tasks whenever it can perform actions on your behalf.",
		width,
	)...)

	lines = append(lines, "")
	return lines
}

func (m *model) bannerSessionLine() string {
	if name := strings.TrimSpace(m.sessionName); name != "" {
		return fmt.Sprintf("Session: %s", sanitizeSessionName(name))
	}
	if id := strings.TrimSpace(m.sessionID); id != "" {
		return fmt.Sprintf("Session: %s", trimIdentifier(id))
	}
	return "Session: establishing..."
}

func (m *model) bannerKonnectHost() string {
	raw := strings.TrimSpace(m.opts.BaseURL)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if host := parsed.Hostname(); host != "" {
		return host
	}
	return parsed.Host
}

func (m *model) renderBannerFallback(width int) string {
	title := "Kai Chat"
	if m.sessionName != "" {
		title = fmt.Sprintf("%s (%s)", title, m.sessionName)
	}
	lineWidth := maxInt(width, 24)
	return fmt.Sprintf("%s\n%s", title, strings.Repeat("-", lineWidth))
}

func normalizeLines(lines []string, target int) []string {
	if len(lines) >= target {
		return lines
	}
	out := make([]string, target)
	copy(out, lines)
	return out
}

func wrapParagraph(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if width <= 0 {
		return []string{text}
	}
	wrapped := wordwrap.String(text, width)
	wrapped = strings.TrimSuffix(wrapped, "\n")
	if wrapped == "" {
		return nil
	}
	return strings.Split(wrapped, "\n")
}

func wrapBullet(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if width <= 2 {
		return []string{"• " + text}
	}
	bodyWidth := width - 2
	wrapped := wordwrap.String(text, bodyWidth)
	wrapped = strings.TrimSuffix(wrapped, "\n")
	if wrapped == "" {
		return []string{"• " + text}
	}
	rows := strings.Split(wrapped, "\n")
	lines := make([]string, len(rows))
	for i, row := range rows {
		row = strings.TrimRight(row, " ")
		if i == 0 {
			lines[i] = "• " + row
			continue
		}
		lines[i] = "  " + row
	}
	return lines
}

func padToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	trimmed := truncate.String(s, uint(width))
	diff := width - lipgloss.Width(trimmed)
	if diff > 0 {
		return trimmed + strings.Repeat(" ", diff)
	}
	return trimmed
}

func (m *model) renderMarkdown(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	return render.Markdown(raw, render.Options{NoColor: !m.opts.UseColor})
}

func (m *model) renderContextLine() string {
	left := "Resource Contexts: none"
	if len(m.contextEntries) > 0 {
		parts := make([]string, 0, len(m.contextEntries))
		for _, entry := range m.contextEntries {
			parts = append(parts, fmt.Sprintf("[%s]", contextLabel(entry)))
		}
		left = "Resource Contexts: " + strings.Join(parts, " ")
	}
	right := ""
	if m.taskStatus != "" {
		right = m.taskStatus
	} else if m.streaming {
		right = "Streaming..."
	}
	return m.renderStatusLine(left, right)
}

func (m *model) renderStatusLine(left, right string) string {
	width := maxInt(m.width, lipgloss.Width(left)+lipgloss.Width(right)+1)
	space := width - lipgloss.Width(left) - lipgloss.Width(right)
	if right != "" {
		if space < 1 {
			space = 1
		}
	} else if space < 0 {
		space = 0
	}
	content := left + strings.Repeat(" ", space)
	if right != "" {
		content += right
	}
	content = padToWidth(content, width)
	if !m.opts.UseColor {
		return content
	}
	return m.styles.statusStyle.Render(content)
}

func (m *model) adjustInputHeight() {
	if m.sessionLimitReached {
		if m.input.Height() != 1 {
			m.input.SetHeight(1)
		}
		return
	}

	trimmed := strings.TrimRight(m.input.View(), "\n")
	lines := 1
	if trimmed != "" {
		lines = strings.Count(trimmed, "\n") + 1
	}
	height := clampInt(lines, promptMinHeight, promptMaxHeight)
	if m.input.Height() != height {
		m.input.SetHeight(height)
	}
}

func (m *model) renderAgentBanner(streaming bool, duration time.Duration) string {
	width := maxInt(m.width, 24)
	message := "Kai"
	if streaming {
		message = fmt.Sprintf("Kai %s", m.spinner.View())
	} else if duration > 0 {
		message = fmt.Sprintf("Kai worked for %s", formatDuration(duration))
	}
	if m.opts.UseColor {
		message = m.styles.promptHelperStyle.Render(message)
	}
	bullet := "⏺"
	if m.opts.UseColor {
		bulletColor := m.styles.promptPlaceholder
		if streaming {
			bulletColor = m.styles.promptThinking
		} else if duration > 0 {
			bulletColor = m.styles.promptSuccess
		}
		bullet = lipgloss.NewStyle().Foreground(bulletColor).Render(bullet)
	}
	left := fmt.Sprintf("%s %s", bullet, message)
	lineWidth := lipgloss.Width(left)
	dashCount := width - lineWidth
	space := ""
	if dashCount > 0 {
		space = " "
		dashCount--
	}
	if dashCount < 0 {
		dashCount = 0
	}
	dashes := strings.Repeat("─", dashCount)
	if m.opts.UseColor {
		dashes = m.styles.promptBorderStyle.Render(dashes)
	}
	return left + space + dashes
}

func (m *model) renderAgentAnalysisBanner(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "Task"
	}
	return m.renderTaskHeader(fmt.Sprintf("Kai task analysis: %s", label))
}

func (m *model) renderTaskHeader(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	width := maxInt(m.width, lipgloss.Width(label)+6)
	if !m.opts.UseColor {
		bullet := "•"
		line := bullet + " " + label
		dashCount := width - lipgloss.Width(line) - 1
		if dashCount < 0 {
			dashCount = 0
		}
		return line + " " + strings.Repeat("-", dashCount)
	}
	bullet := lipgloss.NewStyle().Foreground(m.styles.taskBorder).Render("⏺")
	text := m.styles.promptHelperStyle.Render(label)
	line := lipgloss.JoinHorizontal(lipgloss.Top, bullet, m.styles.promptBorderStyle.Render(" "), text)
	dashCount := width - lipgloss.Width(line) - 1
	if dashCount < 0 {
		dashCount = 0
	}
	dashes := m.styles.promptBorderStyle.Render(strings.Repeat("─", dashCount))
	return lipgloss.JoinHorizontal(lipgloss.Top, line, m.styles.promptBorderStyle.Render(" "), dashes)
}

func (m *model) renderPrompt() string {
	view := m.input.View()
	trailing := 0
	trimmed := view
	for strings.HasSuffix(trimmed, "\n") {
		trimmed = strings.TrimSuffix(trimmed, "\n")
		trailing++
	}
	width := promptContentWidth(trimmed)
	if w := m.input.Width(); w > 0 {
		width = maxInt(width, w)
	}
	if width <= 0 {
		width = 1
	}
	padded := padPromptLines(trimmed, width)
	if trailing > 0 {
		padded += strings.Repeat("\n", trailing)
	}
	display := padded
	border := strings.Repeat("─", width)
	if m.opts.UseColor {
		border = m.styles.promptBorderStyle.Render(border)
	}
	return fmt.Sprintf("%s\n%s\n%s", border, display, border)
}

func (m *model) updateTaskStatusBar(entry *taskEntry, state string, message string, cancelable bool, finished bool) {
	if entry == nil {
		return
	}
	status := strings.TrimSpace(state)
	if status == "" {
		status = strings.TrimSpace(entry.status)
	}
	m.taskStatusBar = taskStatus{
		visible:    true,
		taskID:     strings.TrimSpace(entry.id),
		taskLabel:  taskDisplayLabel(entry),
		status:     status,
		message:    strings.TrimSpace(message),
		cancelable: cancelable,
		finished:   finished,
	}
}

func (m *model) clearTaskStatusBar() {
	m.taskStatusBar = taskStatus{}
}

func (m *model) renderTaskStatusBar() string {
	bar := m.taskStatusBar
	if !bar.visible {
		return ""
	}
	status := strings.TrimSpace(bar.status)
	label := strings.TrimSpace(bar.taskLabel)
	statusLabel := statusDisplayLabel(status)
	friendly := strings.TrimSpace(friendlyStatus(status))

	display := "Task"
	switch {
	case statusLabel != "" && label != "":
		display = fmt.Sprintf("%s – %s", statusLabel, label)
	case label != "":
		display = label
	case statusLabel != "":
		display = statusLabel
	}

	help := ""
	if bar.cancelable && !bar.finished {
		help = "Ctrl+T to stop"
	}

	message := strings.TrimSpace(bar.message)
	if !bar.finished && message == "" {
		message = friendly
	}

	line1 := display
	if !bar.finished && help != "" {
		line1 = fmt.Sprintf("%s  %s", line1, help)
	}

	var line2 string
	if bar.finished {
		candidate := strings.TrimSpace(message)
		if candidate != "" {
			normCandidate := strings.ToLower(candidate)
			if normCandidate == strings.ToLower(friendly) || normCandidate == strings.ToLower(statusLabel) {
				candidate = ""
			}
		}
		if candidate != "" {
			line2 = fmt.Sprintf("%s  %s", candidate, clearTaskStatusHelp)
		} else {
			line2 = clearTaskStatusHelp
		}
	} else {
		if message == "" {
			message = friendly
		}
		if message == "" {
			message = "Task update"
		}
		line2 = message
	}

	width := maxInt(m.width, maxInt(lipgloss.Width(line1), lipgloss.Width(line2)))
	line1 = padToWidth(line1, width)
	line2 = padToWidth(line2, width)
	if !m.opts.UseColor {
		border := strings.Repeat("-", width)
		return fmt.Sprintf("+%s+\n|%s|\n|%s|\n+%s+", border, line1, line2, border)
	}
	return m.styles.taskStatusBarStyle.Render(line1 + "\n" + line2)
}

func sanitizeSessionName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	collapsed := strings.Join(fields, " ")
	runes := []rune(collapsed)
	const maxNameLength = 80
	if len(runes) > maxNameLength {
		const suffix = "..."
		if maxNameLength > len(suffix) {
			return string(runes[:maxNameLength-len(suffix)]) + suffix
		}
		return suffix
	}
	return collapsed
}

func contextLabel(entry contextEntry) string {
	labelType := humanizeLabel(string(entry.entity.Type))
	name := strings.TrimSpace(entry.name)
	snippet := trimIdentifier(entry.entity.ID)
	if snippet != "" {
		if name == "" {
			name = snippet
		} else if !strings.Contains(strings.ToLower(name), strings.ToLower(snippet)) {
			name = fmt.Sprintf("%s (%s)", name, snippet)
		}
	}
	if name == "" {
		return labelType
	}
	return fmt.Sprintf("%s: %s", labelType, name)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	s := (d % time.Minute) / time.Second
	parts := make([]string, 0, 3)
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " ")
}

func shouldSuppressAIMessage(item kai.SessionHistoryItem) bool {
	if item.Tool != nil && strings.TrimSpace(item.Tool.Name) != "" {
		return true
	}
	if len(item.Context) > 0 {
		return true
	}
	s := strings.TrimSpace(item.Message)
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return true
	}
	return false
}

func formatTaskStatusLabel(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "", "done", "cancelled", "stopped":
		return ""
	case "pending":
		return "Task: pending approval"
	case "in_progress":
		return "Task: in progress"
	case "analyzable":
		return "Task: analyzable"
	case "analyzing":
		return "Task: analyzing"
	case "failed", "error":
		return "Task: failed"
	default:
		return "Task: " + strings.ReplaceAll(status, "_", " ")
	}
}

func (m *model) startTaskStatusStream(task *taskEntry) tea.Cmd {
	if task == nil || strings.TrimSpace(m.sessionID) == "" {
		return nil
	}
	baseURL := m.opts.BaseURL
	token := m.opts.Token
	id := task.id
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(m.ctx)
		stream, err := kai.StreamTaskStatus(ctx, nil, baseURL, token, m.sessionID, id)
		if err != nil {
			cancel()
			return taskStatusErrorMsg{err: err}
		}
		return taskStatusStartedMsg{taskID: id, stream: stream, cancel: cancel}
	}
}

func waitForTaskStatus(stream *kai.TaskStatusStream) tea.Cmd {
	if stream == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-stream.Events
		if !ok {
			if err := stream.Err(); err != nil {
				return taskStatusErrorMsg{err: err}
			}
			return taskStatusFinishedMsg{}
		}
		return taskStatusEventMsg{event: evt}
	}
}

func (m *model) stopTaskStatusStream() {
	if m.taskStatusCancel != nil {
		m.taskStatusCancel()
		m.taskStatusCancel = nil
	}
	m.taskStatusStream = nil
}

func (m *model) startAnalyzeTask(task *taskEntry) tea.Cmd {
	if task == nil || strings.TrimSpace(m.sessionID) == "" {
		return nil
	}
	baseURL := m.opts.BaseURL
	token := m.opts.Token
	id := task.id
	m.awaitingAnalysisDecision = false
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(m.ctx)
		stream, err := kai.AnalyzeTaskStream(ctx, nil, baseURL, token, m.sessionID, id)
		if err != nil {
			cancel()
			return taskAnalyzeErrorMsg{err: err}
		}
		return taskAnalyzeStartedMsg{taskID: id, stream: stream, cancel: cancel}
	}
}

func waitForAnalyzeEvent(stream *kai.Stream) tea.Cmd {
	if stream == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-stream.Events
		if !ok {
			if err := stream.Err(); err != nil {
				return taskAnalyzeErrorMsg{err: err}
			}
			return taskAnalyzeFinishedMsg{}
		}
		return taskAnalyzeEventMsg{event: evt}
	}
}

func (m *model) stopAnalyzeStream() {
	if m.analyzeCancel != nil {
		m.analyzeCancel()
		m.analyzeCancel = nil
	}
	m.analyzeStream = nil
}

func (m *model) taskActionCmd(action kai.TaskAction, task *taskEntry) tea.Cmd {
	if task == nil || strings.TrimSpace(m.sessionID) == "" {
		return nil
	}
	baseURL := m.opts.BaseURL
	token := m.opts.Token
	id := task.id
	m.taskActionInFlight = true
	return func() tea.Msg {
		details, err := kai.UpdateTask(m.ctx, nil, baseURL, token, m.sessionID, id, action)
		if err != nil {
			return taskActionResultMsg{taskID: id, action: action, err: err}
		}
		return taskActionResultMsg{taskID: id, action: action, details: details}
	}
}

func (m *model) approvePendingTask() tea.Cmd {
	if m.pendingTask == nil || m.taskActionInFlight {
		return nil
	}
	m.addTaskUpdate(m.pendingTask, "Approving task...", false)
	return m.taskActionCmd(kai.TaskActionStart, m.pendingTask)
}

func (m *model) declinePendingTask() tea.Cmd {
	if m.pendingTask == nil || m.taskActionInFlight {
		return nil
	}
	m.addTaskUpdate(m.pendingTask, "Declining task...", true)
	return m.taskActionCmd(kai.TaskActionStop, m.pendingTask)
}

func (m *model) cancelTask() tea.Cmd {
	if m.taskActionInFlight {
		return nil
	}
	if m.activeTask != nil {
		m.addTaskUpdate(m.activeTask, "Cancelling task...", true)
		return m.taskActionCmd(kai.TaskActionStop, m.activeTask)
	}
	if m.pendingTask != nil {
		m.addTaskUpdate(m.pendingTask, "Declining task...", true)
		return m.taskActionCmd(kai.TaskActionStop, m.pendingTask)
	}
	m.notifyKaiMessage("No active task to cancel.")
	return nil
}

func (m *model) handleTaskDecisionKey(msg tea.KeyMsg) tea.Cmd {
	key := strings.ToLower(msg.String())
	switch key {
	case "y", "enter", "a":
		return m.approvePendingTask()
	case "n", "d":
		return m.declinePendingTask()
	case "ctrl+c":
		return tea.Quit
	default:
		return nil
	}
}

func (m *model) handleAnalysisDecisionKey(msg tea.KeyMsg) tea.Cmd {
	key := strings.ToLower(msg.String())
	switch key {
	case "y", "enter", "a":
		m.addTaskUpdate(m.pendingAnalysisTask, "Analyzing task...", false)
		return m.startAnalyzeTask(m.pendingAnalysisTask)
	case "n", "d":
		m.addTaskUpdate(m.pendingAnalysisTask, "Skipped task analysis.", true)
		m.pendingAnalysisTask = nil
		m.awaitingAnalysisDecision = false
		m.taskStatus = ""
		return nil
	case "ctrl+c":
		return tea.Quit
	default:
		return nil
	}
}

func (m *model) renderAnalysisPrompt(task *taskEntry) string {
	if task == nil {
		return ""
	}
	text := "Task complete. Analyze results? (Y/N)"
	if m.opts.UseColor {
		return m.styles.questionStyle.Render(text)
	}
	return text
}

func formatToolResponse(data any) string {
	switch v := data.(type) {
	case map[string]any:
		if inner, ok := v["data"]; ok {
			if pretty, err := json.MarshalIndent(inner, "", "  "); err == nil {
				return fmt.Sprintf("```json\n%s\n```", string(pretty))
			}
		}
		if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
			return fmt.Sprintf("```json\n%s\n```", string(pretty))
		}
	case string:
		return v
	}
	return ""
}

func parseTaskDetails(data any) *kai.TaskDetails {
	if data == nil {
		return nil
	}
	candidate := data
	switch v := data.(type) {
	case map[string]any:
		if inner, ok := v["data"]; ok {
			candidate = inner
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" || !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return nil
		}
		if inner, ok := m["data"].(map[string]any); ok {
			candidate = inner
		} else {
			candidate = m
		}
	default:
		return nil
	}

	if m, ok := candidate.(map[string]any); ok {
		if task, ok := m["task"].(map[string]any); ok {
			candidate = task
		}
		raw, err := json.Marshal(candidate)
		if err != nil {
			return nil
		}
		var details kai.TaskDetails
		if err := json.Unmarshal(raw, &details); err == nil && details.ID != "" {
			return &details
		}
	}

	return nil
}

func (m *model) handleTaskProposal(details *kai.TaskDetails, origin string) tea.Cmd {
	if details == nil {
		return nil
	}
	entry := m.upsertTask(details)
	if entry == nil {
		return nil
	}
	status := strings.ToLower(details.Status)
	if status == "" {
		status = strings.ToLower(entry.status)
	}
	if status == "" {
		status = "pending"
	}
	entry.status = status
	m.recordTaskProposal(entry, status, origin)
	m.showTaskCard(entry)
	if entry.toolError != "" {
		m.stopTaskStatusStream()
		m.pendingTask = nil
		m.activeTask = nil
		m.pendingAnalysisTask = nil
		m.awaitingTaskDecision = false
		m.awaitingAnalysisDecision = false
		m.taskStatus = formatTaskStatusLabel("failed")
		m.presentTaskFailure(entry)
		return nil
	}

	switch status {
	case "pending":
		m.stopTaskStatusStream()
		m.pendingTask = entry
		m.activeTask = nil
		m.pendingAnalysisTask = nil
		m.awaitingTaskDecision = true
		m.awaitingAnalysisDecision = false
		m.taskStatus = formatTaskStatusLabel("pending")
		entry.updates = nil
		m.presentTaskProposal(entry)
		return nil
	case "in_progress":
		if entry.statusNote == "" || entry.statusNote == "Pending approval" {
			entry.statusNote = "Approved"
		}
		m.pendingTask = nil
		m.pendingAnalysisTask = nil
		m.awaitingTaskDecision = false
		m.awaitingAnalysisDecision = false
		m.activeTask = entry
		m.taskStatus = formatTaskStatusLabel("in_progress")
		m.stopTaskStatusStream()
		m.addTaskUpdate(entry, fmt.Sprintf("Task running: %s", humanizeLabel(entry.taskType)), false)
		return m.startTaskStatusStream(entry)
	case "analyzable":
		m.pendingTask = nil
		m.activeTask = nil
		m.pendingAnalysisTask = entry
		m.awaitingTaskDecision = false
		m.awaitingAnalysisDecision = true
		m.taskStatus = formatTaskStatusLabel("analyzable")
		m.presentTaskSummary(entry)
		return nil
	case "done", "cancelled", "stopped":
		m.pendingTask = nil
		m.activeTask = nil
		m.pendingAnalysisTask = nil
		m.awaitingTaskDecision = false
		m.awaitingAnalysisDecision = false
		m.taskStatus = formatTaskStatusLabel(status)
		m.addTaskUpdate(entry, fmt.Sprintf("Task ended (%s)", humanizeLabel(status)), true)
		return nil
	default:
		m.taskStatus = formatTaskStatusLabel(status)
	}

	return nil
}

func (m *model) presentTaskProposal(entry *taskEntry) {
	if entry == nil {
		return
	}
	entry.updates = nil
	entry.statusNote = "Pending approval"
	m.showTaskCard(entry)
	m.updateTaskStatusBar(entry, entry.status, "Awaiting approval (Y to approve, N to decline)", false, false)
}

func (m *model) presentTaskSummary(entry *taskEntry) {
	if entry == nil {
		return
	}
	entry.status = "analyzable"
	entry.statusNote = "Awaiting analysis"
	m.updateTaskStatusBar(entry, "analyzable", "Awaiting analysis decision (Y to analyze, N to skip)", false, false)
}

func (m *model) showTaskCard(entry *taskEntry) {
	if entry == nil {
		return
	}
	id := strings.TrimSpace(entry.id)
	if id == "" {
		return
	}
	for _, msg := range m.messages {
		if msg.sender == taskSpeaker && msg.content == id {
			return
		}
	}
	m.messages = append(m.messages, message{sender: taskSpeaker, content: id, duration: 0})
}

func (m *model) addTaskUpdate(entry *taskEntry, text string, finished bool) {
	if entry == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if finished {
		entry.statusNote = statusNoteFor(entry.status)
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelInfo, "kai task update",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("task_id", entry.id),
			slog.String("status", entry.status),
			slog.String("message", text))
	}
	status := strings.TrimSpace(entry.status)
	cancelable := strings.ToLower(status) == "in_progress"
	m.updateTaskStatusBar(entry, status, text, cancelable, finished)
}

func (m *model) notifyKaiMessage(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	m.messages = append(m.messages, message{
		sender:   agentSpeaker,
		content:  text,
		duration: 0,
	})
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelInfo, "kai ui message",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("text", text))
	}
}

func (m *model) notifySessionLimit(detail string) {
	detail = strings.TrimSpace(detail)
	m.sessionLimitReached = true
	m.spinnerActive = false
	m.contextLoading = false
	m.input.SetValue("")
	m.input.Blur()
	m.adjustInputHeight()
	messageParts := []string{
		"Kai can't start a new session because you've reached the maximum of 3 active sessions allowed for your account.",
		"- Run `kongctl kai resume` to continue an existing session (use the picker to resume or delete sessions).",
		"- Remove an existing session in the Konnect UI, then try again.",
	}
	message := strings.Join(messageParts, "\n\n")
	normalized := strings.ToLower(detail)
	if detail != "" && normalized != "maximum allowed sessions reached" {
		message = message + fmt.Sprintf("\n\nServer response: %s", detail)
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelWarn, "kai session limit",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("detail", detail))
	}
	m.notifyKaiMessage(message)
}

func (m *model) refreshPlaceholder() {
	if m.sessionLimitReached {
		m.input.Placeholder = "Press Ctrl+C to exit"
		return
	}
	switch {
	case m.reconnecting:
		m.input.Placeholder = "Reconnecting..."
		return
	case m.awaitingTaskDecision && !m.taskActionInFlight:
		m.input.Placeholder = "Press Y to approve, N to decline"
	case m.awaitingAnalysisDecision && m.analyzeStream == nil:
		m.input.Placeholder = "Press Y to analyze, N to skip"
	default:
		m.input.Placeholder = defaultPrompt
	}
}

func (m *model) notifyError(err error) {
	if err == nil {
		return
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelError, "kai ui error",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("error", err.Error()))
	}
	formatted := fmt.Sprintf("Error: %s", err.Error())
	m.notifyKaiMessage(formatted)
}

func (m *model) notifyErrorText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelError, "kai ui error",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("error", text))
	}
	formatted := fmt.Sprintf("Error: %s", text)
	m.notifyKaiMessage(formatted)
}

func (m *model) presentTaskFailure(entry *taskEntry) {
	if entry == nil {
		return
	}
	reason := strings.TrimSpace(entry.toolError)
	if reason == "" {
		return
	}
	label := strings.TrimSpace(humanizeLabel(entry.taskType))
	if label != "" {
		reason = fmt.Sprintf("%s failed: %s", label, reason)
	} else {
		reason = fmt.Sprintf("Task failed: %s", reason)
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		attrs := []slog.Attr{
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("task_id", entry.id),
			slog.String("task_type", entry.taskType),
			slog.String("error", entry.toolError),
		}
		if len(entry.metadata) > 0 {
			attrs = append(attrs, slog.Any("tool_metadata", entry.metadata))
		}
		logger.LogAttrs(m.ctx, slog.LevelError, "kai task failed", attrs...)
	}
	entry.status = "failed"
	m.addTaskUpdate(entry, reason, true)
}

func formatTaskContextLines(contexts []kai.ChatContext) []string {
	if len(contexts) == 0 {
		return nil
	}
	collector := newLineCollector()
	for _, ctx := range contexts {
		label := humanizeLabel(string(ctx.Type))
		value := ctx.ID
		if trimmed := trimIdentifier(ctx.ID); trimmed != ctx.ID {
			value = trimmed
		}
		collector.add(fmt.Sprintf("• %s: %s", label, value))
	}
	return collector.lines
}

func formatTaskParameterLines(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	params, ok := metadata["parameters"].(map[string]any)
	if !ok || len(params) == 0 {
		return nil
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	collector := newLineCollector()
	for _, key := range keys {
		norm := strings.ToLower(key)
		if strings.Contains(norm, "control_plane") {
			continue
		}
		value := formatParameterValue(params[key])
		collector.add(fmt.Sprintf("• %s: %s", humanizeLabel(key), value))
	}
	return collector.lines
}

func humanizeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	parts := strings.FieldsFunc(label, func(r rune) bool { return r == '_' || r == '-' })
	for i := range parts {
		if len(parts[i]) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func formatParameterValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	case int, int32, int64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, formatParameterValue(item))
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		segments := make([]string, 0, len(keys))
		for _, k := range keys {
			segments = append(segments, fmt.Sprintf("%s=%s", humanizeLabel(k), formatParameterValue(v[k])))
		}
		return strings.Join(segments, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func extractToolError(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if errVal, ok := metadata["error"]; ok {
		return fmt.Sprint(errVal)
	}
	if resp, ok := metadata["tool_response"]; ok {
		switch r := resp.(type) {
		case map[string]any:
			if msg, ok := r["error"]; ok {
				return fmt.Sprint(msg)
			}
			if msg, ok := r["message"]; ok {
				return fmt.Sprint(msg)
			}
		case string:
			s := strings.TrimSpace(r)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func (m *model) renderUserLine(content string) string {
	line := fmt.Sprintf("› %s", content)
	if m.opts.UseColor {
		return m.styles.questionStyle.Render(line)
	}
	return line
}

func (m *model) renderTaskCard(entry *taskEntry) string {
	if entry == nil {
		return ""
	}

	lowerStatus := strings.ToLower(strings.TrimSpace(entry.status))
	taskName := taskDisplayLabel(entry)
	headerLabel := fmt.Sprintf("Kai task: %s", taskName)
	if lowerStatus == "pending" {
		headerLabel = fmt.Sprintf("Kai task request: %s", taskName)
	}

	question := strings.TrimSpace(entry.confirmation)
	lines := []string{}
	if lowerStatus == "pending" && question != "" {
		lines = append(lines, question)
	} else if entry.statusNote != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", entry.statusNote))
	}

	details := append([]string{}, formatTaskContextLines(entry.context)...)
	params := formatTaskParameterLines(entry.metadata)
	if len(params) > 0 {
		details = append(details, params...)
	}
	if len(details) > maxTaskDetailLines {
		details = details[:maxTaskDetailLines]
	}
	if len(details) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, details...)
	}

	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("Status: %s", statusNoteFor(lowerStatus)))
	}

	icon := "❓"
	iconColor := m.styles.taskBorder
	switch lowerStatus {
	case "pending":
		icon = "❓"
		iconColor = m.styles.taskBorder
	case "in_progress":
		icon = "⏳"
		iconColor = m.styles.promptThinking
	case "analyzable", "analyzing":
		icon = "🧠"
		iconColor = m.styles.promptAccent
	case "done":
		icon = "✅"
		iconColor = m.styles.promptSuccess
	case "cancelled", "stopped":
		icon = "⛔"
		iconColor = m.styles.promptPlaceholder
	case "failed", "error":
		icon = "❌"
		iconColor = m.styles.promptError
	}
	if entry.toolError != "" && lowerStatus == "" {
		icon = "❌"
		iconColor = m.styles.promptError
	}

	displayLines := make([]string, len(lines))
	firstContentIdx := -1
	for i, line := range lines {
		trim := strings.TrimRight(line, " ")
		displayLines[i] = trim
		if trim != "" && firstContentIdx == -1 {
			firstContentIdx = i
		}
	}
	if firstContentIdx == -1 {
		firstContentIdx = 0
	}

	iconPlain := icon + " "
	iconStyled := iconPlain
	if m.opts.UseColor {
		iconStyled = lipgloss.NewStyle().Foreground(iconColor).Render(icon) + " "
	}

	maxWidth := 0
	for i, line := range displayLines {
		content := line
		if i == firstContentIdx {
			content = iconPlain + line
		}
		if w := lipgloss.Width(content); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth < 32 {
		maxWidth = 32
	}

	padded := make([]string, len(displayLines))
	for i, line := range displayLines {
		content := line
		if i == firstContentIdx {
			content = iconStyled + line
		}
		pad := maxWidth - lipgloss.Width(content)
		if pad < 0 {
			pad = 0
		}
		if line == "" {
			padded[i] = ""
			continue
		}
		text := content + strings.Repeat(" ", pad)
		if m.opts.UseColor {
			padded[i] = m.styles.promptHelperStyle.Render(text)
		} else {
			padded[i] = text
		}
	}

	cardContent := strings.Join(padded, "\n")
	var card string
	if !m.opts.UseColor {
		border := strings.Repeat("─", maxWidth)
		card = fmt.Sprintf("┌%s┐\n│%s│\n└%s┘", border, cardContent, border)
		if headerLabel != "" {
			return headerLabel + "\n" + card
		}
		return card
	}

	card = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.styles.taskBorder).
		Padding(0, 1).
		Render(cardContent)
	if headerLabel != "" {
		return m.renderTaskHeader(headerLabel) + "\n" + card
	}
	return card
}

func (m *model) renderTaskFallback(content string) string {
	id := strings.TrimSpace(content)
	if id == "" {
		return ""
	}
	if entry, ok := m.tasks[id]; ok {
		return m.renderTaskCard(entry)
	}
	return id
}

func taskDisplayLabel(entry *taskEntry) string {
	if entry == nil {
		return ""
	}
	typ := strings.TrimSpace(entry.taskType)
	if typ == "" {
		typ = "Task"
	} else {
		typ = humanizeLabel(typ)
	}
	id := strings.TrimSpace(entry.id)
	suffix := ""
	if len(id) >= 3 {
		suffix = strings.ToLower(id[len(id)-3:])
	}
	if suffix != "" {
		return fmt.Sprintf("%s (%s)", typ, suffix)
	}
	return typ
}

func statusDisplayLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "analyzing":
		return "Under Analysis"
	case "analysis_complete":
		return "Analysis Complete"
	case "analysis_error":
		return "Analysis Error"
	default:
		return humanizeLabel(status)
	}
}

func friendlyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return "Task pending approval"
	case "in_progress":
		return "Task in progress"
	case "analyzable":
		return "Task awaiting analysis"
	case "analyzing":
		return "Task under analysis"
	case "analysis_complete":
		return "Task analysis complete"
	case "analysis_error":
		return "Task analysis failed"
	case "done":
		return "Task completed"
	case "cancelled":
		return "Task cancelled"
	case "stopped":
		return "Task stopped"
	case "failed", "error":
		return "Task failed"
	default:
		s := strings.TrimSpace(status)
		if s == "" {
			return "Task update"
		}
		return humanizeLabel(s)
	}
}

func statusNoteFor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return "Pending approval"
	case "in_progress":
		return "Approved"
	case "analyzable":
		return "Awaiting analysis"
	case "analyzing":
		return "Under analysis"
	case "analysis_complete":
		return "Analysis complete"
	case "analysis_error":
		return "Analysis failed"
	case "done":
		return "Completed"
	case "cancelled":
		return "Cancelled"
	case "stopped":
		return "Stopped"
	case "failed", "error":
		return "Failed"
	default:
		s := strings.TrimSpace(status)
		if s == "" {
			return ""
		}
		return humanizeLabel(s)
	}
}

func splitTaskAnalysisContent(content string) (string, string, bool) {
	if !strings.HasPrefix(content, taskAnalysisMarker) {
		return "", content, false
	}
	rest := strings.TrimPrefix(content, taskAnalysisMarker)
	parts := strings.SplitN(rest, "\n", 2)
	label := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) > 1 {
		body = parts[1]
	}
	return label, body, true
}

func (m *model) appendTaskAnalysis(entry *taskEntry, text string) {
	if entry == nil {
		return
	}
	label := taskDisplayLabel(entry)
	payload := taskAnalysisMarker + label + "\n" + text
	entry.statusNote = "Analysis complete"
	m.messages = append(m.messages, message{sender: agentSpeaker, content: payload, duration: 0})
}

func (m *model) ensureRecorder(sessionID, sessionName string, createdAt time.Time) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	if m.recorder != nil {
		if m.recorder.SessionID() == sessionID {
			if err := m.recorder.SetSessionInfo(sessionName, createdAt); err != nil {
				m.logStorageError("set_session_info", err)
			}
			return false
		}
	}
	recorder, err := storage.NewSessionRecorder(sessionID, storage.Options{
		SessionName:    sessionName,
		SessionCreated: createdAt,
		CLIVersion:     m.opts.Version,
	})
	if err != nil {
		m.logStorageError("init_session_storage", err)
		return false
	}
	m.recorder = recorder
	if err := recorder.SetSessionInfo(sessionName, createdAt); err != nil {
		m.logStorageError("set_session_info", err)
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelDebug, "kai session storage initialized",
			slog.String("session_id", sessionID),
			slog.String("directory", recorder.Directory()))
	}
	return true
}

func (m *model) logStorageError(operation string, err error) {
	if err == nil {
		return
	}
	if logger := kai.ContextLogger(m.ctx); logger != nil {
		logger.LogAttrs(m.ctx, slog.LevelWarn, "kai session storage error",
			slog.String("session_id", strings.TrimSpace(m.sessionID)),
			slog.String("operation", strings.TrimSpace(operation)),
			slog.String("error", err.Error()))
	}
}

func (m *model) recordLifecycleEvent(kind string, metadata map[string]any) {
	if m.recorder == nil {
		return
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return
	}
	payload := map[string]any{"event": kind}
	for k, v := range metadata {
		if strings.TrimSpace(k) == "" || v == nil {
			continue
		}
		payload[k] = v
	}
	if err := m.recorder.AppendEvent(storage.Event{
		Kind:     storage.EventKindLifecycle,
		Metadata: payload,
	}); err != nil {
		m.logStorageError("record_lifecycle", err)
	}
}

func (m *model) recordUserMessage(text string, contexts []kai.ChatContext) {
	if m.recorder == nil {
		return
	}
	evt := storage.Event{
		Kind:    storage.EventKindMessage,
		Role:    "user",
		Content: text,
		Context: convertContexts(contexts),
	}
	if err := m.recorder.AppendEvent(evt); err != nil {
		m.logStorageError("record_user_message", err)
	}
}

func (m *model) recordAgentResponse(text string, duration time.Duration) {
	if m.recorder == nil {
		return
	}
	evt := storage.Event{
		Kind:    storage.EventKindMessage,
		Role:    "agent",
		Content: text,
	}
	if duration > 0 {
		evt.Duration = duration
	}
	if err := m.recorder.AppendEvent(evt); err != nil {
		m.logStorageError("record_agent_response", err)
	}
}

func (m *model) recordAgentError(message string) {
	if m.recorder == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	if err := m.recorder.RecordError(message, map[string]any{"source": "agent"}); err != nil {
		m.logStorageError("record_agent_error", err)
	}
}

func (m *model) recordStreamError(err error) {
	if m.recorder == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if recordErr := m.recorder.RecordError(err.Error(), map[string]any{"source": "stream"}); recordErr != nil {
		m.logStorageError("record_stream_error", recordErr)
	}
}

func (m *model) recordTaskProposal(entry *taskEntry, status, origin string) {
	if m.recorder == nil || entry == nil {
		return
	}
	meta := map[string]any{}
	if strings.TrimSpace(origin) != "" {
		meta["origin"] = origin
	}
	if strings.TrimSpace(entry.toolError) != "" {
		meta["tool_error"] = entry.toolError
	}
	if strings.TrimSpace(entry.statusNote) != "" {
		meta["status_note"] = entry.statusNote
	}
	analysisReady := strings.EqualFold(status, "analyzable")
	m.appendTaskEvent(entry, storage.EventKindTask, status, "", "", meta, analysisReady)
}

func (m *model) recordTaskAction(entry *taskEntry, action kai.TaskAction, actionErr error) {
	if m.recorder == nil {
		return
	}
	meta := map[string]any{
		"action": string(action),
	}
	if actionErr != nil {
		meta["error"] = actionErr.Error()
	}
	status := ""
	if entry != nil {
		status = entry.status
	}
	m.appendTaskEvent(entry, storage.EventKindTask, status, "", string(action), meta, false)
}

func (m *model) recordTaskStatus(entry *taskEntry, status, displayMsg string, finished bool, rawMessage string) {
	if m.recorder == nil {
		return
	}
	meta := map[string]any{}
	if finished {
		meta["finished"] = true
	}
	if trimmed := strings.TrimSpace(displayMsg); trimmed != "" {
		meta["display_message"] = trimmed
	}
	trimmedRaw := strings.TrimSpace(rawMessage)
	message := trimmedRaw
	if message == "" {
		message = strings.TrimSpace(displayMsg)
	}
	if trimmedRaw != "" && trimmedRaw != strings.TrimSpace(displayMsg) {
		meta["raw_message"] = trimmedRaw
	}
	analysisReady := strings.EqualFold(status, "analyzable")
	m.appendTaskEvent(entry, storage.EventKindTaskState, status, message, "", meta, analysisReady)
}

func (m *model) appendTaskEvent(
	entry *taskEntry,
	kind storage.EventKind,
	status string,
	message string,
	action string,
	metadata map[string]any,
	analysisReady bool,
) {
	if m.recorder == nil {
		return
	}
	evt := storage.Event{Kind: kind}
	if len(metadata) > 0 {
		evt.Metadata = make(map[string]any, len(metadata))
		for k, v := range metadata {
			if strings.TrimSpace(k) == "" || v == nil {
				continue
			}
			evt.Metadata[k] = v
		}
	}
	trimmedStatus := strings.TrimSpace(status)
	trimmedAction := strings.TrimSpace(action)
	trimmedMessage := strings.TrimSpace(message)
	task := &storage.TaskEvent{
		Status: trimmedStatus,
	}
	if trimmedAction != "" {
		task.Action = trimmedAction
	}
	if trimmedMessage != "" {
		task.Message = trimmedMessage
	}
	if entry != nil {
		task.ID = entry.id
		task.Type = entry.taskType
		task.Confirmation = entry.confirmation
		task.Metadata = cloneMetadataMap(entry.metadata)
		task.Error = entry.toolError
		task.Context = convertContexts(entry.context)
	}
	if analysisReady {
		task.AnalysisReady = true
	}
	hasTaskData := task.ID != "" ||
		task.Type != "" ||
		task.Status != "" ||
		task.Message != "" ||
		task.Action != "" ||
		task.AnalysisReady
	hasMetadata := len(task.Metadata) > 0
	hasContext := len(task.Context) > 0
	hasOther := strings.TrimSpace(task.Confirmation) != "" || strings.TrimSpace(task.Error) != ""

	if !hasTaskData && !hasMetadata && !hasContext && !hasOther {
		task = nil
	}
	if task != nil {
		evt.Task = task
	} else {
		if evt.Metadata == nil {
			evt.Metadata = map[string]any{}
		}
		if trimmedStatus != "" {
			evt.Metadata["status"] = trimmedStatus
		}
		if trimmedAction != "" {
			evt.Metadata["action"] = trimmedAction
		}
		if trimmedMessage != "" {
			evt.Metadata["message"] = trimmedMessage
		}
	}
	if evt.Metadata == nil {
		evt.Metadata = map[string]any{}
	}
	if err := m.recorder.AppendEvent(evt); err != nil {
		m.logStorageError("record_task_event", err)
	}
}

func convertContexts(contexts []kai.ChatContext) []storage.ContextRef {
	if len(contexts) == 0 {
		return nil
	}
	result := make([]storage.ContextRef, 0, len(contexts))
	for _, ctx := range contexts {
		typ := strings.TrimSpace(string(ctx.Type))
		id := strings.TrimSpace(ctx.ID)
		if typ == "" || id == "" {
			continue
		}
		result = append(result, storage.ContextRef{Type: typ, ID: id})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneMetadataMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	clone := make(map[string]any, len(input))
	for k, v := range input {
		clone[k] = v
	}
	return clone
}

type lineCollector struct {
	lines []string
	seen  map[string]struct{}
}

func newLineCollector() *lineCollector {
	return &lineCollector{seen: make(map[string]struct{})}
}

func (c *lineCollector) add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	norm := strings.ToLower(line)
	if _, ok := c.seen[norm]; ok {
		return
	}
	c.seen[norm] = struct{}{}
	c.lines = append(c.lines, line)
}

func (c *lineCollector) String() string {
	return strings.Join(c.lines, "\n")
}

func (m *model) ingestContext(item kai.SessionHistoryItem) {
	if len(item.Context) == 0 {
		return
	}
	for _, raw := range item.Context {
		typ, _ := raw["type"].(string)
		id, _ := raw["id"].(string)
		if typ == "" || id == "" {
			continue
		}
		name := ""
		if n, ok := raw["name"].(string); ok {
			name = strings.TrimSpace(n)
		}
		ctxEntry := contextEntry{
			entity: kai.ChatContext{Type: kai.ContextType(typ), ID: id},
			name:   name,
		}
		m.upsertContext(ctxEntry)
	}
}

func (m *model) ingestTaskContexts(contexts []kai.ChatContext) {
	for _, ctx := range contexts {
		m.upsertContext(contextEntry{entity: ctx})
	}
}

func trimIdentifier(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func (m *model) applyInitialTasks(tasks []kai.TaskDetails) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		if cmd := m.handleTaskProposal(&t, "initial_load"); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *model) upsertTask(details *kai.TaskDetails) *taskEntry {
	if details == nil {
		return nil
	}
	entry, ok := m.tasks[details.ID]
	if !ok {
		entry = &taskEntry{id: details.ID}
		m.tasks[details.ID] = entry
	}
	entry.taskType = details.Type
	entry.confirmation = strings.TrimSpace(details.ConfirmationMessage)
	entry.context = details.Context
	entry.status = strings.ToLower(details.Status)
	entry.metadata = details.ToolCallMetadata
	entry.toolError = extractToolError(details.ToolCallMetadata)
	m.ingestTaskContexts(details.Context)
	return entry
}

func (m *model) loadActiveTasksCmd() tea.Cmd {
	if strings.TrimSpace(m.sessionID) == "" {
		return nil
	}
	baseURL := m.opts.BaseURL
	token := m.opts.Token
	return func() tea.Msg {
		tasks, err := kai.ListActiveTasks(m.ctx, nil, baseURL, token, m.sessionID)
		return activeTasksLoadedMsg{tasks: tasks, err: err}
	}
}

func (m *model) upsertContext(entry contextEntry) {
	replaced := false
	for i, existing := range m.contextEntries {
		if existing.entity.Type == entry.entity.Type {
			m.contextEntries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		m.contextEntries = append(m.contextEntries, entry)
	}
}

func (m *model) contextsForRequest() []kai.ChatContext {
	if len(m.contextEntries) == 0 {
		return nil
	}
	contexts := make([]kai.ChatContext, len(m.contextEntries))
	for i, entry := range m.contextEntries {
		contexts[i] = entry.entity
	}
	return contexts
}

func (m *model) renderSuggestions() string {
	if len(m.filteredCommands) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, cmd := range m.filteredCommands {
		b.WriteString("  ")
		b.WriteString(cmd.name)
		if cmd.description != "" {
			b.WriteString(" — ")
			b.WriteString(cmd.description)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) commandMatches(raw string) []slashCommand {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	commandPart := trimmed
	if idx := strings.IndexRune(commandPart, ' '); idx >= 0 {
		commandPart = commandPart[:idx]
	}
	commandPart = strings.ToLower(commandPart)

	matches := make([]slashCommand, 0, len(m.commandList))
	for _, cmd := range m.commandList {
		if strings.HasPrefix(cmd.name, commandPart) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

func (m *model) updateSuggestions() {
	matches := m.commandMatches(m.input.Value())
	if len(matches) == 0 {
		m.filteredCommands = nil
		return
	}
	if strings.ContainsRune(m.input.Value(), ' ') {
		m.filteredCommands = nil
		return
	}
	m.filteredCommands = matches
}

func (m *model) completeSuggestion() bool {
	matches := m.commandMatches(m.input.Value())
	if len(matches) == 0 {
		return false
	}
	suggestion := matches[0].name + " "
	m.input.SetValue(suggestion)
	m.input.CursorEnd()
	m.updateSuggestions()
	m.adjustInputHeight()
	return true
}

func (m *model) executeSlashCommand(raw string) (tea.Cmd, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil, false
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return nil, true
	}

	command := parts[0]
	args := strings.TrimSpace(strings.TrimPrefix(trimmed, command))

	switch command {
	case "/context-control-plane":
		if args == "" {
			m.contextLoading = false
			m.notifyErrorText("context-control-plane command requires a name")
			return nil, true
		}
		if m.lookupControlPlane == nil {
			m.contextLoading = false
			m.notifyErrorText("context-control-plane lookup unavailable")
			return nil, true
		}
		if m.contextLoading {
			return nil, true
		}
		m.contextLoading = true
		return resolveControlPlaneCmd(m.ctx, m.lookupControlPlane, args), true
	case "/context-clear":
		m.contextLoading = false
		m.contextEntries = nil
		return nil, true
	case "/clear":
		m.contextLoading = false
		m.messages = nil
		m.history = nil
		m.pendingBuilder.Reset()
		m.pendingPrompt = ""
		return nil, true
	case "/quit":
		m.contextLoading = false
		return tea.Quit, true
	case "/clear-task-status":
		m.contextLoading = false
		if m.taskStatusBar.visible {
			m.clearTaskStatusBar()
		} else {
			m.notifyKaiMessage("Task status bar is already hidden.")
		}
		return nil, true
	case "/task":
		sub := strings.ToLower(strings.TrimSpace(args))
		switch sub {
		case "cancel":
			cmd := m.cancelTask()
			return cmd, true
		case "analyze":
			cmd := m.startAnalyzeTask(m.pendingAnalysisTask)
			if cmd == nil {
				m.notifyKaiMessage("No task ready to analyze.")
			}
			return cmd, true
		case "":
			m.notifyKaiMessage("Usage: /task cancel | /task analyze")
			return nil, true
		default:
			m.notifyErrorText(fmt.Sprintf("unknown task command: %s", sub))
			return nil, true
		}
	default:
		m.contextLoading = false
		m.notifyErrorText(fmt.Sprintf("unknown command: %s", command))
		return nil, true
	}
}

func resolveControlPlaneCmd(
	ctx context.Context,
	lookup func(context.Context, string) (string, error),
	name string,
) tea.Cmd {
	return func() tea.Msg {
		logger := kai.ContextLogger(ctx)
		if logger != nil {
			logger.LogAttrs(ctx, slog.LevelInfo, "kai context resolve started",
				slog.String("resource", "control_plane"),
				slog.String("name", name))
		}
		id, err := lookup(ctx, name)
		if err != nil {
			if logger != nil {
				logger.LogAttrs(ctx, slog.LevelError, "kai context resolve failed",
					slog.String("resource", "control_plane"),
					slog.String("name", name),
					slog.String("error", err.Error()))
			}
			return contextErrorMsg{err: err}
		}
		if logger != nil {
			logger.LogAttrs(ctx, slog.LevelInfo, "kai context resolved",
				slog.String("resource", "control_plane"),
				slog.String("name", name),
				slog.String("id", id))
		}
		return contextAddedMsg{
			entry: contextEntry{
				entity: kai.ChatContext{Type: kai.ContextTypeControlPlane, ID: id},
				name:   name,
			},
		}
	}
}

func (m *model) cleanupAfterStream() {
	m.stopStream()
	m.pendingBuilder.Reset()
	m.pendingPrompt = ""
	m.pendingContexts = nil
	m.streamErr = nil
	m.responseStart = time.Time{}
}

func (m *model) stopStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.stream = nil
	m.streaming = false
}

func (m *model) createSessionCmd() tea.Cmd {
	return func() tea.Msg {
		name := ""
		if m.opts.SessionNameFactory != nil {
			name = strings.TrimSpace(m.opts.SessionNameFactory())
		}
		if name == "" {
			name = fmt.Sprintf("kai-%s-%s", time.Now().Format("20060102-150405"), uuid.NewString()[:8])
		}

		meta, err := kai.CreateSession(m.ctx, nil, m.opts.BaseURL, m.opts.Token, name)
		if err != nil {
			return sessionErrorMsg{err: err}
		}
		if meta.Name == "" {
			meta.Name = name
		}
		return sessionCreatedMsg{id: meta.ID, name: meta.Name, createdAt: meta.CreatedAt}
	}
}

func startStreamCmd(ctx context.Context, opts Options, sessionID, prompt string, contexts []kai.ChatContext) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithCancel(ctx)
		payload := append([]kai.ChatContext(nil), contexts...)
		if len(payload) == 0 {
			payload = nil
		}
		stream, err := kai.ChatStreamSession(reqCtx, nil, opts.BaseURL, opts.Token, sessionID, prompt, payload)
		if err != nil {
			cancel()
			return chatStreamErrorMsg{err: err}
		}
		return chatStreamStartedMsg{stream: stream, cancel: cancel}
	}
}

func recoverChatHistoryCmd(
	ctx context.Context,
	opts Options,
	sessionID, prompt string,
	contexts []kai.ChatContext,
) tea.Cmd {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	baseURL := opts.BaseURL
	token := opts.Token
	cloned := append([]kai.ChatContext(nil), contexts...)
	return func() tea.Msg {
		history, err := kai.GetSessionHistory(ctx, nil, baseURL, token, sessionID)
		if err != nil {
			return chatRecoveryHistoryMsg{
				prompt:   prompt,
				contexts: cloned,
				err:      err,
			}
		}
		return chatRecoveryHistoryMsg{
			history:  history,
			prompt:   prompt,
			contexts: cloned,
		}
	}
}

func waitForEvent(stream *kai.Stream) tea.Cmd {
	if stream == nil {
		return nil
	}

	return func() tea.Msg {
		evt, ok := <-stream.Events
		if !ok {
			if err := stream.Err(); err != nil {
				return chatStreamErrorMsg{err: err}
			}
			return chatStreamFinishedMsg{}
		}
		return chatEventMsg{event: evt}
	}
}

func buildPrompt(_ []exchange, next string) string {
	// When sessions are used, context is handled server-side, so only send the latest prompt.
	return next
}

func eventText(evt kai.ChatEvent) (string, bool) {
	switch v := evt.Data.(type) {
	case map[string]any:
		if inner, ok := v["data"]; ok {
			if str, ok := inner.(string); ok {
				return str, true
			}
		}
	case string:
		return v, true
	}

	return "", false
}

func eventError(evt kai.ChatEvent) string {
	switch v := evt.Data.(type) {
	case map[string]any:
		if msg, ok := v["error"]; ok {
			return fmt.Sprint(msg)
		}
		if msg, ok := v["message"]; ok {
			return fmt.Sprint(msg)
		}
	case string:
		return v
	}
	return fmt.Sprint(evt.Data)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lower, upper int) int {
	if v < lower {
		return lower
	}
	if v > upper {
		return upper
	}
	return v
}

func promptContentWidth(view string) int {
	if view == "" {
		return 1
	}
	lines := strings.Split(view, "\n")
	maxWidth := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth <= 0 {
		return 1
	}
	return maxWidth
}

func padPromptLines(view string, width int) string {
	if width <= 0 {
		width = 1
	}
	if view == "" {
		return strings.Repeat(" ", width)
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		diff := width - lipgloss.Width(line)
		if diff > 0 {
			lines[i] = line + strings.Repeat(" ", diff)
		}
	}
	return strings.Join(lines, "\n")
}

func isTaskFinishedStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "done", "cancelled", "stopped", "failed", "error":
		return true
	default:
		return false
	}
}
