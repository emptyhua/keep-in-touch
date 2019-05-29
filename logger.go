package kit

import (
	"github.com/emptyhua/go-logging/logging"
)

var Logger *logging.Logger = nil

func init() {
	Logger, _ = logging.SimpleLogger("kit")
}
