package logging

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// NewLogger builds a zap logger with consistent settings.
func NewLogger() logr.Logger {
	options := zap.Options{
		Development: true,
		Level:       zapcore.InfoLevel,
	}

	return zap.New(zap.UseFlagOptions(&options))
}
