package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/puppetlabs/inventory/bolt"

	"github.com/jirenius/go-res"
	"github.com/puppetlabs/inventory/inventory"
)

func main() {
	// Create a new RES Service
	s := res.NewService(inventory.ServiceName)
	p := setupTest()
	inventory.NewService(s, bolt.NewStorage(p))

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

func setupTest() string {
	logrus.SetLevel(logrus.DebugLevel)
	bytes, err := ioutil.ReadFile(filepath.Join(`testdata`, `static`, `bolt_precedence.yaml`))
	if err != nil {
		panic(err)
	}

	vd := filepath.Join(`testdata`, `volatile`)
	if err = os.MkdirAll(vd, 0750); err != nil {
		if !os.IsExist(err) {
			panic(err)
		}
	}
	bs := filepath.Join(vd, `bolt_precedence.yaml`)
	err = ioutil.WriteFile(bs, bytes, 0640)
	if err != nil {
		panic(err)
	}
	return bs
}
