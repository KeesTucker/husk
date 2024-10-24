package logging

import (
	"context"
	"time"

	"husk/services"
)

type (
	LogLevel    int
	MessageType int
)

type Log struct {
	Message string
	Level   LogLevel
}

type Message struct {
	Data        string
	MessageType MessageType
}

type Logger struct {
	bufferedLog      []Log
	bufferedMessages []Message
}

type (
	LogSubFunc     func(log Log)
	MessageSubFunc func(message Message)
)

const (
	LogLevelInfo LogLevel = iota
	LogLevelSuccess
	LogLevelWarning
	LogLevelError
	LogLevelResult
)

const (
	MessageTypeCANBUSRead MessageType = iota
	MessageTypeCANBUSWrite
	MessageTypeUDSRead
	MessageTypeUDSWrite
)

const (
	refreshRate = 64
)

var (
	refreshDelay       = time.Duration((1.0 / refreshRate) * float64(time.Second))
	logSubscribers     []LogSubFunc
	messageSubscribers []MessageSubFunc
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

func (l *Logger) AddLogSub(subFunc LogSubFunc) {
	logSubscribers = append(logSubscribers, subFunc)
}

func (l *Logger) AddMessageSub(subFunc MessageSubFunc) {
	messageSubscribers = append(messageSubscribers, subFunc)
}

// WriteLog writes to the log buffer
func (l *Logger) WriteLog(message string, logType LogLevel) {
	log := Log{Message: message, Level: logType}
	l.bufferedLog = append(l.bufferedLog, log)
}

// WriteMessage writes a canbus/uds message to the message buffer
func (l *Logger) WriteMessage(data string, messageType MessageType) {
	message := Message{Data: data, MessageType: messageType}
	l.bufferedMessages = append(l.bufferedMessages, message)
}

// displayLogLoop manages the log refresh and ensures protocol and canbus logs remain in sync
func (l *Logger) displayLogLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(refreshDelay)
			for _, subscriber := range logSubscribers {
				for _, log := range l.bufferedLog {
					subscriber(log)
				}
			}
			for _, subscriber := range messageSubscribers {
				for _, message := range l.bufferedMessages {
					subscriber(message)
				}
			}
			// Clear buffers after displaying
			l.bufferedLog = nil
			l.bufferedMessages = nil
		}
	}
}
