package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const skillMarkdown = `---
name: easycat
description: >
  Read Android logcat through the easycat commands. Use when the user mentions
  logcat, ADB, an Android device, or runs /easycat.
user-invocable: true
---

# easycat

Use the easycat binary on PATH. Run the commands below. Leave the terminal UI alone.

adb must be on PATH. The phone or emulator must show up in adb devices -l.

easycat devices
easycat devices info --serial SERIAL
easycat apps --serial SERIAL [--query TEXT]
easycat apps info --serial SERIAL --package NAME
easycat logs --serial SERIAL --package NAME [--level E|W|I|D] [--text TEXT] [--pid-only] [--lines N] [--from TIME] [--until TIME] [-f]

1. Run easycat devices. Each line is serial, state, and model, separated by tabs. Continue with a row whose state is device.
2. Run easycat devices info --serial SERIAL when you need the raw adb devices -l line. It prints serial:, state:, model:, and raw:.
3. Run easycat apps --serial SERIAL. Add --query TEXT to keep package names that contain that text. Case does not matter.
4. Run easycat apps info --serial SERIAL --package NAME. The output has package: and pid:. An empty pid: means the app is installed and not running.
5. Run easycat logs --serial SERIAL --package NAME. Without -f the command prints the current buffer and exits. --lines N, also -n, stops after that many matching lines. --from and --until keep lines inside that window, including the endpoints. A time is YYYY-MM-DD HH:MM:SS, YYYY-MM-DDTHH:MM:SS, or MM-DD HH:MM:SS, with or without .mmm. -f, also --follow, keeps reading new lines until the process stops, the line count is reached, or a line passes --until. --pid-only defaults to on, so the lines are from the PID resolved when the command started. --level is E, W, I, or D. --text is a substring and ignores case.
6. After the app process restarts, run easycat logs again. That resolves the PID from scratch.
`

var skillRelPaths = []string{
	filepath.Join(".agents", "skills", "easycat", "SKILL.md"),
	filepath.Join(".grok", "skills", "easycat", "SKILL.md"),
	filepath.Join(".claude", "skills", "easycat", "SKILL.md"),
	filepath.Join(".cursor", "skills", "easycat", "SKILL.md"),
}

func cmdInstallSkill(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stderr, "easycat install-skill\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %s", args[0])
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	paths, err := installSkill(home)
	for _, path := range paths {
		fmt.Fprintln(stdout, path)
	}
	return err
}

func installSkill(home string) ([]string, error) {
	written := make([]string, 0, len(skillRelPaths))
	for _, rel := range skillRelPaths {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(path, []byte(skillMarkdown), 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}
