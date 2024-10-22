package gui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"husk/drivers"
	"husk/ecus"
	"husk/services"
	"husk/utils"
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
	logScrollContainer     *container.Scroll
	logLabel               *widget.Label
	manualFrameEntry       *widget.Entry
	sendManualFrameButton  *widget.Button
}

func RegisterGUI() *GUI {
	g := &GUI{}
	defer services.Register(services.ServiceGUI, g)
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
func (g *GUI) buildUI(ctx context.Context) {
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
	g.manualFrameEntry.Disable()
	g.sendManualFrameButton = widget.NewButton("Send CAN", func() { g.sendManualFrame(ctx) })
	g.sendManualFrameButton.Disable()
	manualFrameEntryContainer := container.NewBorder(nil, nil, nil, g.sendManualFrameButton, g.manualFrameEntry)
	driverLabel := widget.NewLabel("Select Driver")
	g.driverScanButton = widget.NewButton("Scan", drivers.ScanForDrivers)
	g.driverSelect = widget.NewSelect(nil, func(_ string) {
		g.driverConnectButton.Enable()
	})
	g.driverSelect.Disable()
	g.driverConnectButton = widget.NewButton("Connect", func() { drivers.Connect(ctx, g.driverSelect.Selected) })
	g.driverConnectButton.Disable()
	g.driverDisconnectButton = widget.NewButton("Disconnect", func() { drivers.Disconnect(); ecus.Disconnect() })
	g.driverDisconnectButton.Disable()
	driverContainer := container.NewHBox(driverLabel, g.driverScanButton, g.driverSelect, g.driverConnectButton, g.driverDisconnectButton)
	ecuLabel := widget.NewLabel("Select Ecu")
	g.ecuScanButton = widget.NewButton("Scan", func() {
		ecus.ScanForECUs(ctx)
	})
	g.ecuScanButton.Disable()
	g.ecuSelect = widget.NewSelect(nil, func(_ string) {
		g.ecuConnectButton.Enable()
	})
	g.ecuSelect.Disable()
	g.ecuConnectButton = widget.NewButton("Connect", func() { ecus.Connect(ctx, g.ecuSelect.Selected) })
	g.ecuConnectButton.Disable()
	g.ecuDisconnectButton = widget.NewButton("Disconnect", func() { ecus.Disconnect() })
	g.ecuDisconnectButton.Disable()
	ecuContainer := container.NewHBox(ecuLabel, g.ecuScanButton, g.ecuSelect, g.ecuConnectButton, g.ecuDisconnectButton)
	commandContainer := container.NewVBox(driverContainer, ecuContainer)
	canBusContainer := container.NewBorder(
		nil,
		manualFrameEntryContainer,
		nil,
		nil,
		g.logScrollContainer,
	)
	content := container.NewHBox(commandContainer, canBusContainer)
	g.window = g.app.NewWindow(windowName)
	g.window.SetContent(content)
	g.window.Resize(fyne.NewSize(600, 400))
}
func (g *GUI) subToEvents() {
	drivers.SubscribeToScanEvent(g.onDriversScan)
	drivers.SubscribeToConnectedEvent(g.onDriverConnected)
	drivers.SubscribeToDisconnectedEvent(g.onDriverDisconnected)
	ecus.SubscribeToScanEvent(g.onECUScan)
	ecus.SubscribeToConnectedEvent(g.onECUConnected)
	ecus.SubscribeToDisconnectedEvent(g.onECUDisconnected)
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
		data, err := utils.HexStringToByteArray(g.manualFrameEntry.Text)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("error: parsing frame: %s\n", err.Error()))
			return
		}
		err = e.SendMessage(ctx, data)
		if err != nil {
			g.WriteToLog(fmt.Sprintf("error: sending manual frame: %v", err))
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
