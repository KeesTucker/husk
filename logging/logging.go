package logging

import (
	"context"
	"time"

	"husk/services"
)

type Logger struct {
	bufferedProtocolLog []string
	bufferedCanbusLog   []string
	bufferedLog         []string
}

type LogSubFunc func(message string, logType LogType)

type LogType int

const (
	LogTypeProtocolLog LogType = iota
	LogTypeCanbusLog
	LogTypeLog
)

const (
	logRefreshRate = 64
)

var (
	logRefreshDelay = time.Duration((1.0 / logRefreshRate) * float64(time.Second))
	logSubscribers  []LogSubFunc
)

func RegisterLogger() *Logger {
	l := &Logger{}
	defer services.Register(services.ServiceLogger, l)
	return l
}

func (l *Logger) Start(ctx context.Context) *Logger {
	go l.displayLogLoop(ctx)
	return l
}

func (l *Logger) AddLogSub(subFunc LogSubFunc) *Logger {
	logSubscribers = append(logSubscribers, subFunc)
	return l
}

// WriteToLog writes to the appropriate log buffer
func (l *Logger) WriteToLog(message string, logType LogType) {
	switch logType {
	case LogTypeProtocolLog:
		l.bufferedProtocolLog = append(l.bufferedProtocolLog, message)
	case LogTypeCanbusLog:
		l.bufferedCanbusLog = append(l.bufferedCanbusLog, message)
	case LogTypeLog:
		l.bufferedLog = append(l.bufferedLog, message)
	}
}

// displayLogLoop manages the log refresh and ensures protocol and canbus logs remain in sync
func (l *Logger) displayLogLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(logRefreshDelay)
			for _, subscriber := range logSubscribers {
				for _, log := range l.bufferedProtocolLog {
					subscriber(log, LogTypeProtocolLog)
				}
				for _, log := range l.bufferedCanbusLog {
					subscriber(log, LogTypeCanbusLog)
				}
				for _, log := range l.bufferedLog {
					subscriber(log, LogTypeLog)
				}
			}

			// Clear logs after refreshing
			l.bufferedProtocolLog = nil
			l.bufferedCanbusLog = nil
			l.bufferedLog = nil
		}
	}
}
