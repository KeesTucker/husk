package logging

import (
	"context"
	"time"

	"husk/services"
)

type Logger struct {
	bufferedLog string
}

type LogSubFunc func(string)

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

// WriteToLog writes to both the gui log and the console
func (l *Logger) WriteToLog(message string) {
	l.bufferedLog += message + "\n"
}

func (l *Logger) displayLogLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(logRefreshDelay)

			for _, subscriber := range logSubscribers {
				subscriber(l.bufferedLog)
			}

			l.bufferedLog = ""
		}
	}
}
