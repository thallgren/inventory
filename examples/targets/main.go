package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	s.SetLogger(logrus.StandardLogger())
	p := setupTest()
	bs := bolt.NewStorage(p)
	is := inventory.NewService(s, bs)
	watcher := bs.Watch(is.Modifications)

	// Start service in separate goroutine
	stop := make(chan bool)
	go func() {
		defer close(stop)
		if err := s.ListenAndServe("nats://localhost:4222"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}()

	// Run a simple webserver to serve the client.
	// This is only for the purpose of making the example easier to run.
	go func() { log.Fatal(http.ListenAndServe(":8084", http.FileServer(http.Dir("wwwroot/")))) }()
	fmt.Println("Client at: http://localhost:8084/")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		// Graceful stop
		_ = s.Shutdown()
		_ = watcher.Close()
	case <-stop:
	}
}

func setupTest() string {
	logrus.SetLevel(logrus.DebugLevel)
	testdata, err := filepath.Abs(filepath.Join(`..`, `..`, `testdata`))
	if err != nil {
		panic(err)
	}
	vd := filepath.Join(testdata, `volatile`)
	err = os.MkdirAll(vd, 0750)
	if err != nil {
		if !os.IsExist(err) {
			panic(err)
		}
	}

	sd := filepath.Join(testdata, `static`)
	files, err := ioutil.ReadDir(sd)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		/* #nosec */
		bytes, err := ioutil.ReadFile(filepath.Join(sd, f.Name()))
		if err != nil {
			panic(err)
		}
		bs := filepath.Join(vd, f.Name())
		err = ioutil.WriteFile(bs, bytes, 0640)
		if err != nil {
			panic(err)
		}
	}
	return vd
}
