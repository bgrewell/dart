package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bgrewell/dart/internal"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/logger"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/bgrewell/dart/pkg/steptypes"
	"github.com/bgrewell/dart/pkg/testtypes"
	"github.com/bgrewell/usage"
	"go.uber.org/dig"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

var (
	version = "dev"
	date    = "dev"
	rev     = "dev"
	branch  = "dev"
)

type CmdlineFlags struct {
	ConfigFile   *string
	Verbose      *bool
	Debug        *bool
	StopOnError  *bool
	PauseOnError *bool
	SetupOnly    *bool
	TeardownOnly *bool
	Iterations   *int
}

type ControllerParams struct {
	fx.In
	Cfg           *config.Configuration
	Nodes         map[string]ifaces.Node
	Tests         []ifaces.Test
	DockerWrapper *docker.Wrapper `optional:"true"`
	LxdWrapper    *lxd.Wrapper    `optional:"true"`
	Formatter     formatters.Formatter
	Flags         *CmdlineFlags
}

type RunParams struct {
	fx.In
	LC         fx.Lifecycle
	Shutdowner fx.Shutdowner
	Ctrl       *internal.TestController
	Flags      *CmdlineFlags
}

func Configuration(cmdFlags *CmdlineFlags) (*config.Configuration, error) {
	// Read in the test configuration file
	cfg, err := config.LoadConfiguration(*cmdFlags.ConfigFile)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func Formatter() (formatters.Formatter, error) {
	return formatters.NewStandardFormatter(), nil
}

func Nodes(cfg *config.Configuration, dockerWrapper *docker.Wrapper, lxdWrapper *lxd.Wrapper) (map[string]ifaces.Node, error) {
	// Create nodes for testing using both Docker and LXD wrappers
	nodes, err := nodetypes.CreateNodesWithWrappers(cfg.Nodes, dockerWrapper, lxdWrapper)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func Tests(cfg *config.Configuration, nodes map[string]ifaces.Node) (tests []ifaces.Test, err error) {
	// Create the tests
	tests, err = testtypes.CreateTests(cfg.Tests, nodes)
	if err != nil {
		return nil, err
	}
	return tests, nil
}

func Setup(cfg *config.Configuration, nodes map[string]ifaces.Node) (setup []ifaces.Step, err error) {
	// Create the steps
	setup, err = steptypes.CreateSteps(cfg.Setup, nodes)
	if err != nil {
		return nil, err
	}
	return setup, nil
}

func Teardown(cfg *config.Configuration, nodes map[string]ifaces.Node) (teardown []ifaces.Step, err error) {
	// Create the steps
	teardown, err = steptypes.CreateSteps(cfg.Teardown, nodes)
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

func LxdWrapper(cfg *config.Configuration) (*lxd.Wrapper, error) {
	// Only create LXD wrapper if LXD is configured
	if cfg.Lxd == nil {
		return nil, nil
	}
	// Create the LXD wrapper
	lw, err := lxd.NewWrapper(cfg.Lxd)
	if err != nil {
		// LXD might not be available on the system, which is fine
		// Just return nil and let nodes handle it individually
		return nil, nil
	}
	return lw, nil
}

func Controller(params ControllerParams) (ctrl *internal.TestController, err error) {
	// TODO: Setup and Teardown are called here because of an issue passing them in the params (not being called in the proper order)
	setup, err := Setup(params.Cfg, params.Nodes)
	if err != nil {
		return nil, err
	}

	teardown, err := Teardown(params.Cfg, params.Nodes)
	if err != nil {
		return nil, err
	}

	// Build the list of platform managers
	var platforms []ifaces.PlatformManager
	if params.DockerWrapper != nil {
		platforms = append(platforms, params.DockerWrapper)
	}
	if params.LxdWrapper != nil {
		platforms = append(platforms, params.LxdWrapper)
	}

	// Create the test controller
	return internal.NewTestController(
		params.Cfg.Suite,
		platforms,
		params.Nodes,
		params.Tests,
		setup,
		teardown,
		*params.Flags.Verbose,
		*params.Flags.Debug,
		*params.Flags.StopOnError,
		*params.Flags.PauseOnError,
		*params.Flags.SetupOnly,
		*params.Flags.TeardownOnly,
		params.Formatter), nil
}

func RegisterHooks(params RunParams) {
	params.LC.Append(fx.Hook{
		OnStart: func(context context.Context) error {
			iterations := 1
			if params.Flags.Iterations != nil {
				iterations = *params.Flags.Iterations
			}
			var lastErr error
			for i := 0; i < iterations; i++ {
				err := params.Ctrl.Run()
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				return params.Shutdowner.Shutdown(fx.ExitCode(1))
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
		usage.WithApplicationVersion(version),
		usage.WithApplicationBuildDate(date),
		usage.WithApplicationCommitHash(rev),
		usage.WithApplicationBranch(branch),
		usage.WithApplicationDescription("DART is a distributed systems testing framework designed to make it easy to perform automation and integration testing on a wide variety of distributed systems."),
	)

	cfgFlags := &CmdlineFlags{}
	cfgFlags.ConfigFile = u.AddStringOption("c", "config", "config.yaml", "The path to the configuration file", "", nil)
	cfgFlags.Verbose = u.AddBooleanOption("v", "verbose", false, "Enable verbose output", "", nil)
	cfgFlags.Debug = u.AddBooleanOption("d", "debug", false, "Enable real-time streaming of command output", "", nil)
	cfgFlags.PauseOnError = u.AddBooleanOption("p", "pause-on-error", false, "Pause on error", "", nil)
	cfgFlags.StopOnError = u.AddBooleanOption("s", "stop-on-error", false, "Stop on error", "", nil)
	cfgFlags.SetupOnly = u.AddBooleanOption("setup", "setup-only", false, "Only run the setup steps", "", nil)
	cfgFlags.TeardownOnly = u.AddBooleanOption("teardown", "teardown-only", false, "Only run the teardown steps", "", nil)
	cfgFlags.Iterations = u.AddIntegerOption("i", "iterations", 1, "Number of iterations to run", "", nil)

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
			LxdWrapper,
			Configuration,
			Formatter,
			Controller,
		),
		fx.Invoke(RegisterHooks),
	)

	if err := app.Start(ctx); err != nil {
		rootErr := dig.RootCause(err)
		var cfgErr *config.ConfigError
		if errors.As(rootErr, &cfgErr) {
			fmt.Fprint(os.Stderr, config.RenderConfigError(cfgErr))
			os.Exit(1)
		}
		log.Fatalf("Failed to start: %v", err)
	}

	shutdownSig := <-app.Wait()

	if err := app.Stop(ctx); err != nil {
		log.Errorf("Failed to stop: %v", err)
	}

	// Propagate the exit code so that if any tests failed we return a non-zero exit code
	// This is useful for CI/CD pipelines or other tools that expect a non-zero exit code on failure
	os.Exit(shutdownSig.ExitCode)
}
