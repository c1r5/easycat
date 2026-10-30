package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/c1r5/easycat/internal/core"
)

const usage = `easycat devices
easycat devices info --serial SERIAL
easycat apps --serial SERIAL [--query TEXT]
easycat apps info --serial SERIAL --package NAME
easycat logs --serial SERIAL --package NAME [--level E|W|I|D] [--text TEXT] [--pid-only]

With no arguments, easycat opens the terminal UI.
`

var errHelpShown = errors.New("help")

func Run(ctx context.Context, args []string) error {
	return execute(ctx, args, core.NewClient(), os.Stdout, os.Stderr)
}

func execute(ctx context.Context, args []string, client core.Client, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stderr, usage)
		return nil
	}
	switch args[0] {
	case "devices":
		return cmdDevices(ctx, args[1:], client, stdout, stderr)
	case "apps":
		return cmdApps(ctx, args[1:], client, stdout, stderr)
	case "logs":
		return cmdLogs(ctx, args[1:], client, stdout, stderr)
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %s", args[0])
	}
}

func cmdDevices(ctx context.Context, args []string, client core.Client, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		devices, err := client.ListDevices(ctx)
		if err != nil {
			return err
		}
		for _, device := range devices {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", device.Serial, device.State, device.Model)
		}
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stderr, "easycat devices\neasycat devices info --serial SERIAL\n")
		return nil
	}
	if args[0] != "info" {
		return fmt.Errorf("unknown devices command %s", args[0])
	}
	fs := flagSet("devices info", stderr)
	serial := fs.String("serial", "", "device serial")
	if err := parseFlags(fs, args[1:]); err != nil {
		return finish(err)
	}
	if *serial == "" {
		return errors.New("--serial is required")
	}
	device, err := client.DeviceInfo(ctx, *serial)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "serial: %s\nstate: %s\nmodel: %s\nraw: %s\n", device.Serial, device.State, device.Model, device.Raw)
	return nil
}

func cmdApps(ctx context.Context, args []string, client core.Client, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "info" {
		return cmdAppInfo(ctx, args[1:], client, stdout, stderr)
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stderr, "easycat apps --serial SERIAL [--query TEXT]\neasycat apps info --serial SERIAL --package NAME\n")
		return nil
	}
	fs := flagSet("apps", stderr)
	serial := fs.String("serial", "", "device serial")
	query := fs.String("query", "", "case-insensitive package substring")
	if err := parseFlags(fs, args); err != nil {
		return finish(err)
	}
	if *serial == "" {
		return errors.New("--serial is required")
	}
	apps, err := client.ListApps(ctx, *serial)
	if err != nil {
		return err
	}
	needle := strings.ToLower(strings.TrimSpace(*query))
	for _, app := range apps {
		if needle == "" || strings.Contains(strings.ToLower(app.Name), needle) {
			fmt.Fprintln(stdout, app.Name)
		}
	}
	return nil
}

func cmdAppInfo(ctx context.Context, args []string, client core.Client, stdout, stderr io.Writer) error {
	fs := flagSet("apps info", stderr)
	serial := fs.String("serial", "", "device serial")
	name := fs.String("package", "", "package name")
	if err := parseFlags(fs, args); err != nil {
		return finish(err)
	}
	if *serial == "" || *name == "" {
		return errors.New("--serial and --package are required")
	}
	app, err := client.AppInfo(ctx, *serial, *name)
	if err != nil {
		return err
	}
	pid, err := client.PID(ctx, *serial, app.Name)
	if err != nil {
		pid = ""
	}
	fmt.Fprintf(stdout, "package: %s\npid: %s\n", app.Name, pid)
	return nil
}

func cmdLogs(ctx context.Context, args []string, client core.Client, stdout, stderr io.Writer) error {
	fs := flagSet("logs", stderr)
	serial := fs.String("serial", "", "device serial")
	name := fs.String("package", "", "package name")
	level := fs.String("level", "", "E, W, I, or D")
	text := fs.String("text", "", "case-insensitive substring")
	pidOnly := fs.Bool("pid-only", true, "print lines from the package pid")
	if err := parseFlags(fs, args); err != nil {
		return finish(err)
	}
	if *serial == "" || *name == "" {
		return errors.New("--serial and --package are required")
	}
	normalized := strings.ToUpper(*level)
	switch normalized {
	case "", "E", "W", "I", "D":
	default:
		return errors.New("level must be E, W, I, or D")
	}
	pid := ""
	if *pidOnly {
		var err error
		pid, err = client.PID(ctx, *serial, *name)
		if err != nil || pid == "" {
			return fmt.Errorf("pid not found for %s", *name)
		}
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	bus := core.NewBus(core.DefaultQueueSize)
	sub := bus.Subscribe()
	defer sub.Cancel()
	stream, err := client.StartLogcat(ctx, *serial, bus, 1)
	if err != nil {
		return err
	}
	defer stream.Stop()

	filters := core.Filters{Package: *name, Text: *text, Level: normalized, PIDOnly: *pidOnly, PID: pid}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-sub.Events:
			if !ok {
				return nil
			}
			switch event.Kind {
			case core.KindLog:
				if event.StreamID == 1 && filters.Match(event.Line) {
					fmt.Fprintln(stdout, event.Line.Raw)
				}
			case core.KindStreamStopped:
				if event.StreamID != 1 || event.Err == nil || errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}
				return event.Err
			}
		}
	}
}

func flagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
	}
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelpShown
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %s", fs.Arg(0))
	}
	return nil
}

func finish(err error) error {
	if errors.Is(err, errHelpShown) {
		return nil
	}
	return err
}
