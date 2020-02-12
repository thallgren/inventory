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

	"github.com/puppetlabs/inventory/change"

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
	lock            sync.Mutex
	path            string    // Path to inventory file
	age             time.Time // Time when file was read from disk
	contents        Group     // the "all" group
	targets         dgo.Map   // merged targets
	unmergedTargets dgo.Map   // targets prior to merge. Map of name <=> array of targets
	aliases         dgo.Map   // map of alias <=> target name
	input           dgo.Map
}

// NewStorage creates a new storage for the bolt inventory version 2 file at the given path
func NewStorage(path string) iapi.Storage {
	return &storage{path: path}
}

func (s *storage) Delete(_ string) ([]*change.Modification, bool) {
	panic("implement me")
}

func (s *storage) Get(key string) ([]*change.Modification, dgo.Value) {
	mods := s.Refresh()
	parts := strings.Split(key, `.`)
	var top dgo.Value
	if parts[0] == `targets` {
		parts = parts[1:]
		top = s.targets.Values()
	} else {
		top = s.targets
	}
	value := dig(parts, top)
	return mods, value
}

func (s *storage) targetsInMatchedGroups(group string) dgo.Map {
	targetNames := vf.MutableMap()
	rx := regexp.MustCompile(regexp.QuoteMeta(group))
	s.contents.FindGroups(rx).Each(func(gv dgo.Value) {
		g := gv.(Group)
		s.unmergedTargets.EachEntry(func(e dgo.MapEntry) {
			if e.Value().(dgo.Array).Any(func(t dgo.Value) bool { return t.(Target).HasParent(g) }) {
				targetNames.Put(e.Key(), vf.True)
			}
		})
	})
	return targetNames
}

func (s *storage) Query(key string, q dgo.Map) ([]*change.Modification, query.Result) {
	mods, v := s.Get(key)
	a, ok := v.(dgo.Array)
	if !ok {
		return mods, nil
	}

	var targetNames dgo.Map
	if group := q.Get(`group`); group != nil {
		targetNames = s.targetsInMatchedGroups(group.String())
	}

	if match := q.Get(`match`); match != nil {
		if targetNames == nil {
			// No limitation on groups, so start with all targets
			targetNames = vf.MapWithCapacity(s.unmergedTargets.Len(), nil)
			s.unmergedTargets.EachKey(func(k dgo.Value) { targetNames.Put(k, vf.True) })
		}

		// limit targetNames using match regexp.
		rx := regexp.MustCompile(regexp.QuoteMeta(match.String()))
		sts := targetNames
		targetNames = vf.MutableMap()
		sts.EachKey(func(n dgo.Value) {
			if rx.FindString(n.String()) != `` {
				targetNames.Put(n, vf.True)
			}
		})

		// also match on aliases
		s.aliases.EachEntry(func(e dgo.MapEntry) {
			if rx.FindString(e.Key().String()) != `` && sts.ContainsKey(e.Value()) {
				targetNames.Put(e.Value(), vf.True)
			}
		})
	}

	if targetNames != nil && targetNames.Len() == 0 {
		return mods, nil
	}

	qr := query.NewResult(false)
	a.EachWithIndex(func(v dgo.Value, i int) {
		m := v.(dgo.Map)
		if targetNames != nil {
			n := m.Get(nameV)
			if n == nil {
				n = m.Get(uriV)
			}
			if !targetNames.ContainsKey(n) {
				return
			}
		}
		qr.Add(vf.Integer(int64(i)), m)
	})
	return mods, qr
}

func (s *storage) QueryKeys(key string) []query.Param {
	parts := strings.Split(key, `.`)
	last := parts[len(parts)-1]
	qks := map[string][]query.Param{
		`targets`: {
			query.NewParam(`match`, typ.String, false),
			query.NewParam(`group`, typ.String, false),
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
func (s *storage) Refresh() []*change.Modification {
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

func (s *storage) readInventory() []*change.Modification {
	data := yaml.Read(s.path)
	if !inventoryFileType.Instance(data) {
		panic(tf.IllegalAssignment(inventoryFileType, data))
	}

	var mods []*change.Modification
	input := data.With(nameV, `all`)
	if s.input == nil {
		s.input = input
	} else {
		mods = change.Map(``, s.input, input, nil)
	}
	all := NewGroup(nil, input)
	ats := vf.MutableMap()
	als := vf.MutableMap()
	all.CollectTargets(ats)
	all.CollectAliases(als)
	all.ResolveStringTargets(als, ats)
	s.unmergedTargets = ats
	s.contents = all
	s.aliases = als

	// Send events to all target subscribers
	ats = ats.Map(func(e dgo.MapEntry) interface{} {
		return MergeTargets(e.Value().(dgo.Array))
	})
	if s.targets == nil {
		s.targets = ats
		return []*change.Modification{}
	}
	return append(mods, change.Map(``, s.targets, ats, nil)...)
}

func (s *storage) Set(key string, model dgo.Map) (mods []*change.Modification, err error) {
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

// dig will into the given value which must be a Map or an Array using the given keys in the given slice.
// It is an error to call this method with an empty keys slice.
func dig(keys []string, v dgo.Value) dgo.Value {
	for _, key := range keys {
		switch c := v.(type) {
		case dgo.Array:
			if i, err := strconv.Atoi(key); err == nil {
				if i >= 0 && i < c.Len() {
					v = c.Get(i)
					continue
				}
			}
		case dgo.Map:
			v = c.Get(key)
			continue
		}
		v = nil
		break
	}
	return v
}
