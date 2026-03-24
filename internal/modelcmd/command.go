package modelcmd

import (
	"fmt"
	"strings"
)

type Command struct {
	Action   string
	Model    string
	Effort   string
	Question string
}

func ParseCommand(input string) (*Command, bool, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/model") {
		return nil, false, nil
	}

	left := input
	question := ""
	if idx := strings.Index(input, " -- "); idx >= 0 {
		left = strings.TrimSpace(input[:idx])
		question = strings.TrimSpace(input[idx+4:])
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(left, "/model")))
	if len(fields) == 0 {
		return &Command{Action: "status", Question: question}, true, nil
	}

	cmd := &Command{
		Action:   strings.ToLower(fields[0]),
		Question: question,
	}
	switch cmd.Action {
	case "status":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf("usage: /model [status]")
		}
	case "set":
		if len(fields) != 2 {
			return nil, true, fmt.Errorf("usage: /model set <model> [-- question]")
		}
		cmd.Model = fields[1]
	case "reset":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf("usage: /model reset [-- question]")
		}
	case "effort":
		if len(fields) != 2 {
			return nil, true, fmt.Errorf("usage: /model effort <level|reset> [-- question]")
		}
		if strings.EqualFold(fields[1], "reset") {
			cmd.Action = "reset-effort"
		} else {
			cmd.Effort = fields[1]
		}
	default:
		if len(fields) == 1 {
			cmd.Action = "set"
			cmd.Model = fields[0]
			return cmd, true, nil
		}
		return nil, true, fmt.Errorf("usage: /model [status|set <model>|effort <level>|reset]")
	}
	return cmd, true, nil
}
