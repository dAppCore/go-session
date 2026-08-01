// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io/fs" // Note: intrinsic — fs.FileInfo metadata for executable checks from hostFS.Stat; no core equivalent

	core "dappco.re/go"
)

// RenderMP4 generates an MP4 video from session events using VHS (charmbracelet).
//
// Example:
// result := session.RenderMP4(sess, "/tmp/session.mp4")
func RenderMP4(sess *Session, outputPath string) core.Result {
	vhsPath := lookupExecutable("vhs")
	if vhsPath == "" {
		return core.Fail(core.E("RenderMP4", "vhs not installed (go install github.com/charmbracelet/vhs@latest)", nil))
	}

	tape := generateTape(sess, outputPath)

	tmpDirResult := hostFS.TempDir("session-")
	if !tmpDirResult.OK {
		return core.Fail(core.E("RenderMP4", "failed to create temp dir", nil))
	}
	tmpDir := tmpDirResult.Value.(string)
	defer hostFS.DeleteAll(tmpDir)

	tapePath := core.PathJoin(tmpDir, core.Concat(core.ID(), ".tape"))
	writeResult := hostFS.Write(tapePath, tape)
	if !writeResult.OK {
		return core.Fail(core.E("RenderMP4", "write tape", resultError(writeResult)))
	}

	runResult := runCommand(vhsPath, tapePath)
	if !runResult.OK {
		return core.Fail(core.E("RenderMP4", "vhs render", resultError(runResult)))
	}

	return core.Ok(nil)
}

// generateTape builds the VHS script used to render a session video.
func generateTape(sess *Session, outputPath string) string {
	b := core.NewBuilder()

	b.WriteString(core.Sprintf("Output %s\n", outputPath))
	b.WriteString("Set FontSize 16\n")
	b.WriteString("Set Width 1400\n")
	b.WriteString("Set Height 800\n")
	b.WriteString("Set TypingSpeed 30ms\n")
	b.WriteString("Set Theme \"Catppuccin Mocha\"\n")
	b.WriteString("Set Shell bash\n")
	b.WriteString("\n")

	// Title frame
	id := sess.ID
	if len(id) > 8 {
		id = id[:8]
	}
	b.WriteString(core.Sprintf("Type \"# Session %s | %s\"\n",
		id, sess.StartTime.Format("2006-01-02 15:04")))
	b.WriteString("Enter\n")
	b.WriteString("Sleep 2s\n")
	b.WriteString("\n")

	for _, evt := range sess.Events {
		if evt.Type != "tool_use" {
			continue
		}

		switch evt.Tool {
		case "Bash":
			cmd := extractCommand(evt.Input)
			if cmd == "" {
				continue
			}
			// Show the command
			b.WriteString(core.Sprintf("Type %q\n", "$ "+cmd))
			b.WriteString("Enter\n")

			// Show abbreviated output
			output := evt.Output
			if len(output) > 200 {
				output = output[:200] + "..."
			}
			if output != "" {
				for _, line := range core.Split(output, "\n") {
					if line == "" {
						continue
					}
					b.WriteString(core.Sprintf("Type %q\n", line))
					b.WriteString("Enter\n")
				}
			}

			// Status indicator
			if !evt.Success {
				b.WriteString("Type \"# ✗ FAILED\"\n")
			} else {
				b.WriteString("Type \"# ✓ OK\"\n")
			}
			b.WriteString("Enter\n")
			b.WriteString("Sleep 1s\n")
			b.WriteString("\n")

		case "Read", "Edit", "Write":
			b.WriteString(core.Sprintf("Type %q\n",
				core.Sprintf("# %s: %s", evt.Tool, truncate(evt.Input, 80))))
			b.WriteString("Enter\n")
			b.WriteString("Sleep 500ms\n")

		case "Task":
			b.WriteString(core.Sprintf("Type %q\n",
				core.Sprintf("# Agent: %s", truncate(evt.Input, 80))))
			b.WriteString("Enter\n")
			b.WriteString("Sleep 1s\n")
		}
	}

	b.WriteString("Sleep 3s\n")
	return b.String()
}

// extractCommand removes a human description suffix from a Bash tool input.
func extractCommand(input string) string {
	// Remove description suffix (after " # ")
	if idx := indexOf(input, " # "); idx > 0 {
		return input[:idx]
	}
	return input
}

// lookupExecutable resolves an executable name from PATH or validates a direct path.
func lookupExecutable(name string) string {
	if name == "" {
		return ""
	}
	if containsAny(name, `/\`) {
		if isExecutablePath(name) {
			return name
		}
		return ""
	}

	for _, dir := range core.Split(core.Env("PATH"), ":") {
		if dir == "" {
			dir = "."
		}
		candidate := core.PathJoin(dir, name)
		if isExecutablePath(candidate) {
			return candidate
		}
	}
	return ""
}

// isExecutablePath reports whether filePath is an executable regular file.
func isExecutablePath(filePath string) bool {
	statResult := hostFS.Stat(filePath)
	if !statResult.OK {
		return false
	}
	info, ok := statResult.Value.(fs.FileInfo)
	if !ok || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

// runCommand executes an external command through the core process abstraction.
func runCommand(command string, args ...string) core.Result {
	c := sessionCore(nil)
	runResult := hostProcess(c).Run(hostContext(c), command, args...)
	if runResult.OK {
		return core.Ok(nil)
	}
	return core.Fail(core.E("runCommand", "run command", resultError(runResult)))
}
