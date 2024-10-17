package gui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"husk/ecus"
	"husk/protocols"
	"husk/services"
)

const (
	windowName                  = "husk"
	manualFrameEntryPlaceholder = "enter can bus message here"
	maxLogCharsLen              = 8192
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
	manualFrameEntry   *widget.Entry
}

func RegisterGUI() *GUI {
	g := &GUI{}
	defer services.Register(services.ServiceGUI, g)
	return g
}

func (g *GUI) Start(ctx context.Context) *GUI {
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

	g.manualFrameEntry = widget.NewEntry()
	g.manualFrameEntry.SetPlaceHolder(manualFrameEntryPlaceholder)

	sendManualFrameButton := widget.NewButton("Send CAN", func() { g.sendManualFrame(ctx) })

	manualFrameEntryContainer := container.NewBorder(nil, nil, nil, sendManualFrameButton, g.manualFrameEntry)

	content := container.NewBorder(
		nil,
		manualFrameEntryContainer,
		nil,
		nil,
		g.logScrollContainer,
	)

	g.window = g.app.NewWindow(windowName)
	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(600, 400))

	g.isRunning = true

	g.window.ShowAndRun()

	return g
}

func (g *GUI) WriteToLog(in string) {
	if !g.isRunning {
		return
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
}

func (g *GUI) sendManualFrame(ctx context.Context) {
	e := services.Get(services.ServiceECU).(ecus.ECUProcessor)

	if g.manualFrameEntry.Text != "" {
		frame, err := protocols.StringToFrame(g.manualFrameEntry.Text)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("error: parsing frame: %s\n", err.Error()))
			return
		}
		// todo: this should call the ecu send frame
		err = e.SendFrame(ctx, frame)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("error: sending manual frame: %v", err))
			return
		}

		g.manualFrameEntry.SetText("")
	}
}
