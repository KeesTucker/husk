package gui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"husk/drivers"
	"husk/ecus"
	"husk/logging"
	"husk/services"
	"husk/uds"
	"husk/utils"
)

const (
	windowName                  = "husk"
	windowWidth                 = 1920
	windowHeight                = 1080
	scrollContainerMinWidth     = 200
	scrollContainerMinHeight    = 100
	scrollThreshold             = 20
	scrollBackgroundColor       = "#000412"
	manualFrameEntryPlaceholder = "Enter manual frame..."
	sendManualFrameButtonText   = "Send CAN"
	driverScanButtonText        = "Scan"
	driverConnectButtonText     = "Connect"
	driverDisconnectButtonText  = "Disconnect"
	ecuScanButtonText           = "Scan"
	ecuConnectButtonText        = "Connect"
	ecuDisconnectButtonText     = "Disconnect"
	protocolSectionLabelText    = "Protocol Messages"
	canbusSectionLabelText      = "CANBUS Frames"
	logSectionLabelText         = "Log"
	driverLabelText             = "Select Driver"
	ecuLabelText                = "Select ECU"
	readErrorsButtonText        = "Read Errors"
	clearErrorsButtonText       = "Clear Errors"
)

var emptyLogLabel *widget.Label = widget.NewLabel("")

type GUI struct {
	app    fyne.App
	window fyne.Window
	// state
	isRunning  bool
	autoScroll bool
	driverName string
	// UI elements
	driverScanButton       *widget.Button
	driverSelect           *widget.Select
	driverConnectButton    *widget.Button
	driverDisconnectButton *widget.Button
	ecuScanButton          *widget.Button
	ecuSelect              *widget.Select
	ecuConnectButton       *widget.Button
	ecuDisconnectButton    *widget.Button
	manualFrameEntry       *widget.Entry
	sendManualFrameButton  *widget.Button
	logEntryContainer      *fyne.Container
	logScrollContainer     *container.Scroll
}

func RegisterGUI() *GUI {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	g := &GUI{}
	defer services.Register(services.ServiceGUI, g)

	l.AddLogSub(g.WriteToLog)
	return g
}

func (g *GUI) Start(ctx context.Context) *GUI {
	g.autoScroll = true
	g.app = app.New()
	g.buildUI(ctx)
	g.subToEvents()
	g.isRunning = true
	g.window.ShowAndRun()
	return g
}

func (g *GUI) WriteToLog(message string, logType logging.LogType) {
	if !g.isRunning {
		return
	}

	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord

	switch logType {
	case logging.LogTypeProtocolLog:
		g.logEntryContainer.Add(container.NewGridWithColumns(3, emptyLogLabel, emptyLogLabel, label))
	case logging.LogTypeCanbusLog:
		g.logEntryContainer.Add(container.NewGridWithColumns(3, emptyLogLabel, label, emptyLogLabel))
	case logging.LogTypeLog:
		g.logEntryContainer.Add(container.NewGridWithColumns(3, label, emptyLogLabel, emptyLogLabel))
	}

	if g.autoScroll {
		g.logScrollContainer.ScrollToBottom()
	}
}

// buildUI constructs the user interface
func (g *GUI) buildUI(ctx context.Context) {
	// Initialize Log section
	g.logEntryContainer = container.NewVBox()
	var logBGContainer *fyne.Container
	logBGContainer, g.logScrollContainer = g.createScrollContainerWithBG(g.logEntryContainer)
	logSectionLabel := widget.NewLabel(logSectionLabelText)
	logSectionLabel.Alignment = fyne.TextAlignCenter
	canbusSectionLabel := widget.NewLabel(canbusSectionLabelText)
	canbusSectionLabel.Alignment = fyne.TextAlignCenter
	protocolSectionLabel := widget.NewLabel(protocolSectionLabelText)
	protocolSectionLabel.Alignment = fyne.TextAlignCenter
	labels := container.NewGridWithColumns(3, logSectionLabel, canbusSectionLabel, protocolSectionLabel)
	logContainer := container.NewBorder(labels, nil, nil, nil, logBGContainer)

	// Initialize command section
	commandContainer := g.createCommandContainer(ctx)

	// Set final split layout
	content := container.NewHSplit(commandContainer, logContainer)
	content.SetOffset(0.25)

	// Initialize and configure the main window
	g.window = g.app.NewWindow(windowName)
	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(windowWidth, windowHeight))
}

// createScrollContainerWithBG creates a scrollable container with a background
func (g *GUI) createScrollContainerWithBG(entryContainer *fyne.Container) (*fyne.Container, *container.Scroll) {
	scroll := container.NewVScroll(entryContainer)
	scroll.SetMinSize(fyne.NewSize(scrollContainerMinWidth, scrollContainerMinHeight))

	// Set background color
	bgColor, _ := parseHexColor(scrollBackgroundColor)
	background := canvas.NewRectangle(bgColor)
	bgContainer := container.NewStack(background, scroll)

	// Handle scroll events to toggle autoScroll
	scroll.OnScrolled = func(offset fyne.Position) {
		contentHeight := scroll.Content.Size().Height
		containerHeight := scroll.Size().Height
		if offset.Y+containerHeight >= contentHeight-scrollThreshold {
			g.autoScroll = true // User is near the bottom
		} else {
			g.autoScroll = false // User scrolled up
		}
	}

	return bgContainer, scroll
}

// createCommandContainer initializes the command section with driver and ECU controls
func (g *GUI) createCommandContainer(ctx context.Context) *fyne.Container {
	// Manual frame entry and send button
	g.manualFrameEntry = widget.NewEntry()
	g.manualFrameEntry.SetPlaceHolder(manualFrameEntryPlaceholder)
	g.manualFrameEntry.Disable()
	g.sendManualFrameButton = widget.NewButton(sendManualFrameButtonText, func() { g.sendManualFrame(ctx) })
	g.sendManualFrameButton.Disable()
	manualFrameEntryContainer := container.NewBorder(nil, nil, nil, g.sendManualFrameButton, g.manualFrameEntry)

	// Driver selection controls
	driverLabel := widget.NewLabel(driverLabelText)
	g.driverScanButton = widget.NewButton(driverScanButtonText, drivers.ScanForDrivers)
	g.driverSelect = widget.NewSelect(nil, func(_ string) {
		g.driverConnectButton.Enable()
	})
	g.driverSelect.Disable()
	g.driverConnectButton = widget.NewButton(driverConnectButtonText, func() { drivers.Connect(ctx, g.driverSelect.Selected) })
	g.driverConnectButton.Disable()
	g.driverDisconnectButton = widget.NewButton(driverDisconnectButtonText, func() {
		drivers.Disconnect()
		ecus.Disconnect()
	})
	g.driverDisconnectButton.Disable()
	driverContainer := container.NewHBox(driverLabel, g.driverScanButton, g.driverSelect, g.driverConnectButton, g.driverDisconnectButton)

	// ECU selection controls
	ecuLabel := widget.NewLabel(ecuLabelText)
	g.ecuScanButton = widget.NewButton(ecuScanButtonText, func() {
		ecus.ScanForECUs(ctx)
	})
	g.ecuScanButton.Disable()
	g.ecuSelect = widget.NewSelect(nil, func(_ string) {
		g.ecuConnectButton.Enable()
	})
	g.ecuSelect.Disable()
	g.ecuConnectButton = widget.NewButton(ecuConnectButtonText, func() { ecus.Connect(ctx, g.ecuSelect.Selected) })
	g.ecuConnectButton.Disable()
	g.ecuDisconnectButton = widget.NewButton(ecuDisconnectButtonText, func() { ecus.Disconnect() })
	g.ecuDisconnectButton.Disable()
	ecuContainer := container.NewHBox(ecuLabel, g.ecuScanButton, g.ecuSelect, g.ecuConnectButton, g.ecuDisconnectButton)

	// Misc commands
	readErrorsButton := widget.NewButton(readErrorsButtonText, func() {
		e := services.Get(services.ServiceECU).(ecus.ECUProcessor)
		e.ReadErrors(ctx)
	})

	clearErrorsButton := widget.NewButton(clearErrorsButtonText, func() {
		e := services.Get(services.ServiceECU).(ecus.ECUProcessor)
		e.ClearErrors(ctx)
	})

	miscCommands := container.NewHBox(readErrorsButton, clearErrorsButton)

	commandContainer := container.NewVBox(driverContainer, ecuContainer, miscCommands, manualFrameEntryContainer)

	return commandContainer
}

// parseHexColor parses a hex color string and returns a color.RGBA
func parseHexColor(s string) (color.Color, error) {
	// Remove the leading # if present
	s = strings.TrimPrefix(s, "#")

	// Ensure the string is 6 or 8 characters long
	if len(s) != 6 && len(s) != 8 {
		return nil, fmt.Errorf("invalid hex color: %s", s)
	}

	// Parse the red, green, and blue components
	r, err := strconv.ParseUint(s[0:2], 16, 8)
	if err != nil {
		return nil, err
	}
	g, err := strconv.ParseUint(s[2:4], 16, 8)
	if err != nil {
		return nil, err
	}
	b, err := strconv.ParseUint(s[4:6], 16, 8)
	if err != nil {
		return nil, err
	}

	var a uint64 = 255 // Default alpha (opaque)
	if len(s) == 8 {
		a, err = strconv.ParseUint(s[6:8], 16, 8)
		if err != nil {
			return nil, err
		}
	}

	return color.RGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: uint8(a),
	}, nil
}

func (g *GUI) subToEvents() {
	drivers.SubscribeToScanEvent(g.onDriversScan)
	drivers.SubscribeToConnectedEvent(g.onDriverConnected)
	drivers.SubscribeToDisconnectedEvent(g.onDriverDisconnected)
	ecus.SubscribeToScanEvent(g.onECUScan)
	ecus.SubscribeToConnectedEvent(g.onECUConnected)
	ecus.SubscribeToDisconnectedEvent(g.onECUDisconnected)
}

func (g *GUI) sendManualFrame(ctx context.Context) {
	if g.manualFrameEntry.Text != "" {
		data, err := utils.HexStringToByteArray(g.manualFrameEntry.Text)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("Error: parsing frame: %s\n", err.Error()), logging.LogTypeLog)
			return
		}
		err = uds.RawDataToMessage(uds.TesterID, data, false).Send(ctx)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("Error: sending manual frame: %v", err), logging.LogTypeLog)
			return
		}
		g.manualFrameEntry.SetText("")
	}
}

func (g *GUI) onDriversScan(availableDriverNames []string) {
	g.driverSelect.SetOptions(availableDriverNames)
	g.driverSelect.Selected = ""
	if len(availableDriverNames) == 0 {
		g.driverConnectButton.Disable()
		g.driverSelect.Disable()
		return
	}
	g.driverSelect.Enable()
}

func (g *GUI) onDriverConnected() {
	g.driverScanButton.Disable()
	g.driverSelect.Disable()
	g.driverConnectButton.Disable()
	g.driverDisconnectButton.Enable()
	g.ecuScanButton.Enable()
}

func (g *GUI) onDriverDisconnected() {
	g.driverScanButton.Enable()
	g.driverSelect.Enable()
	g.driverConnectButton.Enable()
	g.driverDisconnectButton.Disable()
	g.ecuScanButton.Disable()
}

func (g *GUI) onECUScan(availableECUIds []string) {
	g.ecuSelect.SetOptions(availableECUIds)
	g.ecuSelect.Selected = ""
	if len(availableECUIds) == 0 {
		g.ecuConnectButton.Disable()
		g.ecuSelect.Disable()
		return
	}
	g.ecuSelect.Enable()
}

func (g *GUI) onECUConnected() {
	g.ecuScanButton.Disable()
	g.ecuSelect.Disable()
	g.ecuConnectButton.Disable()
	g.ecuDisconnectButton.Enable()
	g.manualFrameEntry.Enable()
	g.sendManualFrameButton.Enable()
}

func (g *GUI) onECUDisconnected() {
	g.ecuScanButton.Enable()
	g.ecuSelect.Enable()
	g.ecuConnectButton.Enable()
	g.ecuDisconnectButton.Disable()
	g.manualFrameEntry.Disable()
	g.sendManualFrameButton.Disable()
}
