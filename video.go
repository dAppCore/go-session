// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io/fs"
	"path"
	"syscall"

	core "dappco.re/go/core"
)

// RenderMP4 generates an MP4 video from session events using VHS (charmbracelet).
func RenderMP4(sess *Session, outputPath string) error {
	vhsPath := lookupExecutable("vhs")
	if vhsPath == "" {
		return core.E("RenderMP4", "vhs not installed (go install github.com/charmbracelet/vhs@latest)", nil)
	}

	tape := generateTape(sess, outputPath)

	tmpDir := hostFS.TempDir("session-")
	if tmpDir == "" {
		return core.E("RenderMP4", "create tape", core.NewError("failed to create temp dir"))
	}
	defer hostFS.DeleteAll(tmpDir)

	tapePath := path.Join(tmpDir, core.Concat(core.ID(), ".tape"))
	writeResult := hostFS.Write(tapePath, tape)
	if !writeResult.OK {
		return core.E("RenderMP4", "write tape", resultError(writeResult))
	}

	if err := runCommand(vhsPath, tapePath); err != nil {
		return core.E("RenderMP4", "vhs render", err)
	}

	return nil
}

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

func extractCommand(input string) string {
	// Remove description suffix (after " # ")
	if idx := indexOf(input, " # "); idx > 0 {
		return input[:idx]
	}
	return input
}

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
		candidate := path.Join(dir, name)
		if isExecutablePath(candidate) {
			return candidate
		}
	}
	return ""
}

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

func runCommand(command string, args ...string) error {
	argv := append([]string{command}, args...)
	procAttr := &syscall.ProcAttr{
		Env:   syscall.Environ(),
		Files: []uintptr{0, 1, 2},
	}

	pid, err := syscall.ForkExec(command, argv, procAttr)
	if err != nil {
		return err
	}

	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		return err
	}

	if status.Exited() && status.ExitStatus() == 0 {
		return nil
	}
	if status.Signaled() {
		return core.NewError(core.Sprintf("command terminated by signal %d", status.Signal()))
	}
	if status.Exited() {
		return core.NewError(core.Sprintf("command exited with status %d", status.ExitStatus()))
	}

	return core.NewError("command failed")
}
