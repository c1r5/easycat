package core

import (
	"context"
	"fmt"
	"strings"
)

type Device struct {
	Serial string
	Model  string
	State  string
	Raw    string
}

func (d Device) Title() string {
	state := d.State
	if state == "" {
		state = "unknown"
	}
	if d.Model != "" {
		return d.Model + "  " + state
	}
	return d.Serial + "  " + state
}

func (d Device) Description() string {
	if d.Model != "" && d.Serial != "" {
		return d.Serial + "  " + d.State
	}
	return d.State
}

func (d Device) FilterValue() string {
	return d.Serial + " " + d.Model + " " + d.State
}

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

func (c Client) ListDevices(ctx context.Context) ([]Device, error) {
	output, err := c.output(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	devices := []Device{}
	for _, line := range strings.Split(string(output), "\n") {
		if device, ok := ParseDeviceLine(line); ok {
			devices = append(devices, device)
		}
	}
	return devices, nil
}

func (c Client) DeviceInfo(ctx context.Context, serial string) (Device, error) {
	devices, err := c.ListDevices(ctx)
	if err != nil {
		return Device{}, err
	}
	for _, device := range devices {
		if device.Serial == serial {
			return device, nil
		}
	}
	return Device{}, fmt.Errorf("device %s not found", serial)
}
