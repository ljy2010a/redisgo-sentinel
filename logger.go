package sentinel

import (
	"fmt"
	"io"
	"log"
	"os"
)

type LogLevel int

const (
	LOG_DEBUG LogLevel = iota
	LOG_INFO
	LOG_WARNING
	LOG_ERR
	// LOG_OFF
	// LOG_UNKNOWN
)

// default log options
const (
	DEFAULT_LOG_PREFIX = "[redis-sentinel]"
	DEFAULT_LOG_FLAG   = log.Ldate | log.Ltime
	DEFAULT_LOG_LEVEL  = LOG_DEBUG
)

type SimpleLogger struct {
	DEBUG *log.Logger
	ERR   *log.Logger
	INFO  *log.Logger
	WARN  *log.Logger
	level LogLevel
}

var logger *SimpleLogger

func init() {
	logger = NewSimpleLogger(
		os.Stdout,
		DEFAULT_LOG_PREFIX,
		DEFAULT_LOG_FLAG,
		DEFAULT_LOG_LEVEL,
	)
	logger.Info("sentinal init ")
}

func NewSimpleLogger(out io.Writer, prefix string, flag int, l LogLevel) *SimpleLogger {
	return &SimpleLogger{
		DEBUG: log.New(out, fmt.Sprintf("%s [debug] ", prefix), flag),
		ERR:   log.New(out, fmt.Sprintf("%s [error] ", prefix), flag),
		INFO:  log.New(out, fmt.Sprintf("%s [info]  ", prefix), flag),
		WARN:  log.New(out, fmt.Sprintf("%s [warn]  ", prefix), flag),
		level: l,
	}
}

func (s *SimpleLogger) Error(v ...interface{}) {
	if s.level <= LOG_ERR {
		s.ERR.Output(2, fmt.Sprint(v...))
	}
	return
}

func (s *SimpleLogger) Errorf(format string, v ...interface{}) {
	if s.level <= LOG_ERR {
		s.ERR.Output(2, fmt.Sprintf(format, v...))
	}
	return
}

func (s *SimpleLogger) Debug(v ...interface{}) {
	if s.level <= LOG_DEBUG {
		s.DEBUG.Output(2, fmt.Sprint(v...))
	}
	return
}

func (s *SimpleLogger) Debugf(format string, v ...interface{}) {
	if s.level <= LOG_DEBUG {
		s.DEBUG.Output(2, fmt.Sprintf(format, v...))
	}
	return
}

func (s *SimpleLogger) Info(v ...interface{}) {
	if s.level <= LOG_INFO {
		s.INFO.Output(2, fmt.Sprint(v...))
	}
	return
}

func (s *SimpleLogger) Infof(format string, v ...interface{}) {
	if s.level <= LOG_INFO {
		s.INFO.Output(2, fmt.Sprintf(format, v...))
	}
	return
}

func (s *SimpleLogger) Warn(v ...interface{}) {
	if s.level <= LOG_WARNING {
		s.WARN.Output(2, fmt.Sprint(v...))
	}
	return
}

func (s *SimpleLogger) Warnf(format string, v ...interface{}) {
	if s.level <= LOG_WARNING {
		s.WARN.Output(2, fmt.Sprintf(format, v...))
	}
	return
}

func (s *SimpleLogger) Level() LogLevel {
	return s.level
}

func (s *SimpleLogger) SetLevel(l LogLevel) {
	s.level = l
	return
}
