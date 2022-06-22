package main

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/fatih/color"
	"github.com/fujiwara/ecrm"
	"github.com/fujiwara/logutils"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

var filter = &logutils.LevelFilter{
	Levels: []logutils.LogLevel{"debug", "info", "notice", "warn", "error"},
	ModifierFuncs: []logutils.ModifierFunc{
		nil,
		logutils.Color(color.FgWhite),
		logutils.Color(color.FgHiBlue),
		logutils.Color(color.FgYellow),
		logutils.Color(color.FgRed, color.Bold),
	},
	Writer: os.Stderr,
}

func main() {
	ecrmApp, err := ecrm.New(context.Background(), os.Getenv("AWS_REGION"))
	if err != nil {
		log.Fatal(err)
	}
	app := &cli.App{
		Name:  "ecrm",
		Usage: "A command line tool for managing ECR repositories",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "ecrm.yaml",
				Usage:   "Load configuration from `FILE`",
				EnvVars: []string{"ECRM_CONFIG"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Set log level (debug, info, notice, warn, error)",
				EnvVars: []string{"ECRM_LOG_LEVEL"},
			},
			&cli.BoolFlag{
				Name:    "no-color",
				Value:   !isatty.IsTerminal(os.Stdout.Fd()),
				Usage:   "Whether or not to color the output",
				EnvVars: []string{"ECRM_NO_COLOR"},
			},
		},
		Before: func(c *cli.Context) error {
			color.NoColor = c.Bool("no-color")
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "plan",
				Usage: "scan ECS resources and find unused ECR images to delete safety.",
				Action: func(c *cli.Context) error {
					setLogLevel(c.String("log-level"))
					return ecrmApp.Run(
						c.String("config"),
						ecrm.Option{
							Repository: c.String("repository"),
							NoColor:    c.Bool("no-color"),
						},
					)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "repository",
						Aliases:     []string{"r"},
						DefaultText: "",
						Usage:       "plan for only images in `REPOSITORY`",
						EnvVars:     []string{"ECRM_REPOSITORY"},
					},
				},
			},
			{
				Name:  "delete",
				Usage: "scan ECS resources and delete unused ECR images.",
				Action: func(c *cli.Context) error {
					setLogLevel(c.String("log-level"))
					return ecrmApp.Run(
						c.String("config"),
						ecrm.Option{
							Delete:     true,
							Force:      c.Bool("force"),
							Repository: c.String("repository"),
							NoColor:    c.Bool("no-color"),
						},
					)
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Usage:   "force delete images without confirmation",
						EnvVars: []string{"ECRM_FORCE"},
					},
					&cli.StringFlag{
						Name:        "repository",
						Aliases:     []string{"r"},
						DefaultText: "",
						Usage:       "delete only images in `REPOSITORY`",
						EnvVars:     []string{"ECRM_REPOSITORY"},
					},
				},
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if isLambda() && os.Getenv("ECRM_NO_LAMBDA_BOOTSTRAP") == "" {
		app.Action = func(c *cli.Context) error {
			setLogLevel(c.String("log-level"))
			subcommand := os.Getenv("ECRM_COMMAND")
			lambda.Start(func() error {
				return ecrmApp.Run(
					c.String("config"),
					ecrm.Option{
						Delete:     subcommand == "delete",
						Force:      subcommand == "delete", //If it works as bootstrap for a Lambda function, delete images without confirmation.
						Repository: os.Getenv("ECRM_REPOSITORY"),
						NoColor:    c.Bool("no-color"),
					},
				)
			})
			return nil
		}
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setLogLevel(level string) {
	if level != "" {
		filter.MinLevel = logutils.LogLevel(level)
	}
	log.SetOutput(filter)
	log.Println("[debug] Setting log level to", level)
}

func isLambda() bool {
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		return true
	}
	return false
}
