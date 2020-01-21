package inventory

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofrs/flock"
	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
	"github.com/lyraproj/dgoyaml/yaml"
)

const ServiceName = `inventory`
const lookup = `lookup`
const lookupValue = `value`
const setValue = `set`

const prefix = ServiceName + `.` + lookup + `.`
const prefixLen = len(prefix)

const valuePrefix = ServiceName + `.` + lookupValue + `.`
const valuePrefixLen = len(valuePrefix)

const valueKey = `__value`

type lookupResult struct {
	Value interface{} `json:"value"`
}

type Service struct {
	dataDir string
	hns     []string
}

func NewService(dataDir string, hierarchyNames ...string) *Service {
	return &Service{dataDir: dataDir, hns: hierarchyNames}
}

func (s *Service) DataDir() string {
	return s.dataDir
}

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

func (s *Service) children(r res.ModelRequest) {
	dir := s.DataDir()
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.NotFound()
			return
		}
		r.Error(res.InternalError(err))
		return
	}

	domains := vf.MutableMap()
	for _, file := range files {
		if file.IsDir() {
			data, err := ioutil.ReadFile(filepath.Join(dir, file.Name(), `data.yaml`))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				r.Error(res.InternalError(err))
				return
			}
			dv, err := yaml.Unmarshal(data)
			if err != nil {
				r.Error(res.InternalError(err))
				return
			}
			if dh, ok := dv.(dgo.Map); ok {
				domains.Put(file.Name(), dh.Get(`url`))
			}
		}
	}
	r.Model(mapToModel(domains, r.ResourceName()+`.`))
}

func (s *Service) getValueHandler(r res.GetRequest) {
	key := r.ResourceName()
	if !strings.HasPrefix(key, valuePrefix) {
		r.NotFound()
		return
	}
	hk := key[valuePrefixLen:]
	result := s.lookupValue(hk)
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
	result := s.lookupValue(hk)
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
	if s.deleteValue(key[prefixLen:]) {
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
		if value, ok := params.(dgo.Map); ok {
			changes := s.setValue(key[prefixLen:], value)
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
	}
	panic(errors.New(`unable to extract model from parameters`))
}

func (s *Service) lookupValue(key string) dgo.Value {
	parts := strings.Split(key, `.`)
	pf := s.readData(parts)
	if pf != nil {
		return pf.Get(valueKey)
	}
	lp := len(parts) - 1
	last := parts[lp]
	parts = parts[:lp]
	if pf = s.readData(parts); pf == nil {
		return nil
	}
	if v := pf.Get(last); v != nil {
		return v
	}
	if lp < len(s.hns) && last == s.hns[lp] {
		// Collect names of subdirectories.
		return s.readChildMap(parts).Merge(pf.WithoutAll(vf.Values(valueKey)))
	}
	return nil
}

func (s *Service) setValue(key string, value dgo.Map) dgo.Map {
	parts := strings.Split(key, `.`)
	path := filepath.Join(s.DataDir(), filepath.Join(parts...), `data.yaml`)
	var pf dgo.Map
	lock := flock.New(path)
	err := lock.RLock()
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	} else {
		defer lock.Close()
		pf = readExistingYaml(path).Copy(false) // read, then thaw frozen map
	}

	var changes dgo.Map
	if pf != nil {
		changes = vf.MutableMap()
		pf = pf.Copy(false)
		value.EachEntry(func(e dgo.MapEntry) {
			if !e.Value().Equals(pf.Put(e.Key(), e.Value())) {
				changes.Put(e.Key(), e.Value())
			}
		})
		pf = pf.Merge(value)
	} else {
		pf = value
		changes = value
	}
	yml, err := yaml.Marshal(pf)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(path, yml, 0644)
	if err != nil {
		panic(err)
	}
	return changes
}

func (s *Service) deleteValue(key string) bool {
	parts := strings.Split(key, `.`)
	lp := len(parts) - 1
	if lp < 1 {
		return false
	}
	key = parts[lp]
	parts = parts[:lp]
	path := filepath.Join(s.DataDir(), filepath.Join(parts...), `data.yaml`)
	lock := flock.New(path)
	err := lock.RLock()
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}

	defer lock.Close()
	pf := readExistingYaml(path).Copy(false) // read, then thaw frozen map
	if pf.Remove(key) != nil {
		yml, err := yaml.Marshal(pf)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(path, yml, 0644)
		if err != nil {
			panic(err)
		}
		return true
	}
	return false
}

func (s *Service) readChildMap(parts []string) dgo.Map {
	dir := filepath.Join(s.DataDir(), filepath.Join(parts...))
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		panic(err)
	}

	children := vf.MutableMap()
	for _, file := range files {
		if file.IsDir() {
			dh := s.readData(append(parts, file.Name()))
			children.Put(file.Name(), dh.Get(valueKey))
		}
	}
	return children
}

func (s *Service) readData(parts []string) dgo.Map {
	path := filepath.Join(s.DataDir(), filepath.Join(parts...), `data.yaml`)
	lock := flock.New(path)
	if err := lock.RLock(); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		panic(err)
	}
	defer lock.Close()
	return readExistingYaml(path)
}

func readExistingYaml(path string) dgo.Map {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	dv, err := yaml.Unmarshal(data)
	if err != nil {
		panic(err)
	}
	if dh, ok := dv.(dgo.Map); ok {
		return dh
	}
	panic(fmt.Errorf(`the file %q does not contain a map of values`, path))
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
