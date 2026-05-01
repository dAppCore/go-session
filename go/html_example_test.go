// SPDX-Licence-Identifier: EUPL-1.2
package session

func ExampleRenderHTML() {
	sess := &Session{ID: "example"}
	_ = RenderHTML(sess, "/tmp/example-session.html")
}
