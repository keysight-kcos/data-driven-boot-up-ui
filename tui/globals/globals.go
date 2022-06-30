// For definitions that need to be accessed in multiple package levels.
// Msgs, enums, etc.
package globals

type SystemdUpdateMsg struct{}

type Screen int

const (
	TopLevel Screen = iota
	Systemd
	UnitInfoScreen
	Scripts
)

func (s Screen) String() string {
	switch s {
	case TopLevel:
		return "TopLevel"
	case Systemd:
		return "Systemd"
	case UnitInfoScreen:
		return "UnitInfoScreen"
	case Scripts:
		return "Scripts"
	}
	return "Unmapped enum value."
}

type SwitchScreenMsg Screen