package selfcmd

import "strings"

func IsWhoAmI(input string) bool {
	return strings.EqualFold(strings.TrimSpace(input), "/whoami")
}
