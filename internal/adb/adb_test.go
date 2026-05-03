package adb

import (
	"reflect"
	"testing"
)

func TestLogcatArgsRealtime(t *testing.T) {
	client := Client{Path: "adb"}
	got := client.logcatArgs("emulator-5554")
	want := []string{"-s", "emulator-5554", "logcat", "-v", "threadtime", "-T", "0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}
