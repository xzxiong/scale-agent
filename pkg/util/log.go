package util

import (
	"github.com/go-logr/logr"
)

var logger logr.Logger

func SetLogger(l logr.Logger) {
	logger = l
}

func GetLogger() logr.Logger {
	return logger
}
