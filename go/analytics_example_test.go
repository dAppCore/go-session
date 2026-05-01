// SPDX-Licence-Identifier: EUPL-1.2
package session

import "time"

func ExampleAnalyse() {
	sess := &Session{Events: []Event{{Type: "tool_use", Tool: "Bash", Duration: time.Second, Success: true}}}
	_ = Analyse(sess)
}

func ExampleFormatAnalytics() {
	analytics := &SessionAnalytics{ToolCounts: map[string]int{"Bash": 1}}
	_ = FormatAnalytics(analytics)
}
