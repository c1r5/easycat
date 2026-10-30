package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c1r5/easycat/internal/core"
)

func TestAgentCommandChain(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "adb")
	body := []byte(`#!/bin/sh
if [ "$1" = "devices" ]; then
  printf '%s\n' "List of devices attached" "emulator-5554 device product:sdk model:Pixel_7" "offline-1 offline"
  exit 0
fi
if [ "$1" = "-s" ] && [ "$3" = "shell" ] && [ "$4" = "pm" ]; then
  printf '%s\n' "package:com.example" "package:com.other"
  exit 0
fi
if [ "$1" = "-s" ] && [ "$3" = "shell" ] && [ "$4" = "pidof" ]; then
  if [ "$5" = "com.example" ]; then
    echo 4321
    exit 0
  fi
  exit 1
fi
if [ "$1" = "-s" ] && [ "$3" = "logcat" ]; then
  printf '%s\n' \
    "01-01 00:00:00.000 4321 1 I Tag: hello app" \
    "01-01 00:00:00.001 9999 1 I Tag: other" \
    "01-01 00:00:00.002 4321 1 E Tag: boom"
  exit 0
fi
echo "unexpected: $*" >&2
exit 1
`)
	if err := os.WriteFile(script, body, 0o755); err != nil {
		t.Fatal(err)
	}
	client := core.Client{Path: script}
	ctx := context.Background()

	var stderr bytes.Buffer
	err := execute(ctx, []string{"--help"}, client, ioDiscard(), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	help := stderr.String()
	for _, part := range []string{
		"easycat devices",
		"devices info",
		"--serial",
		"easycat apps",
		"--query",
		"apps info",
		"--package",
		"easycat logs",
		"--level",
		"--text",
		"--pid-only",
	} {
		if !strings.Contains(help, part) {
			t.Fatalf("help missing %q:\n%s", part, help)
		}
	}

	stderr.Reset()
	err = execute(ctx, []string{"devices", "--help"}, client, ioDiscard(), &stderr)
	if err != nil || !strings.Contains(stderr.String(), "devices info") || !strings.Contains(stderr.String(), "--serial") {
		t.Fatalf("devices help: err=%v\n%s", err, stderr.String())
	}
	stderr.Reset()
	err = execute(ctx, []string{"apps", "--help"}, client, ioDiscard(), &stderr)
	if err != nil || !strings.Contains(stderr.String(), "--query") || !strings.Contains(stderr.String(), "apps info") {
		t.Fatalf("apps help: err=%v\n%s", err, stderr.String())
	}
	stderr.Reset()
	err = execute(ctx, []string{"logs", "--help"}, client, ioDiscard(), &stderr)
	if err != nil || !strings.Contains(stderr.String(), "--level") || !strings.Contains(stderr.String(), "--text") || !strings.Contains(stderr.String(), "--pid-only") {
		t.Fatalf("logs help: err=%v\n%s", err, stderr.String())
	}

	var stdout bytes.Buffer
	if err := execute(ctx, []string{"devices"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "emulator-5554\tdevice\tPixel_7\noffline-1\toffline\t\n" {
		t.Fatalf("devices = %q", stdout.String())
	}

	stdout.Reset()
	if err := execute(ctx, []string{"devices", "info", "--serial", "emulator-5554"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	info := stdout.String()
	for _, part := range []string{"serial: emulator-5554\n", "state: device\n", "model: Pixel_7\n", "model:Pixel_7"} {
		if !strings.Contains(info, part) {
			t.Fatalf("device info missing %q:\n%s", part, info)
		}
	}
	if err := execute(ctx, []string{"devices", "info", "--serial", "missing"}, client, ioDiscard(), ioDiscard()); err == nil {
		t.Fatal("expected missing device to fail")
	}

	stdout.Reset()
	if err := execute(ctx, []string{"apps", "--serial", "emulator-5554", "--query", "EXAMPLE"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "com.example\n" {
		t.Fatalf("apps = %q", stdout.String())
	}

	stdout.Reset()
	if err := execute(ctx, []string{"apps", "info", "--serial", "emulator-5554", "--package", "com.example"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "package: com.example\npid: 4321\n" {
		t.Fatalf("app info = %q", stdout.String())
	}

	stdout.Reset()
	if err := execute(ctx, []string{"logs", "--serial", "emulator-5554", "--package", "com.example"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	logs := stdout.String()
	if !strings.Contains(logs, "hello app") || !strings.Contains(logs, "boom") || strings.Contains(logs, "other") {
		t.Fatalf("logs = %q", logs)
	}

	stdout.Reset()
	if err := execute(ctx, []string{"logs", "--serial", "emulator-5554", "--package", "com.example", "--level", "E"}, client, &stdout, ioDiscard()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() == logs || !strings.Contains(stdout.String(), "boom") || strings.Contains(stdout.String(), "hello app") {
		t.Fatalf("level filter = %q", stdout.String())
	}

	if err := execute(ctx, []string{"logs", "--serial", "emulator-5554", "--package", "com.missing"}, client, ioDiscard(), ioDiscard()); err == nil || !strings.Contains(err.Error(), "pid not found") {
		t.Fatalf("missing pid error = %v", err)
	}
}

func ioDiscard() *bytes.Buffer {
	return &bytes.Buffer{}
}
