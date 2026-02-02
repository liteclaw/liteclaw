package commands

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type channelSelectItem struct {
	key         string
	label       string
	description string
	checked     bool
}

type channelSelectModel struct {
	cursor int
	items  []channelSelectItem
	done   bool
	abort  bool
}

func (m channelSelectModel) Init() tea.Cmd {
	return nil
}

func (m channelSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.abort = true
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			m.items[m.cursor].checked = !m.items[m.cursor].checked
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m channelSelectModel) View() string {
	var b strings.Builder
	b.WriteString("Select channels (↑/↓ move, space toggle, enter confirm)\n\n")
	for i, item := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		check := " "
		if item.checked {
			check = "x"
		}
		line := fmt.Sprintf("%s [%s] %s", cursor, check, item.label)
		if item.description != "" {
			line += " — " + item.description
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nPress q/esc to cancel.\n")
	return b.String()
}

func runChannelMultiSelect(channels []onboardChannelEntry) ([]string, error) {
	items := make([]channelSelectItem, 0, len(channels))
	for _, ch := range channels {
		items = append(items, channelSelectItem{
			key:         ch.Key,
			label:       ch.Label,
			description: ch.Description,
		})
	}

	p := tea.NewProgram(channelSelectModel{items: items}, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	model, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := model.(channelSelectModel)
	if m.abort {
		return nil, fmt.Errorf("channel selection cancelled")
	}

	var selected []string
	for _, item := range m.items {
		if item.checked {
			selected = append(selected, item.key)
		}
	}
	return selected, nil
}
