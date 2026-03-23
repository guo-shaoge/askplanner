package workspace

import (
	"fmt"
	"strings"
)

type Command struct {
	Action   string
	Repo     string
	Ref      string
	Question string
}

func ParseCommand(input string) (*Command, bool, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/ws") {
		return nil, false, nil
	}

	left := input
	question := ""
	if idx := strings.Index(input, " -- "); idx >= 0 {
		left = strings.TrimSpace(input[:idx])
		question = strings.TrimSpace(input[idx+4:])
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(left, "/ws")))
	if len(fields) == 0 {
		return nil, true, fmt.Errorf("usage: /ws <status|switch|sync|reset>")
	}

	cmd := &Command{
		Action:   strings.ToLower(fields[0]),
		Question: question,
	}
	switch cmd.Action {
	case "status":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf("usage: /ws status")
		}
	case "switch":
		if len(fields) < 3 {
			return nil, true, fmt.Errorf("usage: /ws switch <repo> <ref> [-- question]")
		}
		cmd.Repo = fields[1]
		cmd.Ref = fields[2]
	case "sync", "reset":
		if len(fields) > 2 {
			return nil, true, fmt.Errorf("usage: /ws %s [repo|all]", cmd.Action)
		}
		if len(fields) == 2 {
			cmd.Repo = fields[1]
		} else {
			cmd.Repo = "all"
		}
	default:
		return nil, true, fmt.Errorf("unsupported workspace command: %s", cmd.Action)
	}
	return cmd, true, nil
}
