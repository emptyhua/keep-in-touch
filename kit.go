package kit

import (
	"github.com/emptyhua/go-logging/logging"
)

var debug = true

func SetDebug(d bool) {
	debug = d
	if debug {
		Logger.SetLevel(logging.DEBUG)
	}
}
