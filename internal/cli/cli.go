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
	"time"

	"github.com/c1r5/easycat/internal/core"
)

const usage = `easycat devices
easycat devices info --serial SERIAL
easycat apps --serial SERIAL [--query TEXT]
easycat apps info --serial SERIAL --package NAME
easycat logs --serial SERIAL [--package NAME] [--level E|W|I|D] [--text TEXT] [--pid-only] [--lines N] [--from TIME] [--until TIME] [-f]
easycat install-skill

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
	case "install-skill":
		return cmdInstallSkill(args[1:], stdout, stderr)
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
	var lines int
	var follow bool
	fs.IntVar(&lines, "n", 0, "maximum number of matching lines")
	fs.IntVar(&lines, "lines", 0, "maximum number of matching lines")
	fromFlag := fs.String("from", "", "include lines at or after this time")
	untilFlag := fs.String("until", "", "include lines at or before this time")
	fs.BoolVar(&follow, "f", false, "keep reading new lines")
	fs.BoolVar(&follow, "follow", false, "keep reading new lines")
	if err := parseFlags(fs, args); err != nil {
		return finish(err)
	}
	if *serial == "" {
		return errors.New("--serial is required")
	}
	normalized := strings.ToUpper(*level)
	switch normalized {
	case "", "E", "W", "I", "D":
	default:
		return errors.New("level must be E, W, I, or D")
	}
	pid := ""
	pidOnlyOn := *pidOnly && *name != ""
	if pidOnlyOn {
		var err error
		pid, err = client.PID(ctx, *serial, *name)
		if err != nil || pid == "" {
			return fmt.Errorf("pid not found for %s", *name)
		}
	}
	if lines < 0 {
		return errors.New("--lines must be zero or greater")
	}
	now := time.Now()
	var from, until time.Time
	var hasFrom, hasUntil bool
	if *fromFlag != "" {
		parsed, err := parseUserTime(*fromFlag, now)
		if err != nil {
			return err
		}
		from, hasFrom = parsed, true
	}
	if *untilFlag != "" {
		parsed, err := parseUserTime(*untilFlag, now)
		if err != nil {
			return err
		}
		until, hasUntil = parsed, true
	}
	if hasFrom && hasUntil && from.After(until) {
		return errors.New("--from is after --until")
	}
	year := now.Year()
	if hasFrom {
		year = from.Year()
	} else if hasUntil {
		year = until.Year()
	}
	query := core.LogcatQuery{Follow: follow}
	if hasFrom {
		query.Since = from.Format("01-02 15:04:05.000")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	bus := core.NewBus(core.DefaultQueueSize)
	sub := bus.Subscribe()
	defer sub.Cancel()
	stream, err := client.StartLogcatQuery(ctx, *serial, bus, 1, query)
	if err != nil {
		return err
	}
	defer stream.Stop()

	filters := core.Filters{Package: *name, Text: *text, Level: normalized, PIDOnly: pidOnlyOn, PID: pid}
	printed := 0
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
				if event.StreamID != 1 {
					continue
				}
				keep, pastUntil := acceptLogTime(event.Line, from, until, hasFrom, hasUntil, year)
				if pastUntil {
					return nil
				}
				if !keep || !filters.Match(event.Line) {
					continue
				}
				fmt.Fprintln(stdout, event.Line.Raw)
				printed++
				if lines > 0 && printed >= lines {
					return nil
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

func parseUserTime(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"01-02 15:04:05.000",
		"01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, now.Location())
		if err != nil {
			continue
		}
		if !strings.HasPrefix(layout, "2006") {
			parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), now.Location())
		}
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", value)
}

func acceptLogTime(line core.LogLine, from, until time.Time, hasFrom, hasUntil bool, year int) (keep bool, pastUntil bool) {
	if !hasFrom && !hasUntil {
		return true, false
	}
	got, ok := parseLogTime(line.Timestamp, year, from.Location())
	if !ok {
		return false, false
	}
	if hasUntil && got.After(until) {
		return false, true
	}
	if hasFrom && got.Before(from) {
		return false, false
	}
	return true, false
}

func parseLogTime(value string, year int, loc *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if loc == nil {
		loc = time.Local
	}
	for _, layout := range []string{"01-02 15:04:05.000", "01-02 15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err != nil {
			continue
		}
		return time.Date(year, parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc), true
	}
	return time.Time{}, false
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
