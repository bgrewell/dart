package testtypes

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/bgrewell/dart/internal/config"
)

// A misspelled or misplaced option key used to be dropped in silence: the
// suite read as though the option were set while nothing consumed it. The
// most damaging form is a stray `evaluate:` — the test then runs with zero
// checks and is reported as passing.
//
// Rather than maintain a list of valid keys per test type, which drifts the
// moment a factory gains an option, the accessors record every key they read
// while a test is being constructed. Whatever the factory never asked for is
// not an option of that type.
//
// Construction is sequential, so one tracker at a time is enough; the mutex
// guards against a future caller building tests concurrently.
var (
	accessMu      sync.Mutex
	accessTracked map[string]bool
)

// beginTracking starts recording option reads and returns a function that
// stops recording and reports the keys that were never read.
func beginTracking() func(options map[string]interface{}) []string {
	accessMu.Lock()
	accessTracked = map[string]bool{}

	return func(options map[string]interface{}) []string {
		seen := accessTracked
		accessTracked = nil
		accessMu.Unlock()

		unread := make([]string, 0, len(options))
		for key := range options {
			if !seen[key] {
				unread = append(unread, key)
			}
		}
		sort.Strings(unread)
		return unread
	}
}

// acceptedKeys lists what the factory just read, for the error message. It
// must run before the tracker is released.
func acceptedKeys() []string {
	keys := make([]string, 0, len(accessTracked))
	for key := range accessTracked {
		keys = append(keys, key)
	}
	return keys
}

// noteOption records that a factory read an option key. Accessors call it;
// factories that reach into the options map directly call it themselves.
func noteOption(keys ...string) {
	if accessTracked == nil {
		return
	}
	for _, key := range keys {
		accessTracked[key] = true
	}
}

// leadingKeyRe pulls the option name off the front of a message such as
// "timeout must be positive in test %q" — the convention every option error
// in this package follows.
var leadingKeyRe = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\b`)

// locatedOptionError attaches the offending option's position to a factory
// error, so the snippet marks that option rather than the first line of the
// test. The name is taken from the message and only used when the test
// really has an option by that name; anything else keeps the test's own
// location.
func locatedOptionError(cfg *config.TestConfig, err error) error {
	var existing *config.ConfigError
	if errors.As(err, &existing) {
		return err
	}

	message := err.Error()
	match := leadingKeyRe.FindStringSubmatch(message)
	if match == nil {
		return &config.ConfigError{Message: message, Location: cfg.Loc}
	}
	loc, ok := cfg.OptionLocs[match[1]]
	if !ok {
		return &config.ConfigError{Message: message, Location: cfg.Loc}
	}
	return &config.ConfigError{Message: message, Location: loc, Key: match[1]}
}

// unknownOptionError reports unread keys against the test, listing what the
// type does accept so the fix is obvious.
func unknownOptionError(cfg *config.TestConfig, unknown, accepted []string) error {
	sort.Strings(accepted)

	article := "a"
	if cfg.Type != "" && strings.ContainsRune("aeiou", rune(cfg.Type[0])) {
		article = "an"
	}

	message := fmt.Sprintf("unknown option %q in test %q", unknown[0], cfg.Name)
	if len(accepted) > 0 {
		message += fmt.Sprintf(" (%s %s test accepts: %s)",
			article, cfg.Type, strings.Join(accepted, ", "))
	} else {
		message += fmt.Sprintf(" (%s %s test takes no options)", article, cfg.Type)
	}

	location := cfg.Loc
	if loc, ok := cfg.OptionLocs[unknown[0]]; ok {
		location = loc
	}
	return &config.ConfigError{Message: message, Location: location, Key: unknown[0]}
}
