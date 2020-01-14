package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/lyraproj/dgo/dgo"

	"github.com/lyraproj/dgo/streamer"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/vf"
	"github.com/lyraproj/hiera/hiera"
	"github.com/lyraproj/hiera/hieraapi"
	"github.com/lyraproj/hiera/provider"
	sdk "github.com/lyraproj/hierasdk/hiera"
)

const serviceName = `hiera`
const lookup = `lookup`
const prefix = serviceName + `.` + lookup + `.`
const prefixLen = len(prefix)

const lookupValue = `value`
const valuePrefix = serviceName + `.` + lookupValue + `.`
const valuePrefixLen = len(valuePrefix)

type lookupResult struct {
	Value interface{} `json:"value"`
}

func main() {
	configOptions := vf.Map(
		provider.LookupKeyFunctions, []sdk.LookupKey{provider.ConfigLookupKey},
		hieraapi.HieraRoot, `.`)

	hiera.DoWithParent(context.Background(), provider.MuxLookupKey, configOptions, func(hs hieraapi.Session) {
		// Create a new RES Service
		s := res.NewService(serviceName)

		// Add handlers for "value.$key" models. The response will always be a struct
		// containing a model or a collection.
		s.Handle(
			lookupValue+`.>`,
			res.Access(res.AccessGranted),
			res.GetResource(func(r res.GetRequest) {
				getHieraValueHandler(r, hs)
			}),
		)

		// Add handlers for "lookup.$key" models. The response will always be a struct
		// containing a value.
		s.Handle(
			lookup+`.>`,
			res.Access(res.AccessGranted),
			res.GetResource(func(r res.GetRequest) {
				getHieraHandler(r, hs)
			}),
		)

		// Start service in separate goroutine
		stop := make(chan bool)
		go func() {
			defer close(stop)
			if err := s.ListenAndServe("nats://localhost:4222"); err != nil {
				fmt.Printf("%s\n", err.Error())
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
	})
}

func getHieraValueHandler(r res.GetRequest, hs hieraapi.Session) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, valuePrefix) {
		r.NotFound()
		return
	}
	hk := hieraapi.NewKey(key[valuePrefixLen:])
	result := hs.Invocation(nil, nil).Lookup(hk, nil)
	if result == nil {
		r.NotFound()
	} else {
		dc := streamer.DataCollector()
		streamer.New(nil, streamer.DefaultOptions()).Stream(result, dc)
		v := dc.Value()
		switch v := v.(type) {
		case dgo.Array:
			r.Collection(arrayToCollection(v, key+`.`))
		case dgo.Map:
			r.Model(mapToModel(v, key+`.`))
		default:
			// Primitives never found here since they cannot be referenced
			r.NotFound()
		}
	}
}

func getHieraHandler(r res.GetRequest, hs hieraapi.Session) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, prefix) {
		r.NotFound()
		return
	}
	hk := hieraapi.NewKey(key[prefixLen:])
	result := hs.Invocation(nil, nil).Lookup(hk, nil)
	if result == nil {
		r.NotFound()
	} else {
		dc := streamer.DataCollector()
		streamer.New(nil, streamer.DefaultOptions()).Stream(result, dc)
		v := dc.Value()
		var iv interface{}
		switch v := v.(type) {
		case dgo.Array, dgo.Map:
			// Arrays and maps are returned as references
			r.Model(&lookupResult{Value: res.Ref(valuePrefix + hk.Source())})
		default:
			vf.FromValue(v, &iv)
			r.Model(&lookupResult{Value: iv})
		}
	}
}

func arrayToCollection(a dgo.Array, path string) []interface{} {
	s := make([]interface{}, a.Len())
	a.EachWithIndex(func(value dgo.Value, index int) {
		switch value.(type) {
		case dgo.Map, dgo.Array:
			s[index] = res.Ref(path + strconv.Itoa(index))
		default:
			vf.FromValue(value, &s[index])
		}
	})
	return s
}

func mapToModel(m dgo.Map, path string) map[string]interface{} {
	ms := make(map[string]interface{}, m.Len())
	m.EachEntry(func(value dgo.MapEntry) {
		v := value.Value()
		ks := value.Key().(dgo.String).GoString()
		switch v.(type) {
		case dgo.Map, dgo.Array:
			ms[ks] = res.Ref(path + ks)
		default:
			var is interface{}
			vf.FromValue(v, &is)
			ms[ks] = is
		}
	})
	return ms
}
