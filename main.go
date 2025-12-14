package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/ekristen/go-checkpoint"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/ekristen/cast/pkg/common"
	"github.com/ekristen/cast/pkg/httputil"

	_ "github.com/ekristen/cast/pkg/commands/init"
	_ "github.com/ekristen/cast/pkg/commands/install"
	_ "github.com/ekristen/cast/pkg/commands/release"
	_ "github.com/ekristen/cast/pkg/commands/test"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			// log panics forces exit
			if _, ok := r.(*logrus.Entry); ok {
				os.Exit(1)
			}
			panic(r)
		}
	}()

	checkpointResult := make(chan *checkpoint.CheckResponse, 1)
	go func() {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			logrus.Fatalf("Error getting cache directory: %v", err)
		}

		params := &checkpoint.CheckParams{
			Product:    common.NAME,
			Version:    common.SUMMARY,
			HTTPClient: httputil.NewClient(),
		}
		if cacheDir != "" {
			params.CacheFile = filepath.Join(cacheDir, "cast", "checkpoint")
		}

		resp, err := checkpoint.Check(params)
		if err != nil {
			logrus.WithError(err).Debug("checkpoint check failed")
			checkpointResult <- nil
			return
		}
		checkpointResult <- resp
	}()

	cmd := &cli.Command{
		Name:    path.Base(os.Args[0]),
		Usage:   common.AppVersion.Name,
		Version: common.AppVersion.Summary,
		Authors: []any{
			"Erik Kristensen <erik@erikkristensen.com>",
		},
		Commands: common.GetCommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		logrus.Fatal(err)
	}

	select {
	case resp := <-checkpointResult:
		if resp != nil && resp.Outdated {
			fmt.Fprintf(os.Stderr, "\nA new version of %s is available: %s (current: %s)\n", common.NAME, resp.CurrentVersion, common.SUMMARY)
			fmt.Fprintf(os.Stderr, "Download at: %s\n", resp.CurrentDownloadURL)
		}
	default:
	}
}
