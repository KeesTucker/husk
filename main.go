package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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

	// Start a separate goroutine to listen for OS signals to handle shutdown gracefully
	go func() {
		<-signalChan
		l.WriteToLog("Received shutdown signal, canceling context and cleaning up", logging.LogTypeLog)
		cancel()
	}()

	// Register GUI and sub to logger
	g := gui.RegisterGUI()
	// Start logger (this will block)
	g.Start(ctx)
}
