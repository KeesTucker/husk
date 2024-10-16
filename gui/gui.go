package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	windowName                        = "husk"
	manualCanBusFrameEntryPlaceholder = "enter can bus message here"
	maxLogCharsLen                    = 8192
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

func NewGUI() *GUI {
	return &GUI{}
}

func (g *GUI) RunApp() {
	g.autoScroll = true
	g.app = app.New()
	g.logLabel = widget.NewLabel("")
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

	g.window = g.app.NewWindow(windowName)
	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(600, 400))

	g.isRunning = true

	g.window.ShowAndRun()
}

// SetSendManualCanBusFrameCallback allows setting the callback for sending manual CAN bus frames.
func (g *GUI) SetSendManualCanBusFrameCallback(callback func(string)) {
	g.sendManualCanBusFrameCallback = callback
}

func (g *GUI) WriteToLog(in string) bool {
	if !g.isRunning {
		return false
	}

	// Combine existing log text with the new text
	newLabelText := g.logLabel.Text + in

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

	return true
}
