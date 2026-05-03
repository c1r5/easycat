package domain

import "testing"

func TestParseDeviceLine(t *testing.T) {
	device, ok := ParseDeviceLine("emulator-5554 device product:sdk model:Pixel_7 device:emu transport_id:1")
	if !ok {
		t.Fatal("expected device")
	}
	if device.Serial != "emulator-5554" || device.State != "device" || device.Model != "Pixel_7" {
		t.Fatalf("unexpected device: %+v", device)
	}
}

func TestParsePackageLine(t *testing.T) {
	pkg, ok := ParsePackageLine("package:com.foo.app")
	if !ok || pkg.Name != "com.foo.app" {
		t.Fatalf("unexpected package: %+v ok=%v", pkg, ok)
	}
}

func TestParseThreadTimeLogLine(t *testing.T) {
	line := ParseLogLine("05-03 12:01:02.345  1234  5678 E Foo: failed hard")
	if line.Timestamp != "05-03 12:01:02.345" || line.PID != "1234" || line.Level != "E" || line.Tag != "Foo" || line.Message != "failed hard" {
		t.Fatalf("unexpected log line: %+v", line)
	}
}

func TestFiltersMatch(t *testing.T) {
	line := LogLine{Level: "E", PID: "1234", Raw: "05-03 E Foo: failed hard"}
	filters := Filters{Text: "FAILED", Level: "E", PIDOnly: true, PID: "1234"}
	if !filters.Match(line) {
		t.Fatal("expected filters to match")
	}
	filters.PID = "9999"
	if filters.Match(line) {
		t.Fatal("expected pid filter to reject line")
	}
}

func TestLogBufferKeepsLimit(t *testing.T) {
	buffer := NewLogBuffer(2)
	buffer.Add(LogLine{Raw: "one"})
	buffer.Add(LogLine{Raw: "two"})
	buffer.Add(LogLine{Raw: "three"})
	lines := buffer.Lines()
	if len(lines) != 2 || lines[0].Raw != "two" || lines[1].Raw != "three" {
		t.Fatalf("unexpected lines: %+v", lines)
	}
}
