package main

import (
	"context"
	"fmt"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/logger"
	"github.com/bgrewell/dart/pkg"
	"github.com/bgrewell/usage"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

type CmdlineFlags struct {
	ConfigFile   *string
	Verbose      *bool
	StopOnError  *bool
	PauseOnError *bool
	SetupOnly    *bool
	TeardownOnly *bool
}

type ControllerParams struct {
	fx.In
	Cfg   *config.Configuration
	Nodes map[string]pkg.Node
	Tests []pkg.Test
	//Setup     []pkg.Step `group:"setup"`
	//Teardown  []pkg.Step `group:"teardown"`
	Wrapper   *docker.Wrapper
	Formatter formatters.Formatter
	Flags     *CmdlineFlags
}

type RunParams struct {
	fx.In
	LC         fx.Lifecycle
	Shutdowner fx.Shutdowner
	Ctrl       *pkg.TestController
}

func Configuration(cmdFlags *CmdlineFlags) (*config.Configuration, error) {
	// Read in the test configuration file
	//cfg, err := config.LoadConfiguration("examples/basic/basic.yaml")
	//cfg, err := config.LoadConfiguration("examples/docker/docker.yaml")
	cfg, err := config.LoadConfiguration(*cmdFlags.ConfigFile)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func Formatter() (formatters.Formatter, error) {
	return formatters.NewStandardFormatter(), nil
}

func Nodes(cfg *config.Configuration, wrapper *docker.Wrapper) (map[string]pkg.Node, error) {
	// Create nodes for testing
	nodes, err := pkg.CreateNodes(cfg.Nodes, wrapper)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func Tests(cfg *config.Configuration, nodes map[string]pkg.Node) (tests []pkg.Test, err error) {
	// Create the tests
	tests, err = pkg.CreateTests(cfg.Tests, nodes)
	if err != nil {
		return nil, err
	}
	return tests, nil
}

func Setup(cfg *config.Configuration, nodes map[string]pkg.Node) (setup []pkg.Step, err error) {
	// Create the steps
	setup, err = pkg.CreateSteps(cfg.Setup, nodes)
	if err != nil {
		return nil, err
	}
	return setup, nil
}

func Teardown(cfg *config.Configuration, nodes map[string]pkg.Node) (teardown []pkg.Step, err error) {
	// Create the steps
	teardown, err = pkg.CreateSteps(cfg.Teardown, nodes)
	if err != nil {
		return nil, err
	}
	return teardown, nil
}

func DockerWrapper(cfg *config.Configuration) (*docker.Wrapper, error) {
	// Create the Docker wrapper
	dw, err := docker.NewWrapper(cfg)
	if err != nil {
		return nil, err
	}
	return dw, nil
}

func Controller(params ControllerParams) (ctrl *pkg.TestController, err error) {
	// TODO: Setup and Teardown are called here because of an issue passing them in the params (not being called in the proper order)
	setup, err := Setup(params.Cfg, params.Nodes)
	if err != nil {
		return nil, err
	}

	teardown, err := Teardown(params.Cfg, params.Nodes)
	if err != nil {
		return nil, err
	}

	// Create the test controller
	return pkg.NewTestController(
		params.Cfg.Suite,
		params.Wrapper,
		params.Nodes,
		params.Tests,
		setup,
		teardown,
		*params.Flags.Verbose,
		*params.Flags.StopOnError,
		*params.Flags.PauseOnError,
		*params.Flags.SetupOnly,
		*params.Flags.TeardownOnly,
		//params.Setup,
		//params.Teardown,
		params.Formatter), nil
}

func RegisterHooks(params RunParams) {
	params.LC.Append(fx.Hook{
		OnStart: func(context context.Context) error {
			// TODO: Cleanup the mess here
			err := params.Ctrl.Run()
			if err != nil {
				params.Shutdowner.Shutdown(fx.ExitCode(1))
			}
			return params.Shutdowner.Shutdown()
		},
		OnStop: func(context context.Context) error {
			go params.Ctrl.Close()
			return nil
		},
	})
}

func main() {

	u := usage.NewUsage(
		usage.WithApplicationName("dart"),
		usage.WithApplicationVersion("dev"),
		usage.WithApplicationBuildDate("dev"),
		usage.WithApplicationCommitHash("dev"),
		usage.WithApplicationBranch("dev"),
		usage.WithApplicationDescription("DART is a distributed systems testing framework designed to make it easy to perform automation and integration testing on a wide variety of distributed systems."),
	)

	cfgFlags := &CmdlineFlags{}
	cfgFlags.ConfigFile = u.AddStringOption("c", "config", "config.yaml", "The path to the configuration file", "", nil)
	cfgFlags.Verbose = u.AddBooleanOption("v", "verbose", false, "Enable verbose output", "", nil)
	cfgFlags.PauseOnError = u.AddBooleanOption("p", "pause-on-error", false, "Pause on error", "", nil)
	cfgFlags.StopOnError = u.AddBooleanOption("s", "stop-on-error", false, "Stop on error", "", nil)
	cfgFlags.SetupOnly = u.AddBooleanOption("setup", "setup-only", false, "Only run the setup steps", "", nil)
	cfgFlags.TeardownOnly = u.AddBooleanOption("teardown", "teardown-only", false, "Only run the teardown steps", "", nil)

	if !u.Parse() {
		u.PrintError(fmt.Errorf("Failed to parse command line arguments"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.NewLogger().Logger

	app := fx.New(
		fx.WithLogger(func() fxevent.Logger {
			return logger.NewLogger()
		}),
		fx.Provide(
			func() *CmdlineFlags {
				return cfgFlags
			},
			Nodes,
			Tests,
			fx.Annotate(
				Setup,
				fx.ResultTags(`group:"setup"`),
			),
			fx.Annotate(
				Teardown,
				fx.ResultTags(`group:"teardown"`),
			),
			DockerWrapper,
			Configuration,
			Formatter,
			Controller,
		),
		fx.Invoke(RegisterHooks),
	)

	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
	<-app.Done()

	if err := app.Stop(ctx); err != nil {
		log.Errorf("Failed to stop: %v", err)
		return
	}
}
