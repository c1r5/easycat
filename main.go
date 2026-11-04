package main

import (
	"context"
	"fmt"
	"os"

	"github.com/c1r5/easycat/internal/app"
	"github.com/c1r5/easycat/internal/cli"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	if len(os.Args) > 1 {
		err = cli.Run(ctx, os.Args[1:])
	} else {
		err = app.Run(ctx)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
