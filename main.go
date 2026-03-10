package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/services"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}

	return ""
}()

func main() {
	ctx := context.Background()
	var cfg *Config

	app := &cli.Command{
		Name:  "athanor",
		Usage: "Backup quadlet volumes",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "debug",
				Usage:    "enable debug mode",
				Required: false,
				Sources:  cli.EnvVars("ATHANOR_DEBUG"),
				Action: func(ctx context.Context, cm *cli.Command, b bool) error {
					log.Default().SetLevel(log.DebugLevel)
					log.Default().SetReportCaller(true)
					return nil
				},
			},
			&cli.StringFlag{
				Name:     "config",
				Required: false,
				Aliases:  []string{"c"},
				Usage:    "Path to config file",
				Sources:  cli.EnvVars("ATHANOR_CONFIG"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			var err error
			cfg, err = NewConfig(cmd.String("config"))
			return ctx, err
		},
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Show athanor config",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					log.Print(cfg)
					return nil
				},
			},
			{
				Name:  "version",
				Usage: "show version",
				Action: func(ctx context.Context, _ *cli.Command) error {
					fmt.Printf("athanor version git-%v\n", Commit)
					return nil
				},
			},
			{
				Name:  "plan",
				Usage: "Plan a backup run for one or more components. Defaults to all",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "quiet",
						Aliases: []string{"q"},
						Usage:   "Minimize output",
						Value:   false,
					},
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Component name to backup",
					},
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Usage:   "Control output format. Supports text,json",
						Value:   "text",
					},
					&cli.StringFlag{
						Name:    "group",
						Aliases: []string{"g"},
						Usage:   "Backup only quadlets in group",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					compMgr := &reader{cfg.QuadletDir, cfg.DataDir}
					conman, err := containers.NewPodmanManager(&containers.ContainersConfig{
						SecretsPrefix:      "materia-",
						CompressionCommand: cfg.CompressionCommand,
						CompressionSuffix:  cfg.CompressionSuffix,
					})
					if err != nil {
						return err
					}
					serv, err := services.NewServices(ctx, &services.ServicesConfig{})
					if err != nil {
						return err
					}

					plan, err := buildPlan(ctx, compMgr, conman, *serv, cmd.String("name"), cmd.String("group"))
					if err != nil {
						return err
					}
					if !cmd.Bool("quiet") {
						err = printPlan(plan, cmd.String("format"))
						if err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "backup",
				Usage: "Backup one or more components",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "quiet",
						Aliases: []string{"q"},
						Usage:   "Minimize output",
						Value:   false,
					},
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Component name to backup",
					},
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Usage:   "Control output format. Supports text,json",
						Value:   "text",
					},
					&cli.StringFlag{
						Name:    "group",
						Aliases: []string{"g"},
						Usage:   "Backup only quadlets in group",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					compMgr := &reader{cfg.QuadletDir, cfg.DataDir}
					conman, err := containers.NewPodmanManager(&containers.ContainersConfig{
						SecretsPrefix:      "materia-",
						CompressionCommand: cfg.CompressionCommand,
						CompressionSuffix:  cfg.CompressionSuffix,
					})
					if err != nil {
						return err
					}
					serv, err := services.NewServices(ctx, &services.ServicesConfig{})
					if err != nil {
						return err
					}

					plan, err := buildPlan(ctx, compMgr, conman, *serv, cmd.String("name"), cmd.String("group"))
					if err != nil {
						return err
					}
					if !cmd.Bool("quiet") {
						err = printPlan(plan, cmd.String("format"))
						if err != nil {
							return err
						}
					}
					writer := &Writer{
						*serv,
						conman,
					}

					doit := executor.NewExecutor(executor.ExecutorConfig{
						MateriaDir: cfg.DataDir,
						QuadletDir: cfg.QuadletDir,
						OutputDir:  cfg.OutputDir,
					}, writer, 90)
					_, err = doit.Execute(ctx, plan)
					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}
	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
