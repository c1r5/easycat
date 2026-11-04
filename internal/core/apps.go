package core

import (
	"context"
	"fmt"
	"strings"
)

type App struct {
	Name string
}

func (a App) Title() string       { return a.Name }
func (a App) Description() string { return "" }
func (a App) FilterValue() string { return a.Name }

func ParseAppLine(line string) (App, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return App{}, false
	}
	name, ok := strings.CutPrefix(line, "package:")
	if !ok {
		return App{}, false
	}
	name = strings.TrimSpace(name)
	return App{Name: name}, name != ""
}

func (c Client) ListApps(ctx context.Context, serial string) ([]App, error) {
	output, err := c.output(ctx, "-s", serial, "shell", "pm", "list", "packages")
	if err != nil {
		return nil, err
	}
	apps := []App{}
	for _, line := range strings.Split(string(output), "\n") {
		if app, ok := ParseAppLine(line); ok {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func (c Client) AppInfo(ctx context.Context, serial, name string) (App, error) {
	apps, err := c.ListApps(ctx, serial)
	if err != nil {
		return App{}, err
	}
	for _, app := range apps {
		if app.Name == name {
			return app, nil
		}
	}
	return App{}, fmt.Errorf("app %s not found", name)
}

func (c Client) PID(ctx context.Context, serial, name string) (string, error) {
	output, err := c.output(ctx, "-s", serial, "shell", "pidof", name)
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
