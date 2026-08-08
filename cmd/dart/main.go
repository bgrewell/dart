package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bgrewell/dart/internal"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/logger"
	"github.com/bgrewell/dart/internal/lxd"
	"github.com/bgrewell/dart/internal/report"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/bgrewell/dart/pkg/steptypes"
	"github.com/bgrewell/dart/pkg/testtypes"
	"github.com/bgrewell/usage"
	"github.com/fatih/color"
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
	ConfigFile    *string
	Verbose       *bool
	Debug         *bool
	StopOnError   *bool
	PauseOnError  *bool
	SetupOnly     *bool
	TeardownOnly  *bool
	Iterations    *int
	Until         *string
	UntilBehavior *string
	Report        *string
	Check         *bool
	Version       *bool
	LogFile       *string
	Vars          *string
	Only          *string
	SkipTags      *string
	Color         *string
}

type ControllerParams struct {
	fx.In
	Cfg           *config.Configuration
	Nodes         map[string]ifaces.Node
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
	vars, err := parseVarFlags(*cmdFlags.Vars)
	if err != nil {
		return nil, err
	}
	// Read in the test configuration file
	cfg, err := config.LoadConfigurationWithVars(*cmdFlags.ConfigFile, vars)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseVarFlags parses --vars "key=value,key2=value2" overrides.
func parseVarFlags(value string) (map[string]string, error) {
	if value == "" {
		return nil, nil
	}
	vars := map[string]string{}
	for _, pair := range strings.Split(value, ",") {
		key, val, found := strings.Cut(strings.TrimSpace(pair), "=")
		if !found || key == "" {
			return nil, fmt.Errorf("--vars entries must be key=value (got %q)", pair)
		}
		vars[key] = val
	}
	return vars, nil
}

// parseTagFilter parses a --only/--skip value of the form "tag=a,b".
func parseTagFilter(flagName, value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	rest, found := strings.CutPrefix(value, "tag=")
	if !found || rest == "" {
		return nil, fmt.Errorf("--%s must be tag=<name>[,<name>...] (got %q)", flagName, value)
	}
	var tags []string
	for _, tag := range strings.Split(rest, ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("--%s must name at least one tag (got %q)", flagName, value)
	}
	return tags, nil
}

// logCleanup flushes and closes the --log file; os.Exit skips defers, so
// exit paths call it explicitly.
var logCleanup = func() {}

func Formatter(cmdFlags *CmdlineFlags) (formatters.Formatter, error) {
	if *cmdFlags.LogFile != "" {
		file, err := os.Create(*cmdFlags.LogFile)
		if err != nil {
			return nil, fmt.Errorf("cannot open log file: %w", err)
		}
		logWriter := formatters.NewCleanLogWriter(file)
		logCleanup = func() {
			logWriter.Flush()
			file.Close()
		}
		// Debug-streamed command output must reach the transcript too
		stream.GetCoordinator().SetWriters(
			io.MultiWriter(os.Stdout, logWriter),
			io.MultiWriter(os.Stderr, logWriter))
		return formatters.NewStandardFormatterWithWriter(
			io.MultiWriter(os.Stdout, logWriter)), nil
	}
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
	// Build the list of platform managers
	var platforms []ifaces.PlatformManager
	if params.DockerWrapper != nil {
		platforms = append(platforms, params.DockerWrapper)
	}
	if params.LxdWrapper != nil {
		platforms = append(platforms, params.LxdWrapper)
	}

	// Create the test controller with raw configs; steps/tests are created
	// inside Run() after nodes are set up and facts are gathered.
	controller := internal.NewTestController(
		params.Cfg.Suite,
		platforms,
		params.Nodes,
		params.Cfg.Nodes,
		params.Cfg.Setup,
		params.Cfg.Teardown,
		params.Cfg.Tests,
		*params.Flags.Verbose,
		*params.Flags.Debug,
		*params.Flags.StopOnError,
		*params.Flags.PauseOnError,
		*params.Flags.SetupOnly,
		*params.Flags.TeardownOnly,
		*params.Flags.Until,
		*params.Flags.UntilBehavior,
		params.Formatter)

	specs, err := parseReportSpecs(*params.Flags.Report)
	if err != nil {
		return nil, err
	}
	controller.SetReports(specs)

	onlyTags, err := parseTagFilter("only", *params.Flags.Only)
	if err != nil {
		return nil, err
	}
	skipTags, err := parseTagFilter("skip", *params.Flags.SkipTags)
	if err != nil {
		return nil, err
	}
	controller.SetTagFilters(onlyTags, skipTags)
	return controller, nil
}

// parseReportSpecs parses the comma-separated --report value.
func parseReportSpecs(value string) ([]report.Spec, error) {
	if value == "" {
		return nil, nil
	}
	var specs []report.Spec
	for _, item := range strings.Split(value, ",") {
		spec, err := report.ParseSpec(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

var errorStyle = color.New(color.FgRed, color.Bold)

func RegisterHooks(params RunParams) {
	params.LC.Append(fx.Hook{
		OnStart: func(context context.Context) error {
			iterations := 1
			if params.Flags.Iterations != nil {
				iterations = *params.Flags.Iterations
			}
			var lastErr error
			for i := 0; i < iterations; i++ {
				if iterations > 1 {
					// Per-iteration report files: a passing final iteration
					// must not overwrite an earlier failure
					params.Ctrl.SetReportIteration(i + 1)
				}
				err := params.Ctrl.Run()
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), lastErr)
				return params.Shutdowner.Shutdown(fx.ExitCode(1))
			}
			return params.Shutdowner.Shutdown()
		},
		OnStop: func(context context.Context) error {
			// Synchronous: a goroutine here raced process exit, so node
			// connections could die unflushed
			if err := params.Ctrl.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error closing nodes: %v\n", err)
			}
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
	cfgFlags.Until = u.AddStringOption("u", "until", "", "Run up to and including this step or test, then stop", "", nil)
	cfgFlags.UntilBehavior = u.AddStringOption("ub", "until-behavior", "exit", "Behavior when --until target is reached: exit (default) or pause", "", nil)
	cfgFlags.Report = u.AddStringOption("r", "report", "", "Write machine-readable results: format:path (junit:results.xml, json:results.json; comma-separate for both)", "", nil)
	cfgFlags.Version = u.AddBooleanOption("V", "version", false, "Print version information and exit", "", nil)
	cfgFlags.Check = u.AddBooleanOption("ck", "check", false, "Validate the configuration and print the plan without running anything", "", nil)
	cfgFlags.LogFile = u.AddStringOption("l", "log", "", "Write a clean (color-free) transcript of the run to this file", "", nil)
	cfgFlags.Vars = u.AddStringOption("var", "vars", "", "Override suite variables: key=value[,key=value...]", "", nil)
	cfgFlags.Only = u.AddStringOption("o", "only", "", "Run only tests carrying one of these tags: tag=name[,name...]", "", nil)
	cfgFlags.SkipTags = u.AddStringOption("sk", "skip", "", "Exclude tests carrying any of these tags: tag=name[,name...]", "", nil)
	cfgFlags.Color = u.AddStringOption("co", "color", "auto", "Colorize output: auto (a terminal), always, or never", "", nil)

	if !u.Parse() {
		u.PrintError(fmt.Errorf("Failed to parse command line arguments"))
	}

	// Colour is resolved before anything can print, so an error rendered on
	// the way out of parsing already honours the setting
	switch *cfgFlags.Color {
	case "auto":
		// The color package disables itself when stdout is not a terminal;
		// NO_COLOR is honoured here because it is the cross-tool convention
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			color.NoColor = true
		}
	case "always":
		color.NoColor = false
	case "never":
		color.NoColor = true
	default:
		fmt.Fprintf(os.Stderr, "\n%s color must be auto, always, or never (got %q)\n\n",
			errorStyle.Sprint("Error:"), *cfgFlags.Color)
		os.Exit(1)
	}

	// Validate flag values that would otherwise fail silently: zero
	// iterations would exit green having run nothing, and a typo'd
	// until-behavior would silently mean "exit"
	if *cfgFlags.Iterations < 1 {
		fmt.Fprintf(os.Stderr, "\n%s iterations must be at least 1 (got %d)\n\n", errorStyle.Sprint("Error:"), *cfgFlags.Iterations)
		os.Exit(1)
	}
	if *cfgFlags.UntilBehavior != "exit" && *cfgFlags.UntilBehavior != "pause" {
		fmt.Fprintf(os.Stderr, "\n%s until-behavior must be \"exit\" or \"pause\" (got %q)\n\n", errorStyle.Sprint("Error:"), *cfgFlags.UntilBehavior)
		os.Exit(1)
	}

	if *cfgFlags.Version {
		fmt.Printf("dart %s (%s, branch %s, built %s)\n", version, rev, branch, date)
		os.Exit(0)
	}

	if *cfgFlags.Check {
		os.Exit(runCheck(*cfgFlags.ConfigFile, *cfgFlags.Report, *cfgFlags.Vars, *cfgFlags.Only, *cfgFlags.SkipTags))
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
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), rootErr)
		logCleanup()
		os.Exit(1)
	}

	shutdownSig := <-app.Wait()

	if err := app.Stop(ctx); err != nil {
		log.Errorf("Failed to stop: %v", err)
	}

	logCleanup()

	// Propagate the exit code so that if any tests failed we return a non-zero exit code
	// This is useful for CI/CD pipelines or other tools that expect a non-zero exit code on failure
	os.Exit(shutdownSig.ExitCode)
}

// runCheck validates the configuration — full option parsing for every
// node, step, and test — and prints the plan without running anything.
func runCheck(cfgPath, reportValue, varsValue, onlyValue, skipValue string) int {
	// Validate flags the run would reject, so --check green means the real
	// invocation starts
	if _, err := parseReportSpecs(reportValue); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		return 1
	}
	if _, err := parseTagFilter("only", onlyValue); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		return 1
	}
	if _, err := parseTagFilter("skip", skipValue); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		return 1
	}
	vars, err := parseVarFlags(varsValue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		return 1
	}

	cfg, err := config.LoadConfigurationWithVars(cfgPath, vars)
	if err != nil {
		var cfgErr *config.ConfigError
		if errors.As(err, &cfgErr) {
			fmt.Fprint(os.Stderr, config.RenderConfigError(cfgErr))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		}
		return 1
	}

	// Constraints across the whole node list — duplicate names, more than
	// one local node — are the same ones a real run enforces
	if err := nodetypes.ValidateNodeSet(cfg.Nodes); err != nil {
		var cfgErr *config.ConfigError
		if errors.As(err, &cfgErr) {
			fmt.Fprint(os.Stderr, config.RenderConfigError(cfgErr))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s %s\n\n", errorStyle.Sprint("Error:"), err)
		}
		return 1
	}

	mocks := make(map[string]ifaces.Node, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		// Unknown node types must fail --check exactly as they fail a run
		if !nodetypes.IsKnownNodeType(node.Type) {
			fmt.Fprint(os.Stderr, config.RenderConfigError(&config.ConfigError{
				Message:  fmt.Sprintf("unknown node type %q", node.Type),
				Location: node.TypeLoc,
			}))
			return 1
		}
		// Option shapes that need no connection are checked here, so a
		// green --check means the real run gets past construction
		if err := nodetypes.ValidateNodeOptions(node); err != nil {
			fmt.Fprint(os.Stderr, config.RenderConfigError(&config.ConfigError{
				Message:  fmt.Sprintf("node %q: %v", node.Name, err),
				Location: node.Loc,
			}))
			return 1
		}
		mocks[node.Name] = nodetypes.NewCheckNode(node.Type)
	}

	fail := func(stage string, err error) int {
		var cfgErr *config.ConfigError
		if errors.As(err, &cfgErr) {
			fmt.Fprint(os.Stderr, config.RenderConfigError(cfgErr))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s %s: %s\n\n", errorStyle.Sprint("Error:"), stage, err)
		}
		return 1
	}

	setup, err := steptypes.CreateSteps(cfg.Setup, mocks)
	if err != nil {
		return fail("setup", err)
	}
	teardown, err := steptypes.CreateSteps(cfg.Teardown, mocks)
	if err != nil {
		return fail("teardown", err)
	}
	tests, err := testtypes.CreateTests(cfg.Tests, mocks)
	if err != nil {
		return fail("tests", err)
	}

	fmt.Printf("Suite: %s\n", cfg.Suite)
	fmt.Printf("Nodes: %d\n", len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		fmt.Printf("  - %s (%s)\n", node.Name, node.Type)
	}
	fmt.Printf("Setup steps: %d, Tests: %d, Teardown steps: %d\n", len(setup), len(tests), len(teardown))
	fmt.Println("Configuration valid.")
	return 0
}
