package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg := zap.Config{Level: zap.NewAtomicLevelAt(zap.InfoLevel), Development: false, Encoding: "json", EncoderConfig: encCfg, OutputPaths: []string{"stdout"}, ErrorOutputPaths: []string{"stderr"}}
	if os.Getenv("APP_ENV") == "development" {
		cfg = zap.NewDevelopmentConfig()
	}
	l, _ := cfg.Build()
	return l
}
