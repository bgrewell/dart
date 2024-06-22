package main

import (
	"context"
	"fmt"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
)

type RunParams struct {
	fx.In
	LC         fx.Lifecycle
	Shutdowner fx.Shutdowner
	Ctrl       *pkg.TestController
}

func Configuration() (*config.Configuration, error) {
	// Read in the test configuration file
	cfg, err := config.LoadConfiguration("examples/basic/basic.yaml")
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func Formatter() (formatters.Formatter, error) {
	return formatters.NewStandardFormatter(), nil
}

func Nodes(cfg *config.Configuration) (map[string]pkg.Node, error) {
	// Create nodes for testing
	nodes, err := pkg.CreateNodes(cfg.Nodes)
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

func Controller(cfg *config.Configuration, formater formatters.Formatter) (ctrl *pkg.TestController, err error) {
	// Create the test controller
	nodes, err := Nodes(cfg)
	if err != nil {
		return nil, err
	}
	tests, err := Tests(cfg, nodes)
	if err != nil {
		return nil, err
	}
	setup, err := Setup(cfg, nodes)
	if err != nil {
		return nil, err
	}
	teardown, err := Teardown(cfg, nodes)
	if err != nil {
		return nil, err
	}
	return pkg.NewTestController(cfg.Suite, nodes, tests, setup, teardown, formater), nil
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Provide(
			func() (*zap.Logger, error) {
				cfg := zap.NewProductionConfig()
				cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
				return cfg.Build()
			},
			Configuration,
			Formatter,
			Controller,
		),
		fx.Invoke(RegisterHooks),
	)

	if err := app.Start(ctx); err != nil {
		log.Fatalf("Error starting application: %v", err)
	}
	<-app.Done()

	if err := app.Stop(ctx); err != nil {
		fmt.Println("Failed to stop:", err)
		return
	}
}
