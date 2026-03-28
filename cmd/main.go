package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	athanor "git.saintnet.tech/stryan/athanor/internal"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/plan"
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
	var cfg *athanor.Config

	hostname, err := os.Hostname()
	if err != nil {
		log.Warnf("error getting hostname,setting to undefined: %v", err)
		hostname = "undefined"
	}
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
			cfg, err = athanor.NewConfig(cmd.String("config"))
			if err != nil {
				return ctx, fmt.Errorf("can not construct config: %w", err)
			}
			return ctx, validateConfig(cfg)
		},
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Show athanor config",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Println(cfg)
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
					compMgr := &athanor.Reader{QuadletPrefix: cfg.QuadletDir, DataPrefix: cfg.DataDir}
					conman, err := containers.NewPodmanManager(&containers.ContainersConfig{
						SecretsPrefix: "materia-",
						Compression:   cfg.Compression,
					})
					if err != nil {
						return err
					}
					serv, err := services.NewServices(ctx, &services.ServicesConfig{})
					if err != nil {
						return err
					}

					plan, err := buildPlan(ctx, cfg, compMgr, conman, *serv, cmd.String("name"), cmd.String("group"))
					if err != nil {
						return err
					}
					if !cmd.Bool("quiet") {
						err = printBackupPlan(plan, cmd.String("format"))
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
					&cli.StringFlag{
						Name:    "webhook",
						Aliases: []string{"w"},
						Usage:   "Webhook to use instead of configured one",
					},
					&cli.BoolFlag{
						Name:    "notify",
						Aliases: []string{"r"},
						Usage:   "Send backup report notification",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					report := &athanor.BackupReport{
						Hostname:  hostname,
						StartTime: time.Now(),
					}
					compMgr := &athanor.Reader{QuadletPrefix: cfg.QuadletDir, DataPrefix: cfg.DataDir}
					conman, err := containers.NewPodmanManager(&containers.ContainersConfig{
						SecretsPrefix: "materia-",
						Compression:   cfg.Compression,
						Remote:        (os.Getenv("container") == "podman"),
					})
					if err != nil {
						return err
					}
					shouldNotify := cfg.Notify
					if cmd.HasName("notify") {
						shouldNotify = cmd.Bool("notify")
					}
					serv, err := services.NewServices(ctx, &services.ServicesConfig{})
					if err != nil {
						return err
					}

					plan, err := buildPlan(ctx, cfg, compMgr, conman, *serv, cmd.String("name"), cmd.String("group"))
					if err != nil {
						return err
					}
					if !cmd.Bool("quiet") {
						err = printBackupPlan(plan, cmd.String("format"))
						if err != nil {
							return err
						}
					}
					writer := &athanor.Writer{
						ServiceManager:   *serv,
						ContainerManager: conman,
					}

					doit := executor.NewExecutor(executor.ExecutorConfig{
						MateriaDir: cfg.DataDir,
						QuadletDir: cfg.QuadletDir,
						OutputDir:  cfg.OutputDir,
					}, writer, 90)
					for _, c := range plan.Keys() {
						if !cmd.Bool("quiet") {
							fmt.Printf("Backing up %v\n", c)
						}
						p := plan.Components[c]
						_, err = doit.Execute(ctx, p)
						report.AddReport(c, p, err)
					}
					report.EndTime = time.Now()
					finalReport, err := report.Report()
					if err != nil {
						return fmt.Errorf("could not generate final report: %w", err)
					}
					if !cmd.Bool("quiet") {
						fmt.Println(finalReport)
					}
					if shouldNotify {
						dest := cfg.Webhook
						if dest == "" {
							dest = cmd.String("webhook")
						}
						err = athanor.Notify(ctx, dest, hostname, finalReport)
						if err != nil {
							return fmt.Errorf("could not send final report: %w", err)
						}
					}

					return nil
				},
			},
			{
				Name:  "restore",
				Usage: "Restore a component",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:        "target",
						Value:       "",
						Destination: new(string),
						UsageText:   "Target component to restore",
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "source",
						Usage:   "Source directory to restore from. Defaults to .",
						Sources: cli.ValueSourceChain{},
						Value:   ".",
						Aliases: []string{"-s"},
					},
					&cli.BoolFlag{
						Name:    "plan",
						Usage:   "Only plan the restore, don't run it",
						Value:   false,
						Aliases: []string{"-p", "-d"},
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					source := c.String("source")
					target := c.StringArg("target")
					if target == "" {
						return fmt.Errorf("need restore target")
					}
					compMgr := &athanor.Reader{QuadletPrefix: cfg.QuadletDir, DataPrefix: cfg.DataDir}
					conman, err := containers.NewPodmanManager(&containers.ContainersConfig{
						SecretsPrefix: "materia-",
						Compression:   cfg.Compression,
						Remote:        (os.Getenv("container") == "podman"),
					})
					if err != nil {
						return err
					}

					serv, err := services.NewServices(ctx, &services.ServicesConfig{})
					if err != nil {
						return err
					}

					targetComponent, err := athanor.LoadComponent(ctx, conman, compMgr, target)
					if err != nil {
						return err
					}
					volmap := make(map[components.Resource]string)
					needNames := []string{}
					for _, v := range targetComponent.Resources.List() {
						if v.Kind == components.ResourceTypeVolume {
							needNames = append(needNames, v.Name())
							volmap[v] = ""
						}
					}
					volkeys := make([]components.Resource, 0, len(volmap))
					for r := range volmap {
						volkeys = append(volkeys, r)
					}

					log.Info("verifying volumes", "needed", needNames)

					sourceEntries, err := os.ReadDir(source)
					if err != nil {
						return err
					}
					for _, e := range sourceEntries {
						for _, sv := range volkeys {
							svname := strings.TrimSuffix(sv.Name(), ".volume")
							if strings.Contains(e.Name(), svname) {
								if volmap[sv] != "" {
									return fmt.Errorf("multiple source canidates for volume %v: current: %v new %v", sv.Name(), volmap[sv], e.Name())
								}
								log.Info("found source volume", "canidate", e.Name(), "target", sv.Name())
								volmap[sv] = filepath.Join(source, e.Name())
								break
							}
						}
					}
					missingVolumes := make([]string, 0, len(needNames))
					for _, k := range volkeys {
						if volmap[k] == "" {
							missingVolumes = append(missingVolumes, k.Name())
						}
					}
					if len(missingVolumes) > 0 {
						return fmt.Errorf("missing source volumes: %v", missingVolumes)
					}
					plan := plan.NewPlan()
					// stop services
					needToStop := make(map[components.Resource]struct{})
					for _, src := range targetComponent.Services.List() {
						liveService, err := serv.GetService(ctx, src.Service)
						if err != nil {
							return err
						}
						if liveService.State == "active" || liveService.State == "activating" {
							srcRes, err := targetComponent.Resources.Get(src.Service)
							if errors.Is(err, components.ErrResourceNotFound) {
								srcRes = components.Resource{
									Path:   src.Service,
									Parent: targetComponent.Name,
									Kind:   components.ResourceTypeService,
								}
							} else if err != nil {
								return err
							}
							needToStop[srcRes] = struct{}{}
						}
					}
					for _, quadlet := range targetComponent.Resources.List() {
						if quadlet.Kind == components.ResourceTypeContainer || quadlet.Kind == components.ResourceTypePod {
							liveService, err := serv.GetService(ctx, quadlet.Service())
							if err != nil {
								return err
							}
							if liveService.State == "active" || liveService.State == "activating" {
								needToStop[quadlet] = struct{}{}
							}
						}
					}
					ntsKeys := make([]components.Resource, 0, len(needToStop))
					for r := range needToStop {
						ntsKeys = append(ntsKeys, r)
					}
					for _, k := range ntsKeys {
						err = plan.Add(actions.Action{
							Todo:     actions.ActionStop,
							Parent:   targetComponent,
							Target:   k,
							Priority: 1,
						})
						if err != nil {
							return err
						}
						err = plan.Add(actions.Action{
							Todo:     actions.ActionStop,
							Parent:   targetComponent,
							Target:   k,
							Priority: 3,
						})
						if err != nil {
							return err
						}
					}

					// import volumes
					for vol, src := range volmap {
						importAction := actions.Action{
							Todo:     actions.ActionImport,
							Parent:   targetComponent,
							Target:   vol,
							Priority: 2,
							Metadata: &actions.ActionMetadata{
								VolumeName: &src,
							},
						}
						fmt.Fprintf(os.Stderr, "FBLTHP[631]: main.go:422: importAction=%+v\n", *importAction.Metadata.VolumeName)

						err = plan.Add(importAction)
						if err != nil {
							return err
						}
					}

					if c.Bool("plan") {
						fmt.Println(plan.Pretty())
						return nil
					}

					writer := &athanor.Writer{
						ServiceManager:   *serv,
						ContainerManager: conman,
					}

					doit := executor.NewExecutor(executor.ExecutorConfig{
						MateriaDir: cfg.DataDir,
						QuadletDir: cfg.QuadletDir,
						OutputDir:  cfg.OutputDir,
					}, writer, 90)
					log.Info("restoring", "component", targetComponent.Name)
					steps, err := doit.Execute(ctx, plan)
					if err != nil {
						return err
					}
					log.Info("Restore succesful", "steps completed", steps)
					return nil
				},
			},
			{
				Name:  "notify",
				Usage: "Send notifications",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "success",
						Aliases: []string{"s"},
						Usage:   "Send backup successful notifications",
					},
					&cli.BoolFlag{
						Name:    "failure",
						Aliases: []string{"f"},
						Usage:   "Send backup failure notifications",
					},

					&cli.BoolFlag{
						Name:    "heartbeat",
						Aliases: []string{"h"},
						Usage:   "Send heartbeast notification (i.e. no attached message, just a POST)",
					},
					&cli.StringFlag{
						Name:    "webhook",
						Aliases: []string{"w"},
						Usage:   "Webhook to use instead of configured one",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					if cfg.Webhook == "" && c.String("webhook") == "" {
						return errors.New("need webhook location defined for notify")
					}
					if !c.Bool("success") && !c.Bool("failure") && !c.Bool("heartbeat") {
						return errors.New("need notification type")
					}
					dest := cfg.Webhook
					if dest == "" {
						dest = c.String("webhook")
					}
					hostname, err := os.Hostname()
					if err != nil {
						return fmt.Errorf("error getting hostname: %w", err)
					}
					if c.Bool("success") {
						err = athanor.Notify(ctx, dest, hostname, "Backup was succesful")
					} else if c.Bool("failure") {
						err = athanor.Notify(ctx, dest, hostname, "Backup failed!")
					} else {
						err = athanor.Notify(ctx, dest, hostname, "")
					}
					return err
				},
			},
		},
	}
	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func validateConfig(cfg *athanor.Config) error {
	if cfg.QuadletDir == "" {
		return fmt.Errorf("need quadlet directory set")
	}

	if cfg.DataDir == "" {
		return fmt.Errorf("need data directory set")
	}
	if cfg.OutputDir == "" {
		return fmt.Errorf("need output directory set")
	}
	if _, err := os.Stat(cfg.QuadletDir); err != nil {
		return fmt.Errorf("could not verify quadlet directory: %w", err)
	}
	if _, err := os.Stat(cfg.DataDir); err != nil {
		return fmt.Errorf("could not verify data directory: %w", err)
	}
	if _, err := os.Stat(cfg.OutputDir); err != nil {
		return fmt.Errorf("could not verify output directory: %w", err)
	}
	return nil
}
