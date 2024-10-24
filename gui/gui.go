package gui

import (
	"context"
	"fmt"
	"image/color"
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
	manualFrameEntryPlaceholder = "Enter manual frame..."
	sendManualFrameButtonText   = "Send CAN"
	driverScanButtonText        = "Scan"
	driverConnectButtonText     = "Connect"
	driverDisconnectButtonText  = "Disconnect"
	ecuScanButtonText           = "Scan"
	ecuConnectButtonText        = "Connect"
	ecuDisconnectButtonText     = "Disconnect"
	logSectionLabelText         = "Log"
	messagesSectionLabelText    = "CANBUS/Protocol Messages"
	driverLabelText             = "Select Driver"
	ecuLabelText                = "Select ECU"
	readErrorsButtonText        = "Read Errors"
	clearErrorsButtonText       = "Clear Errors"
)

type GUI struct {
	app    fyne.App
	window fyne.Window
	// state
	isRunning          bool
	autoScrollLogs     bool
	autoScrollMessages bool
	driverName         string
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
	logContainer           *fyne.Container
	logScrollContainer     *container.Scroll
	messageContainer       *fyne.Container
	messageScrollContainer *container.Scroll
}

func RegisterGUI() *GUI {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	g := &GUI{}
	defer services.Register(services.ServiceGUI, g)

	l.AddLogSub(g.writeLog)
	l.AddMessageSub(g.writeMessage)
	return g
}

func (g *GUI) Start(ctx context.Context) *GUI {
	g.autoScrollLogs = true
	g.autoScrollMessages = true
	g.app = app.New()
	g.app.Settings().SetTheme(&HuskTheme{})
	g.buildUI(ctx)
	g.subToEvents()
	g.isRunning = true
	g.window.ShowAndRun()
	return g
}

// buildUI constructs the user interface
func (g *GUI) buildUI(ctx context.Context) {
	// Initialize Log section
	g.logContainer = container.NewVBox()
	var outerLogContainer *fyne.Container
	outerLogContainer, g.logScrollContainer = g.createScrollContainerWithBG(
		logSectionLabelText,
		g.logContainer,
		true,
	)

	g.messageContainer = container.NewVBox()
	var outerMessageContainer *fyne.Container
	outerMessageContainer, g.messageScrollContainer = g.createScrollContainerWithBG(
		messagesSectionLabelText,
		g.messageContainer,
		false,
	)

	// Initialize command section
	commandContainer := g.createCommandContainer(ctx)

	// Set up splits
	messageAndLogSplit := container.NewHSplit(outerLogContainer, outerMessageContainer)
	messageAndLogSplit.SetOffset(0.40)
	content := container.NewHSplit(commandContainer, messageAndLogSplit)
	content.SetOffset(0.25)

	// Initialize and configure the main window
	g.window = g.app.NewWindow(windowName)
	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(windowWidth, windowHeight))
}

// createScrollContainerWithBG creates a scrollable container with a background. TODO: isLog needs to be an enum or something, this is a bit jank.
func (g *GUI) createScrollContainerWithBG(
	title string,
	entryContainer *fyne.Container,
	isLog bool,
) (
	*fyne.Container,
	*container.Scroll,
) {
	size := fyne.NewSize(scrollContainerMinWidth, scrollContainerMinHeight)
	// Double pad the entry container and add it to a scroll
	scroll := container.NewScroll(
		container.NewPadded(
			container.NewPadded(entryContainer),
		),
	)
	scroll.SetMinSize(size)
	// Set background color
	bgColor := color.Black
	background := canvas.NewRectangle(bgColor)
	bgContainer := container.NewStack(background, scroll)

	// Handle scroll events to toggle autoScroll
	scroll.OnScrolled = func(offset fyne.Position) {
		contentHeight := scroll.Content.Size().Height
		containerHeight := scroll.Size().Height
		if offset.Y+containerHeight >= contentHeight-scrollThreshold {
			if isLog {
				g.autoScrollLogs = true
			} else {
				g.autoScrollMessages = true
			}
		} else {
			if isLog {
				g.autoScrollLogs = false
			} else {
				g.autoScrollMessages = false
			}
		}
	}

	label := widget.NewLabel(title)
	label.Alignment = fyne.TextAlignCenter

	scrollWithLabel := container.NewBorder(label, nil, nil, nil, bgContainer)
	return scrollWithLabel, scroll
}

// createCommandContainer initializes the command section with driver and ECU controls
func (g *GUI) createCommandContainer(ctx context.Context) *fyne.Container {
	// Manual frame entry and send button
	g.manualFrameEntry = widget.NewEntry()
	g.manualFrameEntry.SetPlaceHolder(manualFrameEntryPlaceholder)
	g.manualFrameEntry.Disable()
	g.sendManualFrameButton = widget.NewButton(
		sendManualFrameButtonText,
		func() { g.sendManualFrame(ctx) },
	)
	g.sendManualFrameButton.Disable()
	manualFrameEntryContainer := container.NewBorder(
		nil,
		nil,
		nil,
		g.sendManualFrameButton,
		g.manualFrameEntry)

	// Driver selection controls
	driverLabel := widget.NewLabel(driverLabelText)
	g.driverScanButton = widget.NewButton(driverScanButtonText, drivers.ScanForDrivers)
	g.driverSelect = widget.NewSelect(nil, func(_ string) {
		g.driverConnectButton.Enable()
	})
	g.driverSelect.Disable()
	g.driverConnectButton = widget.NewButton(
		driverConnectButtonText, func() { drivers.Connect(ctx, g.driverSelect.Selected) })
	g.driverConnectButton.Disable()
	g.driverDisconnectButton = widget.NewButton(
		driverDisconnectButtonText, func() { drivers.Disconnect(); ecus.Disconnect() })
	g.driverDisconnectButton.Disable()
	driverContainer := container.NewHBox(
		driverLabel,
		g.driverScanButton,
		g.driverSelect,
		g.driverConnectButton,
		g.driverDisconnectButton,
	)

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
	g.ecuConnectButton = widget.NewButton(
		ecuConnectButtonText, func() { ecus.Connect(ctx, g.ecuSelect.Selected) })
	g.ecuConnectButton.Disable()
	g.ecuDisconnectButton = widget.NewButton(
		ecuDisconnectButtonText, func() { ecus.Disconnect() })
	g.ecuDisconnectButton.Disable()
	ecuContainer := container.NewHBox(
		ecuLabel,
		g.ecuScanButton,
		g.ecuSelect,
		g.ecuConnectButton,
		g.ecuDisconnectButton,
	)

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

	commandContainer := container.NewBorder(
		nil,
		manualFrameEntryContainer,
		nil,
		nil,
		container.NewVBox(
			driverContainer,
			ecuContainer,
			miscCommands,
		),
	)

	return commandContainer
}

func (g *GUI) subToEvents() {
	drivers.SubscribeToScanEvent(g.onDriversScan)
	drivers.SubscribeToConnectedEvent(g.onDriverConnected)
	drivers.SubscribeToDisconnectedEvent(g.onDriverDisconnected)
	ecus.SubscribeToScanEvent(g.onECUScan)
	ecus.SubscribeToConnectedEvent(g.onECUConnected)
	ecus.SubscribeToDisconnectedEvent(g.onECUDisconnected)
}

func (g *GUI) writeLog(log logging.Log) {
	if !g.isRunning {
		return
	}

	messageLines := strings.Split(log.Message, "\n")

	for i, line := range messageLines {
		if i > 0 {
			line = "	" + line
		}
		label := canvas.NewText(line, color.Black)
		label.TextSize = 14
		label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

		switch log.Level {
		case logging.LogLevelSuccess:
			label.Color = color.RGBA{R: 0, G: 255, B: 0, A: 255}
		case logging.LogLevelInfo:
			label.Color = color.White
		case logging.LogLevelWarning:
			label.Color = color.RGBA{R: 255, G: 255, B: 0, A: 255}
		case logging.LogLevelError:
			label.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
		case logging.LogLevelResult:
			label.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255}
		}

		// Add label to log container
		g.logContainer.Add(label)
	}

	// Automatically scroll to bottom if autoScroll is enabled
	if g.autoScrollLogs {
		g.logScrollContainer.ScrollToBottom()
	}
}

func (g *GUI) writeMessage(message logging.Message) {
	if !g.isRunning {
		return
	}

	// Break up message request response chains
	if message.MessageType == logging.MessageTypeUDSWrite {
		// Create a horizontal line
		line := canvas.NewLine(color.White)
		line.StrokeWidth = 2
		line.Resize(fyne.NewSize(g.messageContainer.Size().Width, 2))
		// Add the horizontal line to the container
		g.messageContainer.Add(line)
	}

	dataLines := strings.Split(message.Data, "\n")

	for i, line := range dataLines {
		if i > 0 {
			line = "	" + line
		}
		label := canvas.NewText(line, color.Black)
		label.TextSize = 14
		label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

		switch message.MessageType {
		case logging.MessageTypeCANBUSWrite:
			label.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255}
		case logging.MessageTypeCANBUSRead:
			label.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
		case logging.MessageTypeUDSWrite:
			label.Color = color.RGBA{R: 0, G: 255, B: 0, A: 255}
		case logging.MessageTypeUDSRead:
			label.Color = color.RGBA{R: 255, G: 0, B: 255, A: 255}
		}

		// Add label to log container
		g.messageContainer.Add(label)
	}

	// Create a faint horizontal line to break up messages
	line := canvas.NewLine(color.Gray{Y: 128})
	line.StrokeWidth = 1
	line.Resize(fyne.NewSize(g.messageContainer.Size().Width, 1))
	g.messageContainer.Add(line)

	// Automatically scroll to bottom if autoScroll is enabled
	if g.autoScrollMessages {
		g.messageScrollContainer.ScrollToBottom()
	}
}

func (g *GUI) sendManualFrame(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	if g.manualFrameEntry.Text != "" {
		data, err := utils.HexStringToByteArray(g.manualFrameEntry.Text)
		if err != nil {
			l.WriteLog(fmt.Sprintf("Error parsing frame: %s", err.Error()), logging.LogLevelError)
			return
		}
		err = uds.RawDataToMessage(uds.TesterID, data, false).Send(ctx)
		if err != nil {
			l.WriteLog(fmt.Sprintf("Error sending manual frame: %s", err.Error()), logging.LogLevelError)
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
