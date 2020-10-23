package kit

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger = nil
var atomicLevel zap.AtomicLevel

var debug = true

func SetDebug(d bool) {
	debug = d
	if debug {
		atomicLevel.SetLevel(zap.DebugLevel)
	}
}

func init() {
	atomicLevel = zap.NewAtomicLevel()

	// To keep the example deterministic, disable timestamps in the output.
	encoderCfg := zap.NewProductionEncoderConfig()

	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atomicLevel,
	))

	Logger = logger.Sugar()
}
