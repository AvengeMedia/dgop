package models

type KeyAction string

const (
	ActionQuit        KeyAction = "quit"
	ActionRefresh     KeyAction = "refresh"
	ActionDetails     KeyAction = "details"
	ActionKill        KeyAction = "kill"
	ActionSortCPU     KeyAction = "sortCPU"
	ActionSortMemory  KeyAction = "sortMemory"
	ActionSortName    KeyAction = "sortName"
	ActionSortPID     KeyAction = "sortPID"
	ActionGroup       KeyAction = "group"
	ActionNavUp       KeyAction = "navUp"
	ActionNavDown     KeyAction = "navDown"
	ActionSelectLeft  KeyAction = "selectLeft"
	ActionSelectRight KeyAction = "selectRight"
	ActionConfirm     KeyAction = "confirm"
	ActionCancel      KeyAction = "cancel"
)

type Keybinds map[KeyAction][]string

func DefaultKeybinds() Keybinds {
	return Keybinds{
		ActionQuit:        {"q", "ctrl+c"},
		ActionRefresh:     {"r"},
		ActionDetails:     {"d"},
		ActionKill:        {"x"},
		ActionSortCPU:     {"c"},
		ActionSortMemory:  {"m"},
		ActionSortName:    {"n"},
		ActionSortPID:     {"p"},
		ActionGroup:       {"g"},
		ActionNavUp:       {"up", "k"},
		ActionNavDown:     {"down", "j"},
		ActionSelectLeft:  {"left", "h"},
		ActionSelectRight: {"right", "l"},
		ActionConfirm:     {"enter"},
		ActionCancel:      {"esc", "escape"},
	}
}
