package commands

import (
	"context"
	"fmt"
	"path"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func GlobalFlags() []cli.Flag {
	globalFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "log-level",
			Usage:   "Log Level",
			Aliases: []string{"l"},
			Sources: cli.EnvVars("LOGLEVEL"),
			Value:   "info",
		},
		&cli.BoolFlag{
			Name:  "log-caller",
			Usage: "log the caller (aka line number and file)",
		},
		&cli.BoolFlag{
			Name:  "log-disable-color",
			Usage: "disable log coloring",
		},
		&cli.BoolFlag{
			Name:  "log-full-timestamp",
			Usage: "force log output to always show full timestamp",
		},
	}

	return globalFlags
}

func GlobalBefore(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	formatter := &logrus.TextFormatter{
		DisableColors: cmd.Bool("log-disable-color"),
		FullTimestamp: cmd.Bool("log-full-timestamp"),
	}
	if cmd.Bool("log-caller") {
		logrus.SetReportCaller(true)

		formatter.CallerPrettyfier = func(f *runtime.Frame) (string, string) {
			return "", fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
		}
	}

	logrus.SetFormatter(formatter)

	switch cmd.String("log-level") {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	}

	return ctx, nil
}
