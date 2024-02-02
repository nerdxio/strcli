package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
	"os"
	"strings"
)

const (
	initialInputs = 3
	resultHeight  = 5
	helpHeight    = 5
)

var (
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	endOfBufferStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235"))

	focusedPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder())
)

type keymap = struct {
	next, prev, quit, compare key.Binding
}

func newTextarea() textarea.Model {
	t := textarea.New()
	t.Prompt = ""
	t.Placeholder = "Type something"
	t.ShowLineNumbers = true
	t.Cursor.Style = cursorStyle
	t.FocusedStyle.Placeholder = focusedPlaceholderStyle
	t.BlurredStyle.Placeholder = placeholderStyle
	t.FocusedStyle.CursorLine = cursorLineStyle
	t.FocusedStyle.Base = focusedBorderStyle
	t.BlurredStyle.Base = blurredBorderStyle
	t.FocusedStyle.EndOfBuffer = endOfBufferStyle
	t.BlurredStyle.EndOfBuffer = endOfBufferStyle
	t.KeyMap.DeleteWordBackward.SetEnabled(false)
	t.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))
	t.KeyMap.LinePrevious = key.NewBinding(key.WithKeys("up"))
	t.Blur()
	return t
}

type model struct {
	width  int
	height int
	keymap keymap
	help   help.Model
	inputs []textarea.Model
	focus  int
	diff   string
}

func newModel() model {
	m := model{
		inputs: make([]textarea.Model, initialInputs),
		help:   help.New(),
		keymap: keymap{
			next: key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "next"),
			),
			prev: key.NewBinding(
				key.WithKeys("shift+tab"),
				key.WithHelp("shift+tab", "prev"),
			),
			quit: key.NewBinding(
				key.WithKeys("esc", "ctrl+c"),
				key.WithHelp("esc", "quit"),
			),
			compare: key.NewBinding(
				key.WithKeys("ctrl+r"),
				key.WithHelp("ctrl+r", "compare"),
			),
		},
	}
	for i := 0; i < initialInputs-1; i++ { // Only create editable textareas for the first two
		m.inputs[i] = newTextarea()
	}
	m.inputs[m.focus].Focus()

	// Create a new textarea for the result
	t := newTextarea()
	m.inputs[initialInputs-1] = t // Add it to the inputs

	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.quit):
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			return m, tea.Quit

		case key.Matches(msg, m.keymap.next):
			m.inputs[m.focus].Blur()
			m.focus++
			if m.focus > len(m.inputs)-1 {
				m.focus = 0
			}
			cmd := m.inputs[m.focus].Focus()
			cmds = append(cmds, cmd)

		case key.Matches(msg, m.keymap.prev):
			m.inputs[m.focus].Blur()
			m.focus--
			if m.focus < 0 {
				m.focus = len(m.inputs) - 1
			}
			cmd := m.inputs[m.focus].Focus()
			cmds = append(cmds, cmd)

		case key.Matches(msg, m.keymap.compare):
			// Get the text from the two textareas
			text1 := m.inputs[0].Value()
			text2 := m.inputs[1].Value()

			// Use diffmatchpatch to compare the texts
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(text1, text2, false)

			// Colorize the diffs
			coloredDiff := colorizeDiffs(diffs)

			// Set the colored diff in the third textarea
			m.inputs[2].SetValue(coloredDiff)

			// Update m.diff
			m.diff = coloredDiff
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	}

	m.sizeInputs()

	// Update all textareas
	for i := range m.inputs {
		newModel, cmd := m.inputs[i].Update(msg)
		m.inputs[i] = newModel
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) sizeInputs() {
	for i := 0; i < len(m.inputs)-1; i++ { // Only size the first two textareas
		m.inputs[i].SetWidth(m.width / (len(m.inputs) - 1))
		m.inputs[i].SetHeight((m.height - helpHeight - resultHeight) / 2)
	}

	// Size the result textarea
	m.inputs[len(m.inputs)-1].SetWidth(m.width)
	m.inputs[len(m.inputs)-1].SetHeight(resultHeight)
}

func (m model) View() string {
	help := m.help.ShortHelpView([]key.Binding{
		m.keymap.next,
		m.keymap.prev,
		m.keymap.quit,
		m.keymap.compare,
	})

	var views []string
	for i := 0; i < len(m.inputs)-1; i++ { // Only join the first two textareas horizontally
		views = append(views, m.inputs[i].View())
	}

	// Wrap the diff result to the terminal width
	diff := wrapText(m.diff, m.width)

	return lipgloss.JoinHorizontal(lipgloss.Top, views...) + "\n" + m.inputs[len(m.inputs)-1].View() + "\n" + " " + help + "\n\n" + diff
}

func colorizeDiffs(diffs []diffmatchpatch.Diff) string {
	var coloredDiff string
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			// Green for insertions
			coloredDiff += lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render(diff.Text)
		case diffmatchpatch.DiffDelete:
			// Red for deletions
			coloredDiff += lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(diff.Text)
		case diffmatchpatch.DiffEqual:
			coloredDiff += diff.Text
		}
		coloredDiff += "\n"
	}
	return coloredDiff
}

// Wrap text to terminal width
func wrapText(input string, limit int) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return input
	}
	wrapped := words[0]
	remain := limit - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > remain {
			wrapped += "\n" + word
			remain = limit - len(word)
		} else {
			wrapped += " " + word
			remain -= len(word) + 1
		}
	}
	return wrapped
}
func main() {
	if _, err := tea.NewProgram(newModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error while running program:", err)
		os.Exit(1)
	}
	//dmp := diffmatchpatch.New()
	//
	//str1 := "Hello"
	//str2 := "Hello Go bro "
	//
	//diffs := dmp.DiffMain(str1, str2, false)
	//fmt.Println(diffs)
	//
	//fmt.Println(dmp.DiffPrettyText(diffs))
}
