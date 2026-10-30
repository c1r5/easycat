package core

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

var (
	threadTimeRE = regexp.MustCompile(`^\s*(\d\d-\d\d\s+\d\d:\d\d:\d\d\.\d+)\s+(\d+)\s+\d+\s+([A-Z])\s+([^:]+):\s?(.*)$`)
	briefRE      = regexp.MustCompile(`^\s*([A-Z])/([^(]+)\(\s*(\d+)\):\s?(.*)$`)
)

type LogLine struct {
	Timestamp string
	Level     string
	Tag       string
	PID       string
	Message   string
	Raw       string
}

func ParseLogLine(line string) LogLine {
	raw := strings.TrimRight(line, "\r\n")
	if matches := threadTimeRE.FindStringSubmatch(raw); matches != nil {
		return LogLine{
			Timestamp: matches[1],
			PID:       matches[2],
			Level:     matches[3],
			Tag:       strings.TrimSpace(matches[4]),
			Message:   matches[5],
			Raw:       raw,
		}
	}
	if matches := briefRE.FindStringSubmatch(raw); matches != nil {
		return LogLine{
			Level:   matches[1],
			Tag:     strings.TrimSpace(matches[2]),
			PID:     matches[3],
			Message: matches[4],
			Raw:     raw,
		}
	}
	return LogLine{Raw: raw}
}

type Stream struct {
	cancel context.CancelFunc
}

func (s *Stream) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func (c Client) StartLogcat(ctx context.Context, serial string, bus *Bus, streamID int) (*Stream, error) {
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

	go func() {
		var stopErr error
		defer func() {
			if bus != nil {
				bus.Publish(Event{Kind: KindStreamStopped, StreamID: streamID, Err: stopErr})
			}
		}()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if streamCtx.Err() != nil {
				return
			}
			if bus != nil {
				bus.Publish(Event{
					Kind:     KindLog,
					StreamID: streamID,
					Line:     ParseLogLine(scanner.Text()),
				})
			}
		}
		scanErr := scanner.Err()
		waitErr := cmd.Wait()
		if errors.Is(streamCtx.Err(), context.Canceled) {
			return
		}
		if scanErr != nil {
			stopErr = scanErr
			return
		}
		if waitErr != nil {
			errScanner := bufio.NewScanner(stderr)
			var errText string
			for errScanner.Scan() {
				errText += errScanner.Text() + "\n"
			}
			if strings.TrimSpace(errText) != "" {
				stopErr = errors.New(strings.TrimSpace(errText))
				return
			}
			stopErr = waitErr
		}
	}()

	return &Stream{cancel: cancel}, nil
}

func (c Client) logcatArgs(serial string) []string {
	return []string{"-s", serial, "logcat", "-v", "threadtime", "-T", "0"}
}

func (c Client) LogcatWindow() string {
	return "realtime"
}
