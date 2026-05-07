package tui

import "time"

const (
	adbPIDTimeout    = 2 * time.Second
	logBatchMaxLines = 64
	logBatchWindow   = 16 * time.Millisecond
)
