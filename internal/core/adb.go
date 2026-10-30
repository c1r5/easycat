package core

import (
	"context"
	"os/exec"
)

type Client struct {
	Path string
}

func NewClient() Client {
	return Client{Path: "adb"}
}

func (c Client) output(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, c.Path, args...).Output()
}
