package domain

import (
	"regexp"
	"strings"
)

var (
	threadTimeRE = regexp.MustCompile(`^\s*(\d\d-\d\d\s+\d\d:\d\d:\d\d\.\d+)\s+(\d+)\s+\d+\s+([A-Z])\s+([^:]+):\s?(.*)$`)
	briefRE      = regexp.MustCompile(`^\s*([A-Z])/([^(]+)\(\s*(\d+)\):\s?(.*)$`)
)

func ParseDeviceLine(line string) (Device, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || strings.HasPrefix(line, "List of devices") {
		return Device{}, false
	}

	device := Device{
		Serial: fields[0],
		State:  fields[1],
		Raw:    line,
	}
	for _, field := range fields[2:] {
		if model, ok := strings.CutPrefix(field, "model:"); ok {
			device.Model = model
			break
		}
	}
	return device, true
}

func ParsePackageLine(line string) (Package, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Package{}, false
	}
	name, ok := strings.CutPrefix(line, "package:")
	if !ok {
		return Package{}, false
	}
	name = strings.TrimSpace(name)
	return Package{Name: name}, name != ""
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
