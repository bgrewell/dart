package logger

import (
	"github.com/sirupsen/logrus"
	"go.uber.org/fx/fxevent"
)

func NewLogger() *LogrusLogger {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	//log.SetFormatter(&CustomFormatter{}) // Use the custom formatter
	return &LogrusLogger{
		Logger: log,
	}
}

type LogrusLogger struct {
	Logger *logrus.Logger
}

func (l *LogrusLogger) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		l.Logger.WithFields(logrus.Fields{
			"callee": e.FunctionName,
			"caller": e.CallerName,
		}).Info("Executing OnStart hook")
	case *fxevent.OnStartExecuted:
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"callee": e.FunctionName,
				"caller": e.CallerName,
				"error":  e.Err,
			}).Error("OnStart hook failed")
		} else {
			l.Logger.WithFields(logrus.Fields{
				"callee":  e.FunctionName,
				"caller":  e.CallerName,
				"runtime": e.Runtime,
			}).Info("Executed OnStart hook")
		}
	case *fxevent.OnStopExecuting:
		l.Logger.WithFields(logrus.Fields{
			"callee": e.FunctionName,
			"caller": e.CallerName,
		}).Info("Executing OnStop hook")
	case *fxevent.OnStopExecuted:
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"callee": e.FunctionName,
				"caller": e.CallerName,
				"error":  e.Err,
			}).Error("OnStop hook failed")
		} else {
			l.Logger.WithFields(logrus.Fields{
				"callee":  e.FunctionName,
				"caller":  e.CallerName,
				"runtime": e.Runtime,
			}).Info("Executed OnStop hook")
		}
	case *fxevent.Supplied:
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"type":  e.TypeName,
				"error": e.Err,
			}).Error("Error supplying")
		} else {
			l.Logger.WithFields(logrus.Fields{
				"type": e.TypeName,
			}).Info("Supplied")
		}
	case *fxevent.Provided:
		for _, rtype := range e.OutputTypeNames {
			l.Logger.WithFields(logrus.Fields{
				"constructor": e.ConstructorName,
				"type":        rtype,
			}).Info("Provided")
		}
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"constructor": e.ConstructorName,
				"error":       e.Err,
			}).Error("Error providing")
		}
	case *fxevent.Invoking:
		l.Logger.WithFields(logrus.Fields{
			"function": e.FunctionName,
		}).Info("Invoking function")
	case *fxevent.Invoked:
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"function": e.FunctionName,
				"error":    e.Err,
			}).Error("Error invoked")
		} else {
			l.Logger.WithFields(logrus.Fields{
				"function": e.FunctionName,
			}).Info("Function invoked")
		}
	case *fxevent.Stopping:
		l.Logger.WithFields(logrus.Fields{
			"signal": e.Signal,
		}).Info("Stopping application")
	case *fxevent.Stopped:
		if e.Err != nil {
			l.Logger.WithFields(logrus.Fields{
				"error": e.Err,
			}).Error("Error stopping application")
		} else {
			l.Logger.Info("Application stopped")
		}
	}
}
