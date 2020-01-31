// Package inventory contains the resgate based inventory service
package inventory

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/puppetlabs/inventory/query"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/iapi"
)

// ServiceName is the name of the Resgate service
const ServiceName = `inventory`

const valueKey = `__value`
const prefix = ServiceName + `.`
const prefixLen = len(prefix)

const valuePrefix = ServiceName + `.` + valueKey + `.`
const valuePrefixLen = len(valuePrefix)

type lookupResult struct {
	Value interface{} `json:"value"`
}

// A Service contains all the resgate handles and a storage.
type Service struct {
	storage iapi.Storage
}

// NewService creates a new Resgate service that will use the givne storage
func NewService(storage iapi.Storage) *Service {
	return &Service{storage}
}

// AddHandlers will initialize the given resgate service with the handlers of this Service
func (s *Service) AddHandlers(rs *res.Service) {
	// Add handlers for "lookup.$key" models. The response will always be a struct
	// containing a value.
	rs.Handle(
		`>`,
		res.Access(res.AccessGranted),
		res.GetResource(s.getHandler),
		res.Set(s.setHandler),
		res.Call("delete", s.deleteHandler),
	)
}

func (s *Service) getComplex(r res.GetRequest) {
	key := r.ResourceName()
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
	if strings.HasPrefix(key, valuePrefix) {
		s.getComplex(r)
		return
	}
	if !strings.HasPrefix(key, prefix) {
		r.NotFound()
		return
	}
	hk := key[prefixLen:]
	if r.Query() == `` {
		s.doGet(r, hk)
	} else {
		s.doQuery(r, hk, r.ParseQuery())
	}
}

func (s *Service) normalizeQuery(r res.GetRequest, key string, query url.Values) (string, dgo.Map, bool) {
	// Build a normalized (predictable order) query string
	pqs := strings.Builder{}
	nqs := s.storage.QueryKeys(key)
	qvs := vf.MutableMap()
	for qn := range query {
		found := false
		for _, qp := range nqs {
			if qn == qp.Name() {
				found = true
				break
			}
		}
		if !found {
			r.InvalidQuery(fmt.Sprintf(`unknown parameter '%s'`, qn))
			return ``, nil, false
		}
	}

	for _, qp := range nqs {
		qn := qp.Name()
		qe := query.Get(qn)
		if qe == `` {
			if qp.Required() {
				r.InvalidQuery(fmt.Sprintf(`missing required parameter '%s'`, qn))
				return ``, nil, false
			}
			continue
		}
		qvs.Put(qn, vf.New(qp.Type(), vf.Value(qe)))
		if pqs.Len() > 0 {
			_ = pqs.WriteByte('&')
		}
		_, _ = pqs.WriteString(qn)
		_ = pqs.WriteByte('=')
		_, _ = pqs.WriteString(url.QueryEscape(qe))
	}
	nq := pqs.String()
	return nq, qvs, true
}

func (s *Service) doQuery(r res.GetRequest, key string, query url.Values) {
	nq, qvs, ok := s.normalizeQuery(r, key, query)
	if !ok {
		return
	}
	result := s.storage.Query(key, qvs)
	if result == nil {
		r.NotFound()
	} else {
		switch {
		case result.Singleton():
			var iv interface{}
			dc := streamer.DataCollector()
			streamer.New(nil, streamer.DefaultOptions()).Stream(result.Value(0), dc)
			v := dc.Value()
			vf.FromValue(v, &iv)
			r.Model(&lookupResult{Value: iv})
		case result.IsMap():
			r.QueryModel(queryToModel(result, r.ResourceName()+`.`), nq)
		default:
			r.QueryCollection(queryToCollection(result, r.ResourceName()+`.`), nq)
		}
	}
}

func (s *Service) doGet(r res.GetRequest, key string) {
	result := s.storage.Get(key)
	if result == nil {
		r.NotFound()
	} else {
		dc := streamer.DataCollector()
		streamer.New(nil, streamer.DefaultOptions()).Stream(result, dc)
		v := dc.Value()
		var iv interface{}
		switch v := v.(type) {
		case dgo.Array:
			r.Collection(arrayToCollection(v, r.ResourceName()+`.`))
		case dgo.Map:
			r.Model(mapToModel(v, r.ResourceName()+`.`))
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

func queryToCollection(a query.Result, path string) []interface{} {
	st := streamer.New(nil, streamer.DefaultOptions())
	s := make([]interface{}, a.Len())
	a.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		dc := streamer.DataCollector()
		st.Stream(value, dc)
		value = dc.Value()
		switch value.(type) {
		case dgo.Map, dgo.Array:
			s[index] = res.Ref(path + ref.String())
		default:
			vf.FromValue(value, &s[index])
		}
	})
	return s
}

func queryToModel(a query.Result, path string) map[string]interface{} {
	st := streamer.New(nil, streamer.DefaultOptions())
	ms := make(map[string]interface{}, a.Len())
	a.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		rs := ref.(dgo.String).GoString()
		dc := streamer.DataCollector()
		st.Stream(value, dc)
		value = dc.Value()
		switch value.(type) {
		case dgo.Map, dgo.Array:
			ms[rs] = res.Ref(path + rs)
		default:
			var is interface{}
			vf.FromValue(value, &is)
			ms[rs] = is
		}
	})
	return ms
}
