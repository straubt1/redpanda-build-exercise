package applog

import (
	"log"
	"os"
)

// Go's default logger writes to stderr. ArcBox (and docker log split) treat that as errors.
var (
	Info = log.New(os.Stdout, "", log.LstdFlags)
	Err  = log.New(os.Stderr, "", log.LstdFlags)
)
