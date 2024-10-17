package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"husk/drivers"
	"husk/ecus"
	"husk/gui"
	"husk/logging"
)

func main() {
	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the context is canceled to free resources when main function exits

	// Set up a channel to listen for OS signals to gracefully handle shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Register and start services
	l := logging.RegisterLogger().Start(ctx)
	d := drivers.RegisterArduinoDriver().Start(ctx)
	ecus.RegisterProcessor(ecus.ECUTypeHusqvarnaKTM).Start(ctx)

	// Start a separate goroutine to listen for OS signals to handle shutdown gracefully
	go func() {
		<-signalChan
		l.WriteToLog("received shutdown signal, canceling context and cleaning up...")
		cancel()
	}()

	// Register GUI and sub to logger
	g := gui.RegisterGUI()
	l.AddLogSub(g.WriteToLog)
	// Start logger (this will block)
	g.Start(ctx)

	// Ensure cleanup of resources
	if err := d.Cleanup(); err != nil {
		l.WriteToLog(fmt.Sprintf("error: during driver cleanup: %s", err.Error()))
	}
}
