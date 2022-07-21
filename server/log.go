package server

import (
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", log.Lshortfile|log.LstdFlags)

const (
	isLogger = true
)

func logf(f string, v ...interface{}) {
	if isLogger {
		logger.Output(2, fmt.Sprintf(f, v...))
	}
}

type logHelper struct {
	prefix string
}

func (l *logHelper) Write(p []byte) (n int, err error) {
	if isLogger {
		logger.Printf("%s%s\n", l.prefix, p)
		return len(p), nil
	}
	return len(p), nil
}

func newLogHelper(prefix string) *logHelper {
	return &logHelper{prefix}
}
