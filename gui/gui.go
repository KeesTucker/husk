package gui

import (
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
	app fyne.App

	// state
	autoScroll bool

	// UI elements
	rawCanBusFrameViewer *container.Scroll
	rawCanBusFrameOutput *widget.Label

	// callbacks
	sendManualCanBusFrameCallback func(string)
}

func (g *GUI) RunApp() {
	g.app = app.New()
	w := g.app.NewWindow(windowName)

	g.rawCanBusFrameOutput = widget.NewLabel("")
	g.rawCanBusFrameOutput.Wrapping = fyne.TextWrapWord

	g.rawCanBusFrameViewer = container.NewVScroll(g.rawCanBusFrameOutput)
	g.rawCanBusFrameViewer.SetMinSize(fyne.NewSize(400, 300))
	g.autoScroll = true

	// Turn off auto scroll when user scrolls up.
	g.rawCanBusFrameViewer.OnScrolled = func(offset fyne.Position) {
		if offset.Y+g.rawCanBusFrameViewer.Size().Height >= g.rawCanBusFrameViewer.Content.Size().Height - 20 { //todo: make auto scroll offset const and tune for feel
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
		g.rawCanBusFrameViewer,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 400))
	w.ShowAndRun()

	w.ShowAndRun()
}

// SetSendManualCanBusFrameCallback allows setting the callback for sending manual CAN bus frames.
func (g *GUI) SetSendManualCanBusFrameCallback(callback func(string)) {
	g.sendManualCanBusFrameCallback = callback
}

// OnCanBusFrameReceive is called when a new CAN bus frame is received.
func (g *GUI) OnCanBusFrameReceive(frame *canbus.Frame) {
	// Append the new frame to the output label and add a newline for better formatting
	newText := g.rawCanBusFrameOutput.Text + frame.String() + "\n"

	// Update the label text on the UI thread
	g.rawCanBusFrameOutput.SetText(newText)

	// Auto-scroll if enabled
	if g.autoScroll {
		g.rawCanBusFrameViewer.ScrollToBottom()
	}
}
