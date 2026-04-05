//go:build !darwin

package clinic

import "time"

func newChromeStatementPlanFetcher(time.Duration) StatementPlanFetcher {
	return nil
}
