package editor

import (
	"log"
)

type GonvimLogger struct {
	logger *log.Logger
}

func (l *GonvimLogger) Println(v ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Println(v...)
}

func (l *GonvimLogger) Printf(format string, v ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Printf(format, v...)
}
