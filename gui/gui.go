package gui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"husk/canbus"
)

const (
	windowName                        = "husk"
	manualCanBusFrameEntryPlaceholder = "enter can bus message here"
)

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
}

func (g *GUI) RunApp() {
	g.app = app.New()
	g.window = g.app.NewWindow(windowName)

	g.logLabel = widget.NewLabel("")
	g.logLabel.Wrapping = fyne.TextWrapWord

	g.logScrollContainer = container.NewVScroll(g.logLabel)
	g.logScrollContainer.SetMinSize(fyne.NewSize(400, 300))
	g.autoScroll = true

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

	g.window.ShowAndRun()
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
	g.WriteToLog(fmt.Sprintf("%s%s\n", g.logLabel.Text, frame.String()))
}

// WriteToLog writes the gui log
func (g *GUI) WriteToLog(newText string) {
	if !g.isRunning {
		return
	}

	// Update the label text
	g.logLabel.SetText(newText)

	// Auto-scroll if enabled
	if g.autoScroll {
		g.logScrollContainer.ScrollToBottom()
	}
}
