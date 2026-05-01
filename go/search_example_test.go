// SPDX-Licence-Identifier: EUPL-1.2
package session

func ExampleSearch() {
	_ = Search("/tmp/claude-projects", "go test")
}

func ExampleSearchSeq() {
	for item := range SearchSeq("/tmp/claude-projects", "go test") {
		_ = item
	}
}
