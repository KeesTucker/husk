package gui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"husk/canbus"
	"time"
)

const (
	windowName                        = "husk"
	manualCanBusFrameEntryPlaceholder = "enter can bus message here"
	maxLogCharsLen                    = 8192
	logRefreshRate                    = 64
)

var logRefreshDelay = time.Duration((1.0 / logRefreshRate) * float64(time.Second))

type GUI struct {
	app    fyne.App
	window fyne.Window

	// state
	isRunning  bool
	autoScroll bool

	// UI elements
	logScrollContainer *container.Scroll
	logLabel           *widget.Label

	// callbacks
	sendManualCanBusFrameCallback func(string)

	incomingLog string
}

func NewGUI() *GUI {
	return &GUI{
		app:        app.New(),
		autoScroll: true,
		logLabel:   widget.NewLabel(""),
	}
}

func (g *GUI) RunApp(ctx context.Context) {
	g.window = g.app.NewWindow(windowName)
	g.logLabel.Wrapping = fyne.TextWrapWord
	g.logScrollContainer = container.NewVScroll(g.logLabel)
	g.logScrollContainer.SetMinSize(fyne.NewSize(400, 300))

	// Turn off auto scroll when user scrolls up.
	g.logScrollContainer.OnScrolled = func(offset fyne.Position) {
		if offset.Y+g.logScrollContainer.Size().Height >= g.logScrollContainer.Content.Size().Height-20 {
			g.autoScroll = true // User is near the bottom
		} else {
			g.autoScroll = false // User scrolled up
		}
	}

	manualCanBusFrameEntry := widget.NewEntry()
	manualCanBusFrameEntry.SetPlaceHolder(manualCanBusFrameEntryPlaceholder)

	sendManualCanBusFrameButton := widget.NewButton("Send CAN", func() {
		if g.sendManualCanBusFrameCallback != nil && manualCanBusFrameEntry.Text != "" {
			g.sendManualCanBusFrameCallback(manualCanBusFrameEntry.Text)
			manualCanBusFrameEntry.SetText("")
		}
	})

	manualCanBusFrameEntryContainer := container.NewBorder(nil, nil, nil, sendManualCanBusFrameButton, manualCanBusFrameEntry)

	content := container.NewBorder(
		nil,
		manualCanBusFrameEntryContainer,
		nil,
		nil,
		g.logScrollContainer,
	)

	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(600, 400))

	g.isRunning = true

	go g.logLoop(ctx)

	g.window.ShowAndRun()
}

func (g *GUI) logLoop(ctx context.Context) {
	for {
		if !g.isRunning {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(logRefreshDelay)

			// Combine existing log text with the new text
			newLabelText := fmt.Sprintf("%s%s", g.logLabel.Text, g.incomingLog)
			g.incomingLog = ""

			// Convert to runes to handle multi-byte characters properly
			runes := []rune(newLabelText)

			// Check if the combined text exceeds 1000 characters
			if len(runes) > maxLogCharsLen {
				// Trim the oldest characters to maintain a 1000-character limit
				runes = runes[len(runes)-maxLogCharsLen:]
				newLabelText = string(runes)
			}

			// Update the label text with the capped log
			g.logLabel.SetText(newLabelText)

			// Auto-scroll if enabled
			if g.autoScroll {
				g.logScrollContainer.ScrollToBottom()
			}
		}
	}
}

// SetSendManualCanBusFrameCallback allows setting the callback for sending manual CAN bus frames.
func (g *GUI) SetSendManualCanBusFrameCallback(callback func(string)) {
	g.sendManualCanBusFrameCallback = callback
}

// OnCanBusFrameReceive is called when a new CAN bus frame is received.
func (g *GUI) OnCanBusFrameReceive(frame *canbus.Frame) {
	if !g.isRunning {
		return
	}

	// Append the new frame to the output label
	g.WriteToLog(frame.String())
}

func (g *GUI) WriteToLog(newLine string) {
	g.incomingLog += newLine + "\n"
}
