// Package bolt contains the storage for the bolt inventory version 2 file
package bolt

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/typ"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/change"
	"github.com/puppetlabs/inventory/iapi"
	"github.com/puppetlabs/inventory/query"
	"github.com/puppetlabs/inventory/yaml"
	"github.com/sirupsen/logrus"
)

// Storage is an extension of the iapi.Storage interface that adds the ability to add a
// Watcher to that detects changes to the underlying files.
type Storage interface {
	iapi.Storage

	Watch(func([]*change.Modification)) *fsnotify.Watcher
}

var inventoryFileType dgo.Type
var namePattern = tf.Pattern(regexp.MustCompile(`\A[a-z0-9_][a-z0-9_-]*\z`))
var asciiPattern = tf.Pattern(regexp.MustCompile(`\A[[:ascii:]]+\z`))
var dataMap = tf.Map(asciiPattern, tf.Parse(`data`))

func init() {
	// Add dgo types used for validity checking of the inventory files
	tf.AddDefaultAliases(func(am dgo.AliasAdder) {
		am.Add(namePattern, vf.String(`namePattern`))
		am.Add(asciiPattern, vf.String(`asciiPattern`))
		am.Add(dataMap, vf.String(`dataMap`))

		// The targetMap type describes a target
		tf.ParseFile(am, `internal`, `targetMap={
			alias?: namePattern|[]namePattern,
			config?: dataMap,
			facts?: dataMap,
			features?: []asciiPattern,
			name?: namePattern,
			uri?: asciiPattern,
			vars?: dataMap
		}`)

		// The groupMap type describes a group
		tf.ParseFile(am, `internal`, `groupMap={
			config?: dataMap,
			facts?: dataMap,
			features?: []asciiPattern,
			groups?: []groupMap,
			name: namePattern,
			targets?: [](targetMap|asciiPattern),
			vars?: dataMap,
		}`)

		// The inventoryMap type describes the inventory file
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
const target = `target`
const targets = `targets`

var realmV = vf.String(`realm`)

type storage struct {
	lock       sync.Mutex
	path       string            // Path to directory containing inventory files
	age        time.Time         // Time when directory was checked for new realms
	realmMap   map[string]*realm // the realms. One per inventory file
	targets    dgo.Array         // all merged targets as an array
	targetByID dgo.Map           // all merged, keyed by id
}

type realm struct {
	path            string    // Path to inventory file
	age             time.Time // Time when file was read from disk
	contents        Group     // the realm group
	targets         dgo.Map   // merged targets, keyed by id
	targetsByName   dgo.Map   // merged targets, keyed by name
	unmergedTargets dgo.Map   // targets prior to merge. Map of name <=> array of targets
	aliases         dgo.Map   // map of alias <=> target name
	input           dgo.Map
}

// NewStorage creates a new storage for the bolt inventory version 2 file at the given path
func NewStorage(path string) Storage {
	return &storage{path: path}
}

func (s *storage) Delete(_ string) ([]*change.Modification, bool) {
	panic("implement me")
}

func (s *storage) Get(key string) ([]*change.Modification, dgo.Value) {
	mods := s.refresh()
	parts := strings.Split(key, `.`)
	if len(parts) == 0 {
		return mods, nil
	}
	var result dgo.Value
	p0 := parts[0]
	switch p0 {
	case target:
		if len(parts) >= 2 {
			result = s.targetByID.Get(parts[1])
			if result != nil && len(parts) > 2 {
				result = dig(parts[2:], result)
			}
		}
	case targets:
		result = s.targets
		if len(parts) > 1 {
			result = dig(parts[1:], result)
		}
	default:
		if realm, ok := s.realmMap[p0]; ok {
			result = realm.get(parts[1:])
		}
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
		m := v.(Target)
		n := m.Name()
		if n == nil {
			n = m.URI()
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

func (s *storage) watchFunc(watcher *fsnotify.Watcher, onModify func([]*change.Modification)) {
	for {
		select {
		case event, ok := <-watcher.Events:
			switch {
			case !ok:
				return
			case event.Op&(fsnotify.Write) != 0:
				if strings.HasSuffix(event.Name, `.yaml`) {
					mods := s.refreshRealms()
					if len(mods) > 0 {
						onModify(mods)
					}
				}
			case event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0:
				mods := s.refresh()
				if len(mods) > 0 {
					onModify(mods)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logrus.Error("error:", err)
		}
	}
}

func (s *storage) Watch(onModify func([]*change.Modification)) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	go s.watchFunc(watcher, onModify)

	err = watcher.Add(s.path)
	if err != nil {
		log.Fatal(err)
	}
	return watcher
}

func (s *storage) refresh() []*change.Modification {
	s.lock.Lock()
	defer s.lock.Unlock()

	fis, err := ioutil.ReadDir(s.path)
	if err != nil {
		panic(err)
	}

	initial := s.realmMap == nil
	if initial {
		s.realmMap = make(map[string]*realm, len(fis))
	}

	fiNames := make(map[string]bool, len(fis))
	changed := false
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

		fiNames[rn] = true
		if _, ok := s.realmMap[rn]; !ok {
			rp := filepath.Join(s.path, fi.Name())
			s.realmMap[rn] = &realm{path: rp}
			logrus.Debugf("added file %s as realm %s", rp, rn)
			changed = true
		}
	}

	for _, rn := range s.realmNames() {
		if !fiNames[rn] {
			logrus.Debugf("removed realm %s", rn)
			delete(s.realmMap, rn)
			changed = true
		}
	}

	mods := s.readRealms(changed)
	if initial {
		logrus.Debugf("dir %s initialized at: %s", s.path, s.age)
		mods = nil
	}
	return mods
}

func (s *storage) refreshRealms() []*change.Modification {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.readRealms(false)
}

func (s *storage) readRealms(changed bool) []*change.Modification {
	all := vf.MutableMap()
	for _, realmName := range s.realmNames() {
		realm := s.realmMap[realmName]
		if realm.refresh() {
			changed = true
			if realm.targets == nil {
				logrus.Debugf("removed realm %s", realmName)
				delete(s.realmMap, realmName)
				continue
			}
		}
		all.PutAll(realm.targets.Copy(false))
	}

	s.targetByID = all
	if s.targets == nil {
		s.targets = all.Values()
		return nil
	}
	if changed {
		return change.Array(`targets`, s.targets, all.Values(), nil)
	}
	return nil
}

// realmNames returns all realm names alphabetically sorted
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

// realmNames returns all realms alphabetically sorted
func (s *storage) realms() []*realm {
	ns := s.realmNames()
	rs := make([]*realm, len(ns))
	for i := range ns {
		rs[i] = s.realmMap[ns[i]]
	}
	return rs
}

func (s *storage) Set(key string, model dgo.Map) (mods []*change.Modification, err error) {
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
	mods, v := s.Get(key)
	if v != nil {
		if t, ok := v.(dgo.Map); ok {
			if id, ok := t.Get(idV).(dgo.String); ok {
				rn, n := splitID(id.GoString())
				if realm, ok := s.realmMap[rn]; ok {
					return realm.applyChange(n, model)
				}
			}
		}
	}
	return mods, iapi.NotFound(key)
}

func (r *realm) applyChange(targetID string, model dgo.Map) (mods []*change.Modification, err error) {
	// if targets, ok := r.unmergedTargets.Get(targetId).(dgo.Array); ok {
	//
	// }
	return nil, iapi.NotFound(targetID)
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
		top = r.targetsByName
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
func (r *realm) refresh() bool {
	now := time.Now()
	if r.contents == nil {
		r.age = now
		r.readInventory()
		return true
	}

	if now.Sub(r.age) < minRefresh {
		return false
	}

	cs, err := os.Stat(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.targets = nil // Gone
			return true
		}
		panic(err)
	}

	if cs.ModTime().After(r.age) {
		logrus.Debugf("file %s modified at: %s, last refresh at: %s", r.path, cs.ModTime(), r.age)
		r.age = now
		r.readInventory()
		return true
	}
	r.age = now
	return false
}

func (r *realm) readInventory() {
	defer func() {
		if e := recover(); e != nil {
			logrus.Errorf(`unable to read inventory from %v: %s`, e, r.path)
		}
	}()

	data := yaml.Read(r.path)
	if !inventoryFileType.Instance(data) {
		panic(tf.IllegalAssignment(inventoryFileType, data))
	}

	fn := filepath.Base(r.path)
	ext := filepath.Ext(fn)
	r.input = data.With(nameV, fn[:len(fn)-len(ext)])
	all := NewGroup(nil, r.input)
	ats := vf.MutableMap()
	als := vf.MutableMap()
	all.CollectTargets(ats)
	all.CollectAliases(als)
	all.ResolveStringTargets(als, ats)
	ats.Freeze()
	als.Freeze()
	r.unmergedTargets = ats
	r.contents = all
	r.aliases = als

	tgm := vf.MutableMap()
	tgn := vf.MutableMap()
	ats.EachEntry(func(e dgo.MapEntry) {
		merged := r.mergeTargets(e.Value().(dgo.Array))
		tgm.Put(merged.ID(), merged)
		if name := merged.Name(); name != nil {
			tgn.Put(name, merged)
		}
	})
	tgm.Freeze()
	tgn.Freeze()
	r.targets = tgm
	r.targetsByName = tgn
}

func splitID(id string) (string, string) {
	v, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		panic(err)
	}
	vs := string(v)
	di := strings.IndexByte(vs, '.')
	if di < 1 {
		panic(errors.New(`invalid ID`))
	}
	return vs[:di], vs[di+1:]
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
		case Target:
			v = c.Input().Get(key)
			continue
		case dgo.Map:
			v = c.Get(key)
			continue
		}
		v = nil
		break
	}
	return v
}
