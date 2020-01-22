// Package inventory contains the resgate based inventory service
package inventory

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
)

// ServiceName is the name of the Resgate service
const ServiceName = `inventory`
const lookup = `lookup`
const lookupValue = `value`

const prefix = ServiceName + `.` + lookup + `.`
const prefixLen = len(prefix)

const valuePrefix = ServiceName + `.` + lookupValue + `.`
const valuePrefixLen = len(valuePrefix)

const valueKey = `__value`

type lookupResult struct {
	Value interface{} `json:"value"`
}

// A Service contains all the resgate handles and a storage.
type Service struct {
	storage Storage
}

// NewService creates a new Resgate service that will use the givne storage
func NewService(storage Storage) *Service {
	return &Service{storage}
}

// AddHandlers will initialize the given resgate service with the handlers of this Service
func (s *Service) AddHandlers(rs *res.Service) {
	// Add handlers for "value.$key" models. The response will always be a struct
	// containing a model or a collection.
	rs.Handle(
		lookupValue+`.>`,
		res.Access(res.AccessGranted),
		res.GetResource(s.getValueHandler),
	)

	// Add handlers for "lookup.$key" models. The response will always be a struct
	// containing a value.
	rs.Handle(
		lookup+`.>`,
		res.Access(res.AccessGranted),
		res.GetResource(s.getHandler),
		res.Set(s.setHandler),
		res.Call("delete", s.deleteHandler),
	)
}

func (s *Service) getValueHandler(r res.GetRequest) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, valuePrefix) {
		r.NotFound()
		return
	}
	hk := key[valuePrefixLen:]
	result := s.storage.Get(hk)
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

func (s *Service) getHandler(r res.GetRequest) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, prefix) {
		r.NotFound()
		return
	}
	hk := key[prefixLen:]
	result := s.storage.Get(hk)
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
			r.Model(&lookupResult{Value: res.Ref(valuePrefix + hk)})
		default:
			vf.FromValue(v, &iv)
			r.Model(&lookupResult{Value: iv})
		}
	}
}

func (s *Service) deleteHandler(r res.CallRequest) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, prefix) {
		r.NotFound()
		return
	}
	if s.storage.Delete(key[prefixLen:]) {
		r.DeleteEvent()
		r.OK(nil)
	} else {
		r.NotFound()
	}
}

func (s *Service) setHandler(r res.CallRequest) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, prefix) {
		r.NotFound()
		return
	}
	if params, ok := streamer.UnmarshalJSON(r.RawParams(), nil).(dgo.Map); ok {
		changes, err := s.storage.Set(key[prefixLen:], params)
		if err != nil {
			panic(err)
		}
		if changes.Len() > 0 {
			// Send a change event with updated fields
			var cm map[string]interface{}
			vf.FromValue(changes, &cm)
			r.ChangeEvent(cm)
		}

		// Send success response
		r.OK(nil)
		return
	}
	panic(errors.New(`unable to extract model from parameters`))
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
