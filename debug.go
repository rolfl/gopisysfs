package gopisysfs

import ()

// LogFunction declares a signature that can be used for this library to log information to.
// Set a log function by calling SetLogFn(...).
type LogFunction func(format string, args ...interface{})

// The log function we use for logging, may be nil.
var logfn LogFunction

// SetLogFn instructs this library to use the specified function to send log messages to.
// Set to nil to disable loggin.
// For example `gopisysfs.SetLogFn(log.Printf)` (but note that trace details will be wrong in the log library with that call).
func SetLogFn(lfn LogFunction) {
	logfn = lfn
}

// info is internally used to log details.
func info(format string, args ...interface{}) {
	lfn := logfn
	if lfn == nil {
		return
	}
	lfn(format, args...)
}
