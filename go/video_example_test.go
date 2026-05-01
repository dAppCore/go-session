// SPDX-Licence-Identifier: EUPL-1.2
package session

func ExampleRenderMP4() {
	sess := &Session{ID: "example"}
	_ = RenderMP4(sess, "/tmp/example-session.mp4")
}
