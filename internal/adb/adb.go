package adb

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/c1r5/easycat/internal/domain"
)

type Client struct {
	Path string
}

func NewClient() Client {
	return Client{Path: "adb"}
}

func (c Client) ListDevices(ctx context.Context) ([]domain.Device, error) {
	output, err := exec.CommandContext(ctx, c.Path, "devices", "-l").Output()
	if err != nil {
		return nil, err
	}
	devices := []domain.Device{}
	for _, line := range strings.Split(string(output), "\n") {
		if device, ok := domain.ParseDeviceLine(line); ok {
			devices = append(devices, device)
		}
	}
	return devices, nil
}

func (c Client) ListPackages(ctx context.Context, serial string) ([]domain.Package, error) {
	output, err := exec.CommandContext(ctx, c.Path, "-s", serial, "shell", "pm", "list", "packages").Output()
	if err != nil {
		return nil, err
	}
	packages := []domain.Package{}
	for _, line := range strings.Split(string(output), "\n") {
		if pkg, ok := domain.ParsePackageLine(line); ok {
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func (c Client) PIDOf(ctx context.Context, serial, packageName string) (string, error) {
	output, err := exec.CommandContext(ctx, c.Path, "-s", serial, "shell", "pidof", packageName).Output()
	pid := strings.TrimSpace(string(output))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(pid)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

type Stream struct {
	Lines  <-chan domain.LogLine
	Done   <-chan error
	cancel context.CancelFunc
}

func (s *Stream) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func (c Client) StartLogcat(ctx context.Context, serial string) (*Stream, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(streamCtx, c.Path, c.logcatArgs(serial)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	lines := make(chan domain.LogLine, 256)
	done := make(chan error, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case lines <- domain.ParseLogLine(scanner.Text()):
			case <-streamCtx.Done():
				done <- nil
				return
			}
		}
		scanErr := scanner.Err()
		waitErr := cmd.Wait()
		if errors.Is(streamCtx.Err(), context.Canceled) {
			done <- nil
			return
		}
		if scanErr != nil {
			done <- scanErr
			return
		}
		if waitErr != nil {
			errScanner := bufio.NewScanner(stderr)
			var errText string
			for errScanner.Scan() {
				errText += errScanner.Text() + "\n"
			}
			if strings.TrimSpace(errText) != "" {
				done <- errors.New(strings.TrimSpace(errText))
				return
			}
			done <- waitErr
			return
		}
		done <- nil
	}()

	return &Stream{Lines: lines, Done: done, cancel: cancel}, nil
}

func (c Client) logcatArgs(serial string) []string {
	return []string{"-s", serial, "logcat", "-v", "threadtime", "-T", "0"}
}

func (c Client) LogcatWindow() string {
	return "realtime"
}
