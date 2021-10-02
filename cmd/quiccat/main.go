package main

import (
	"os"

	"github.com/lthibault/log"
	"github.com/urfave/cli/v2"

	"github.com/taylorjdawson/quiccat/internal/cmd/client"
	"github.com/taylorjdawson/quiccat/internal/cmd/server"
)

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "logfmt",
		Aliases: []string{"f"},
		Usage:   "text, json, none",
		Value:   "text",
		EnvVars: []string{"LOGFMT"},
	},
	&cli.StringFlag{
		Name:    "loglvl",
		Usage:   "trace, debug, info, warn, error, fatal",
		Value:   "info",
		EnvVars: []string{"LOGLVL"},
	},
	&cli.BoolFlag{
		Name:    "prettyprint",
		Aliases: []string{"pp"},
		Usage:   "pretty-print JSON output",
		Hidden:  true,
	},
}

var commands = []*cli.Command{
	server.Command(),
	client.Command(),
}

func main() {
	app := cli.App{
		Name:     "quiccat",
		Usage:    "simple, quic (pun), cli tool",
		Flags:    flags,
		Commands: commands,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}