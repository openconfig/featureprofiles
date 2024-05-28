package flagUtils

import "flag"

type FlagOptions struct {
	LocalRun bool
	FirexRun bool
	NoDBRun  bool
}

var flagOptions FlagOptions

func init() {
	flag.BoolVar(&flagOptions.LocalRun, "local_run", false, "Used for local run")
	flag.BoolVar(&flagOptions.FirexRun, "firex_run", false, "Set env variables for firex run")
	flag.BoolVar(&flagOptions.NoDBRun, "no_db_run", true, "Don't upload to database")
}

// ParseFlags returns the parsed flag values
func ParseFlags() FlagOptions {
	return flagOptions
}
