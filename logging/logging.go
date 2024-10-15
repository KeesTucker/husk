package logging

import (
	"fmt"
	"husk/gui"
)

type Logger struct {
	g *gui.GUI
}

func NewLogger(gui *gui.GUI) *Logger {
	return &Logger{g: gui}
}

// WriteToLog writes to both the gui log and the console
func (l *Logger) WriteToLog(message string) {
	fmt.Println(message)
	if l.g != nil {
		l.g.WriteToLog(message)
	}
}
