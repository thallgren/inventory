// Package bolt contains the storage for the bolt inventory version 2 file
package bolt

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/typ"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/iapi"
	"github.com/puppetlabs/inventory/query"
	"github.com/puppetlabs/inventory/yaml"
)

var inventoryFileType dgo.Type
var namePattern = tf.Pattern(regexp.MustCompile(`\A[a-z0-9_][a-z0-9_-]*\z`))
var asciiPattern = tf.Pattern(regexp.MustCompile(`\A[[:ascii:]]+\z`))
var dataMap = tf.Map(asciiPattern, tf.Parse(`data`))

func init() {
	tf.AddDefaultAliases(func(am dgo.AliasAdder) {
		am.Add(namePattern, vf.String(`namePattern`))
		am.Add(asciiPattern, vf.String(`asciiPattern`))
		am.Add(dataMap, vf.String(`dataMap`))
		tf.ParseFile(am, `internal`, `targetMap={
			alias?: namePattern|[]namePattern,
			config?: dataMap,
			facts?: dataMap,
			features?: []asciiPattern,
			name?: namePattern,
			uri?: asciiPattern,
			vars?: dataMap
		}`)

		tf.ParseFile(am, `internal`, `groupMap={
			config?: dataMap,
			facts?: dataMap,
			features?: []asciiPattern,
			groups?: []groupMap,
			name: namePattern,
			targets?: [](targetMap|asciiPattern),
			vars?: dataMap,
		}`)

		inventoryFileType = tf.ParseFile(am, `internal`, `inventoryMap={
			version: 2,
			config?: dataMap,
			facts?: dataMap,
			features?: []asciiPattern,
			groups?: []groupMap,
			targets?: [](targetMap|asciiPattern),
			vars?: dataMap,
		}`).(dgo.Type)
	})
}

const minRefresh = time.Second * 1

type storage struct {
	lock     sync.Mutex
	path     string
	age      time.Time
	contents Group   // the "all" group
	targets  dgo.Map // resolved targets
}

// NewStorage creates a new storage for the bolt inventory version 2 file at the given path
func NewStorage(path string) iapi.Storage {
	return &storage{path: path}
}

func (s *storage) Delete(_ string) ([]*iapi.Modification, bool) {
	panic("implement me")
}

func (s *storage) Get(key string) ([]*iapi.Modification, dgo.Value) {
	mods := s.Refresh()
	value := s.dig(strings.Split(key, `.`), 0, s.targets)
	return mods, value
}

func (s *storage) dig(parts []string, depth int, c dgo.Value) dgo.Value {
	n := len(parts) - depth
	if n == 0 {
		return c
	}
	v := getAtKey(c, parts, depth)
	switch v.(type) {
	case dgo.Map, dgo.Array:
		if n > 1 {
			v = s.dig(parts, depth+1, v)
		}
	}
	return v
}

// getAtKey uses the given key to access a value in the given collection, which is assumed
// to be a Map or an Array. The given key must be parsable to a positive integer in case the
// collection is an Array. The function returns nil when no value is found.
func getAtKey(c dgo.Value, parts []string, depth int) dgo.Value {
	k := parts[depth]
	if depth == 0 && parts[0] == `targets` {
		return c
	}
	switch c := c.(type) {
	case dgo.Array:
		if i, err := strconv.Atoi(k); err == nil {
			if i >= 0 && i < c.Len() {
				return c.Get(i)
			}
		}
	case dgo.Map:
		return c.Get(k)
	}
	return nil
}

func (s *storage) Query(key string, q dgo.Map) ([]*iapi.Modification, query.Result) {
	mods, v := s.Get(key)
	if a, ok := v.(dgo.Map); ok {
		match := q.Get(`match`) // required parameter
		rx := regexp.MustCompile(regexp.QuoteMeta(match.(dgo.String).GoString()))
		qr := query.NewResult(false)
		a.EachEntry(func(e dgo.MapEntry) {
			if ks, ok := e.Key().(dgo.String); ok && rx.FindString(ks.GoString()) != `` {
				qr.Add(ks, e.Value())
			}
		})
		return mods, qr
	}
	return mods, nil
}

func (s *storage) QueryKeys(key string) []query.Param {
	parts := strings.Split(key, `.`)
	last := parts[len(parts)-1]
	qks := map[string][]query.Param{
		`targets`: {
			query.NewParam(`match`, typ.String, true),
		},
		`groups`: {
			query.NewParam(`match`, typ.String, true),
		},
	}[last]
	if qks == nil {
		qks = []query.Param{}
	}
	return qks
}

// refreshContents read the inventory yaml file on disk if the cache is deemed to be out of date. The
// cache is considered up to date if the last known state of the file is less than the value of the
// const minRefresh, or if a new stat call shows that the file hasn't been updated.
func (s *storage) Refresh() []*iapi.Modification {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := time.Now()
	if s.contents == nil {
		s.age = now
		return s.readInventory()
	}

	if now.Sub(s.age) < minRefresh {
		return nil
	}

	cs, err := os.Stat(s.path)
	if err != nil {
		panic(err)
	}

	if cs.ModTime().After(s.age) {
		s.age = now
		return s.readInventory()
	}
	return nil
}

func (s *storage) readInventory() []*iapi.Modification {
	data := yaml.Read(s.path)
	if !inventoryFileType.Instance(data) {
		panic(tf.IllegalAssignment(inventoryFileType, data))
	}
	all := NewGroup(nil, data.With(nameV, `all`))
	ats := vf.MutableMap()
	all.CollectTargets(ats)
	all.ResolveStringTargets(ats)
	s.contents = all

	// Send events to all target subscribers
	ats = ats.Map(func(e dgo.MapEntry) interface{} {
		return MergeTargets(e.Value().(dgo.Array))
	})
	if s.targets == nil {
		s.targets = ats
		return []*iapi.Modification{}
	}
	return modifyMap(``, s.targets, ats, nil)
}

type keyBuilder struct {
	strings.Builder
}

func (sb *keyBuilder) makeKey(p string, k string) string {
	if p == `` {
		return k
	}
	sb.Reset()
	_, _ = sb.WriteString(p)
	_ = sb.WriteByte('.')
	_, _ = sb.WriteString(k)
	return sb.String()
}

func isComplex(v dgo.Value) bool {
	switch v.(type) {
	case dgo.Map, dgo.Array:
		return true
	default:
		return false
	}
}

func modifyMap(p string, a, b dgo.Map, mods []*iapi.Modification) []*iapi.Modification {
	changedProps := vf.MutableMap()
	sb := &keyBuilder{}
	var ktm dgo.Array
	a.EachEntry(func(e dgo.MapEntry) {
		if b.ContainsKey(e.Key()) {
			return
		}
		if isComplex(e.Value()) {
			mods = append(mods, &iapi.Modification{ResourceName: sb.makeKey(p, e.Key().String()), Type: iapi.Delete})
		} else {
			changedProps.Put(e.Key(), iapi.Deleted)
		}
		if ktm == nil {
			ktm = vf.MutableValues()
		}
		ktm.Add(e.Key())
	})
	if ktm != nil {
		a.RemoveAll(ktm)
	}

	b.EachEntry(func(e dgo.MapEntry) {
		k := e.Key()
		v := e.Value()
		old := a.Get(k)
		if v.Equals(old) {
			return
		}
		sk := sb.makeKey(p, k.String())
		switch old := old.(type) {
		case nil:
			if isComplex(e.Value()) {
				mods = append(mods, &iapi.Modification{ResourceName: sk, Value: v, Type: iapi.Create})
			} else {
				changedProps.Put(k, v)
			}
			a.Put(k, v)
		case dgo.Map:
			if nv, ok := v.(dgo.Map); ok {
				mods = modifyMap(sk, old, nv, mods)
			} else {
				// Change from map to something else
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
				a.Put(k, v)
			}
		case dgo.Array:
			if nv, ok := v.(dgo.Array); ok {
				mods = modifyArray(sk, old, nv, mods)
			} else {
				// Change from array to something else
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
				a.Put(k, v)
			}
		default:
			if isComplex(e.Value()) {
				// Change from simple to complex
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
			} else {
				changedProps.Put(k, v)
			}
			a.Put(k, v)
		}
	})
	if changedProps.Len() > 0 {
		mods = append(mods, &iapi.Modification{ResourceName: p, Value: changedProps, Type: iapi.Change})
	}
	return mods
}

func modifyArray(p string, a, b dgo.Array, mods []*iapi.Modification) []*iapi.Modification {
	sb := &keyBuilder{}
	if a.Len() > b.Len() {
		t := a.Len()
		for i := b.Len(); i < t; i++ {
			if isComplex(a.Get(i)) {
				mods = append(mods, &iapi.Modification{ResourceName: sb.makeKey(p, strconv.Itoa(i)), Type: iapi.Delete})
			} else {
				mods = append(mods, &iapi.Modification{ResourceName: p, Index: i, Type: iapi.Remove})
			}
		}
	}

	b.EachWithIndex(func(v dgo.Value, i int) {
		var old dgo.Value
		if i < a.Len() {
			old = a.Get(i)
			if v.Equals(old) {
				return
			}
		}
		sk := sb.makeKey(p, strconv.Itoa(i))
		switch old := old.(type) {
		case nil:
			if isComplex(a.Get(i)) {
				// Adding new resource
				mods = append(mods, &iapi.Modification{ResourceName: sk, Value: v, Type: iapi.Create})
			} else {
				mods = append(mods, &iapi.Modification{ResourceName: p, Index: i, Value: v, Type: iapi.Add})
			}
			a.Add(v)
		case dgo.Map:
			if nv, ok := v.(dgo.Map); ok {
				mods = modifyMap(sk, old, nv, mods)
			} else {
				// Change from map to something else
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
				a.Set(i, v)
			}
		case dgo.Array:
			if nv, ok := v.(dgo.Array); ok {
				mods = modifyArray(sk, old, nv, mods)
			} else {
				// Change from array to something else
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
				a.Set(i, v)
			}
		default:
			if isComplex(v) {
				// Change from simple to complex
				mods = append(mods, &iapi.Modification{ResourceName: sk, Type: iapi.Reset})
			} else {
				mods = append(mods, &iapi.Modification{ResourceName: p, Index: i, Value: v, Type: iapi.Set})
			}
			a.Set(i, v)
		}
	})
	return mods
}

func (s *storage) Set(key string, model dgo.Map) (mods []*iapi.Modification, err error) {
	mods = s.Refresh()
	defer func() {
		if pe := recover(); pe != nil {
			var ok bool
			if err, ok = pe.(error); ok {
				return
			}
			var es string
			if es, ok = pe.(string); ok {
				err = errors.New(es)
				return
			}
			panic(pe)
		}
	}()
	return
}
