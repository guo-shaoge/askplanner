package admin

import (
	"fmt"
	"strings"
)

type Command struct {
	Action  string
	UserKey string
}

func ParseCommand(input string) (*Command, bool, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/admin") {
		return nil, false, nil
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(input, "/admin")))
	if len(fields) == 0 {
		return nil, true, fmt.Errorf("usage: /admin reset-user <user>")
	}

	cmd := &Command{Action: strings.ToLower(fields[0])}
	switch cmd.Action {
	case "reset-user":
		if len(fields) != 2 {
			return nil, true, fmt.Errorf("usage: /admin reset-user <user>")
		}
		cmd.UserKey = fields[1]
	default:
		return nil, true, fmt.Errorf("unsupported admin command: %s", cmd.Action)
	}
	return cmd, true, nil
}
