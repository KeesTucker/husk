package logging

import (
	"context"
	"fmt"
	"time"

	"husk/gui"
)

type Logger struct {
	g             *gui.GUI
	bufferedUILog string
}

const (
	logRefreshRate = 64
)

var logRefreshDelay = time.Duration((1.0 / logRefreshRate) * float64(time.Second))

func NewLogger(ctx context.Context, gui *gui.GUI) *Logger {
	l := &Logger{g: gui}
	go l.displayLogLoop(ctx)
	return l
}

// WriteToLog writes to both the gui log and the console
func (l *Logger) WriteToLog(message string) {
	fmt.Println(message)
	if l.g != nil {
		l.bufferedUILog += message + "\n"
	}
}

func (l *Logger) displayLogLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(logRefreshDelay)

			if l.g.WriteToLog(l.bufferedUILog) {
				// If write to log is successful clear the log
				l.bufferedUILog = ""
			}

		}
	}
}
