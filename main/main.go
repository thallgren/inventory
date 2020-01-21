package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/jirenius/go-res"
	"github.com/puppetlabs/inventory/inventory"
)

func main() {
	// Create a new RES Service
	s := res.NewService(inventory.ServiceName)

	p, err := filepath.Abs(`testdata`)
	if err != nil {
		panic(err)
	}
	is := inventory.NewService(p, `realms`, `nodes`, `facts`)
	is.AddHandlers(s)

	// Start service in separate goroutine
	stop := make(chan bool)
	go func() {
		defer close(stop)
		if err := s.ListenAndServe("nats://localhost:4222"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		// Graceful stop
		_ = s.Shutdown()
	case <-stop:
	}
}
