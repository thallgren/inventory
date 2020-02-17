// Package bolt contains the storage for the bolt inventory version 2 file
package bolt

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
const targets = `targets`

var realmV = vf.String(`realm`)

type storage struct {
	lock     sync.Mutex
	path     string            // Path to directory containing inventory files
	age      time.Time         // Time when directory was checked for new realms
	realmMap map[string]*realm // the realms. One per inventory file
}

type realm struct {
	path            string    // Path to inventory file
	age             time.Time // Time when file was read from disk
	contents        Group     // the realm group
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
	if len(parts) == 0 {
		return mods, nil
	}
	var result dgo.Value
	p0 := parts[0]
	if p0 == targets {
		// Flatten all targets into an array and ensure that all targets contain the realm
		all := vf.MutableValues()
		for _, realm := range s.realms() {
			rn := realm.contents.Name()
			if trgs, ok := realm.get([]string{p0}).(dgo.Array); ok {
				trgs.Each(func(tv dgo.Value) {
					t := tv.(dgo.Map)
					t.Put(realmV, rn)
					all.Add(t)
				})
			}
		}
		if len(parts) > 1 {
			result = dig(parts[1:], all)
		} else {
			result = all
		}
	} else if realm, ok := s.realmMap[p0]; ok {
		result = realm.get(parts[1:])
	}
	return mods, result
}

func (s *storage) matchingTargets(realmMatch, groupMatch string) dgo.Map {
	targetNames := vf.MutableMap()
	var rs []*realm
	if realmMatch == `` {
		rs = s.realms()
	} else {
		rrx := regexp.MustCompile(regexp.QuoteMeta(realmMatch))
		for _, rn := range s.realmNames() {
			if rrx.FindString(rn) != `` {
				rs = append(rs, s.realmMap[rn])
			}
		}
	}

	var grx *regexp.Regexp
	if groupMatch != `` {
		grx = regexp.MustCompile(regexp.QuoteMeta(groupMatch))
	}
	for _, r := range rs {
		r.matchingTargets(grx, targetNames)
	}
	return targetNames
}

func (s *storage) Query(key string, q dgo.Map) ([]*change.Modification, query.Result) {
	mods, v := s.Get(key)
	a, ok := v.(dgo.Array)
	if !ok || a.Len() == 0 {
		return mods, nil
	}

	stringParam := func(parameterName string) string {
		if s, ok := q.Get(parameterName).(dgo.String); ok {
			return s.GoString()
		}
		return ``
	}

	targetNames := s.matchingTargets(stringParam(`realm`), stringParam(`group`))
	if targetNames.Len() == 0 {
		return mods, nil
	}

	targetMatch := stringParam(`target`)
	if targetMatch != `` {
		// limit targetNames using match regexp.
		rx := regexp.MustCompile(regexp.QuoteMeta(targetMatch))
		sts := targetNames
		targetNames = vf.MutableMap()
		sts.EachKey(func(n dgo.Value) {
			if rx.FindString(n.String()) != `` {
				targetNames.Put(n, vf.True)
			}
		})
		if targetNames.Len() == 0 {
			return mods, nil
		}
	}

	qr := query.NewResult(false)
	a.EachWithIndex(func(v dgo.Value, i int) {
		m := v.(dgo.Map)
		n := m.Get(nameV)
		if n == nil {
			n = m.Get(uriV)
		}
		if !targetNames.ContainsKey(n) {
			return
		}
		qr.Add(vf.Integer(int64(i)), m)
	})
	return mods, qr
}

func (s *storage) QueryKeys(key string) []query.Param {
	parts := strings.Split(key, `.`)
	switch {
	case len(parts) == 1 && parts[0] == targets:
		return []query.Param{
			query.NewParam(`target`, typ.String, false),
			query.NewParam(`group`, typ.String, false),
			query.NewParam(`realm`, typ.String, false),
		}
	case len(parts) == 2 && parts[1] == targets: // prefixed with realm
		return []query.Param{
			query.NewParam(`target`, typ.String, false),
			query.NewParam(`group`, typ.String, false),
		}
	default:
		return nil
	}
}

func (s *storage) Refresh() []*change.Modification {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := time.Now()
	if s.realmMap == nil {
		s.age = now
		s.readRealms()
		return nil
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
		return s.readRealms()
	}

	mods := make([]*change.Modification, 0)
	for _, realm := range s.realmMap {
		mods = append(mods, realm.Refresh()...)
	}
	return mods
}

func (s *storage) readRealms() (mods []*change.Modification) {
	fis, err := ioutil.ReadDir(s.path)
	if err != nil {
		panic(err)
	}

	if s.realmMap == nil {
		s.realmMap = make(map[string]*realm, len(fis))
	}

	fns := make(map[string]bool, len(fis))
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		rn := fi.Name()
		switch {
		case strings.HasSuffix(rn, `.yaml`):
			rn = rn[:len(rn)-5]
		case strings.HasSuffix(rn, `.yml`):
			rn = rn[:len(rn)-4]
		default:
			continue
		}

		fns[rn] = true
		if r, ok := s.realmMap[rn]; ok {
			mods = append(mods, r.Refresh()...)
		} else {
			r = &realm{path: filepath.Join(s.path, fi.Name())}
			r.Refresh()
			s.realmMap[rn] = r
			mods = append(mods, &change.Modification{ResourceName: rn, Type: change.Create, Value: vf.Map()})
		}
	}
	for _, fn := range s.realmNames() {
		if _, ok := fns[fn]; !ok {
			// a realm has been removed
			mods = append(mods, &change.Modification{ResourceName: fn, Type: change.Delete})
		}
	}
	return mods
}

func (s *storage) realmNames() []string {
	ns := make([]string, len(s.realmMap))
	i := 0
	for n := range s.realmMap {
		ns[i] = n
		i++
	}
	sort.Strings(ns)
	return ns
}

func (s *storage) realms() []*realm {
	ns := s.realmNames()
	rs := make([]*realm, len(ns))
	for i := range ns {
		rs[i] = s.realmMap[ns[i]]
	}
	return rs
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

func (r *realm) get(parts []string) dgo.Value {
	if len(parts) == 0 {
		return nil
	}
	var top dgo.Value
	if parts[0] == targets {
		parts = parts[1:]
		top = r.targets.Values()
	} else {
		top = r.targets
	}
	value := dig(parts, top)
	return value
}

// matchingTargets will add the name of all targets that, among its parents, have a group whose name matches the given
// pattern. If the pattern is nil, all groups will match.
func (r *realm) matchingTargets(groupNamePattern *regexp.Regexp, targetNames dgo.Map) {
	if groupNamePattern == nil {
		r.unmergedTargets.EachKey(func(tn dgo.Value) {
			targetNames.Put(tn, vf.True)
		})
		return
	}

	r.contents.FindGroups(groupNamePattern).Each(func(gv dgo.Value) {
		g := gv.(Group)
		r.unmergedTargets.EachEntry(func(e dgo.MapEntry) {
			if e.Value().(dgo.Array).Any(func(t dgo.Value) bool { return t.(Target).HasParent(g) }) {
				targetNames.Put(e.Key(), vf.True)
			}
		})
	})
}

// refreshContents read the inventory yaml file on disk if the cache is deemed to be out of date. The
// cache is considered up to date if the last known state of the file is less than the value of the
// const minRefresh, or if a new stat call shows that the file hasn't been updated.
func (r *realm) Refresh() []*change.Modification {
	now := time.Now()
	if r.contents == nil {
		r.age = now
		return r.readInventory()
	}

	if now.Sub(r.age) < minRefresh {
		return nil
	}

	cs, err := os.Stat(r.path)
	if err != nil {
		panic(err)
	}

	if cs.ModTime().After(r.age) {
		r.age = now
		return r.readInventory()
	}
	return nil
}

func (r *realm) readInventory() []*change.Modification {
	data := yaml.Read(r.path)
	if !inventoryFileType.Instance(data) {
		panic(tf.IllegalAssignment(inventoryFileType, data))
	}

	var mods []*change.Modification
	fn := filepath.Base(r.path)
	ext := filepath.Ext(fn)
	input := data.With(nameV, fn[:len(fn)-len(ext)])
	if r.input == nil {
		r.input = input
	} else {
		mods = change.Map(``, r.input, input, nil)
	}
	all := NewGroup(nil, input)
	ats := vf.MutableMap()
	als := vf.MutableMap()
	all.CollectTargets(ats)
	all.CollectAliases(als)
	all.ResolveStringTargets(als, ats)
	r.unmergedTargets = ats
	r.contents = all
	r.aliases = als

	// Send events to all target subscribers
	ats = ats.Map(func(e dgo.MapEntry) interface{} {
		return MergeTargets(e.Value().(dgo.Array))
	})
	if r.targets == nil {
		r.targets = ats
		return []*change.Modification{}
	}
	return append(mods, change.Map(``, r.targets, ats, nil)...)
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
