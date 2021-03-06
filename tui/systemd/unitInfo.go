// model for the UnitInfo page. More information for a specified unit.
package systemd

import (
	"fmt"
	"sort"
	g "spirit-box/tui/globals"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type unitInfo struct {
	name     string
	viewport viewport.Model
}

func InitUnitInfo(properties map[string]interface{}, width, height int) unitInfo {
	u := unitInfo{}

	keys := make([]string, len(properties))
	i := 0
	for k := range properties {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	var b strings.Builder
	for _, key := range keys {
		v := properties[key]
		fmt.Fprintf(&b, "%s: %v\n", key, v)
	}

	u.viewport = viewport.New(width, height)
	u.viewport.SetContent(b.String())
	return u
}

func (u unitInfo) Update(msg tea.Msg) (unitInfo, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return u, func() tea.Msg { return g.SwitchScreenMsg(g.Systemd) }
		}
	}

	var cmd tea.Cmd
	u.viewport, cmd = u.viewport.Update(msg)
	return u, cmd
}

func (u unitInfo) View() string {
	return u.viewport.View()
}
