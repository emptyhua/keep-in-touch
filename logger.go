package kit

import (
	"github.com/ccding/go-logging/logging"
)

var Logger *logging.Logger = nil

func init() {
	Logger, _ = logging.SimpleLogger("kit")
}
