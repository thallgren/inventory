// Package inventory contains the resgate based inventory service
package inventory

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/change"
	"github.com/puppetlabs/inventory/iapi"
	"github.com/sirupsen/logrus"
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
	resService *res.Service
	storage    iapi.Storage
}

// NewService creates a new Resgate service that will use the givne storage
func NewService(rs *res.Service, storage iapi.Storage) *Service {
	s := &Service{rs, storage}
	// Add handlers for "lookup.$key" models. The response will always be a struct
	// containing a value.
	rs.Handle(
		`>`,
		res.Access(res.AccessGranted),
		res.GetResource(s.getHandler),
		res.Set(s.setHandler),
		res.Call("delete", s.deleteHandler),
	)
	return s
}

func (s *Service) getComplex(r res.GetRequest) {
	key := r.ResourceName()
	hk := key[valuePrefixLen:]
	mods, result := s.storage.Get(hk)
	s.Modifications(mods)
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
	mods, result := s.storage.Query(key, qvs)
	s.Modifications(mods)
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
	mods, result := s.storage.Get(key)
	s.Modifications(mods)
	if result == nil {
		r.NotFound()
	} else {
		dc := streamer.DataCollector()
		streamer.New(nil, streamer.DefaultOptions()).Stream(result, dc)
		v := dc.Value()
		switch v := v.(type) {
		case dgo.Array:
			r.Collection(arrayToCollection(v, r.ResourceName()+`.`))
		case dgo.Map:
			r.Model(mapToModel(v, r.ResourceName()+`.`))
		default:
			var iv interface{}
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
	mods, ok := s.storage.Delete(key[prefixLen:])
	s.Modifications(mods)
	if ok {
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
		mods, err := s.storage.Set(key[prefixLen:], params)
		if err != nil {
			panic(err)
		}
		s.Modifications(mods)

		// Send success response
		r.OK(nil)
		return
	}
	panic(errors.New(`unable to extract model from parameters`))
}

func (s *Service) Modifications(mods []*change.Modification) {
	for _, mod := range mods {
		s.sendModificationEvent(mod)
	}
}

func (s *Service) sendModificationEvent(mod *change.Modification) {
	rid := prefix + mod.ResourceName
	r, err := s.resService.Resource(rid)
	if err != nil {
		panic(err)
	}
	switch mod.Type {
	case change.Delete:
		logrus.Debugf(`Delete: %s`, rid)
		r.DeleteEvent()
	case change.Reset:
		logrus.Debugf(`Reset: %s`, rid)
		r.ResetEvent()
	case change.Create:
		v := convertValue(rid, mod.Value)
		logrus.Debugf(`Create: %s = %v`, rid, v)
		r.CreateEvent(v)
	case change.Change:
		m := make(map[string]interface{})
		mod.Value.(dgo.Map).EachEntry(func(e dgo.MapEntry) {
			k := e.Key().String()
			if e.Value() == change.Deleted {
				m[k] = res.DeleteAction
			} else {
				m[k] = convertValue(rid+`.`+k, e.Value())
			}
		})
		logrus.Debugf(`Change: %s = %v`, rid, m)
		r.ChangeEvent(m)
	case change.Add:
		v := convertValue(rid+`.`+strconv.Itoa(mod.Index), mod.Value)
		logrus.Debugf(`Add: %s[%d] = %v`, rid, mod.Index, v)
		r.AddEvent(v, mod.Index)
	case change.Remove:
		logrus.Debugf(`Remove: %s[%d]`, rid, mod.Index)
		r.RemoveEvent(mod.Index)
	case change.Set:
		// NOTE: Some confusion here. What should be sent when a collection value is replaced?
		//  see ticket: https://github.com/resgateio/resgate/issues/145
		v := convertValue(rid+`.`+strconv.Itoa(mod.Index), mod.Value)
		logrus.Debugf(`Set: %s[%d] = %v`, rid, mod.Index, v)
		r.RemoveEvent(mod.Index)
		r.AddEvent(v, mod.Index)
	}
}

func convertValue(key string, result dgo.Value) interface{} {
	dc := streamer.DataCollector()
	streamer.New(nil, streamer.DefaultOptions()).Stream(result, dc)
	v := dc.Value()
	var iv interface{}
	if change.IsComplex(v) {
		iv = res.Ref(key)
	} else {
		vf.FromValue(v, &iv)
	}
	return iv
}
