// Package bolt contains the storage for the bolt inventory version 2 file
package bolt

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/puppetlabs/inventory/query"

	"github.com/lyraproj/dgo/typ"

	"github.com/puppetlabs/inventory/iapi"
	"github.com/puppetlabs/inventory/yaml"

	"github.com/lyraproj/dgo/util"

	"github.com/sirupsen/logrus"

	"github.com/lyraproj/dgo/tf"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/vf"
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

type groupOrTarget interface {
	dgo.Value
	asMap() dgo.Map
	get(key string) (dgo.Value, bool)
	set(key string, value dgo.Value) bool
	name() dgo.String
	init(dgo.Map)
	initMap(includeName bool) dgo.Map
}

func eq(a, b dgo.Value) bool {
	return a == b || a != nil && a.Equals(b)
}

var createFunc = func(value dgo.Value, v groupOrTarget) dgo.Value {
	v.init(value.(dgo.Map))
	return v
}

var initMapFunc = func(v dgo.Value) dgo.Value {
	return v.(groupOrTarget).initMap(true)
}

type data struct {
	input    dgo.Map
	config   dgo.Map
	facts    dgo.Map
	vars     dgo.Map
	features dgo.Array
}

var nameV = vf.String(`name`)

func (d *data) assign(k string, t dgo.Type, v dgo.Value, rq bool) bool {
	if v == nil {
		if rq {
			panic(fmt.Errorf(`cannot assign nil to required field %s`, k))
		}
		return d.input.Remove(k) != nil
	}
	if !t.Instance(v) {
		panic(fmt.Errorf(`cannot assign %s to field %s %s`, v, k, t))
	}
	o := d.input.Put(k, v)
	return !v.Equals(o)
}

func (d *data) stringAssign(k string, t dgo.Type, v dgo.Value, rq bool, p *dgo.String) bool {
	if c := d.assign(k, t, v, rq); c {
		if v == nil {
			*p = nil
		} else {
			*p = v.(dgo.String)
		}
		return true
	}
	return false
}

func (d *data) mapAssign(k string, t dgo.Type, v dgo.Value, rq bool, p *dgo.Map) bool {
	if c := d.assign(k, t, v, rq); c {
		if v == nil {
			*p = nil
		} else {
			*p = v.(dgo.Map)
		}
		return true
	}
	return false
}

func (d *data) arrayAssign(k string, t dgo.Type, v dgo.Value, rq bool, p *dgo.Array) bool {
	if c := d.assign(k, t, v, rq); c {
		if v == nil {
			*p = nil
		} else {
			*p = v.(dgo.Array)
		}
		return true
	}
	return false
}

func (d *data) equals(od *data) bool {
	return eq(d.config, od.config) && eq(d.facts, od.facts) && eq(d.vars, od.vars) && eq(d.features, od.features)
}

func (d *data) get(key string) (dgo.Value, bool) {
	switch key {
	case `config`:
		return d.config, true
	case `facts`:
		return d.facts, true
	case `vars`:
		return d.vars, true
	case `features`:
		return d.features, true
	default:
		return nil, false
	}
}

func (d *data) set(key string, v dgo.Value) bool {
	switch key {
	case `config`:
		return d.mapAssign(key, dataMap, v, false, &d.config)
	case `facts`:
		return d.mapAssign(key, dataMap, v, false, &d.facts)
	case `vars`:
		return d.mapAssign(key, dataMap, v, false, &d.vars)
	case `features`:
		return d.arrayAssign(key, tf.Array(asciiPattern), v, false, &d.features)
	default:
		return false
	}
}

func (d *data) name() dgo.String {
	if n, ok := d.input.Get(nameV).(dgo.String); ok {
		return n
	}
	return nil
}

func (d *data) hash() int {
	h := 7
	if d.config != nil {
		h = h*31 + d.config.HashCode()
	}
	if d.facts != nil {
		h = h*31 + d.facts.HashCode()
	}
	if d.vars != nil {
		h = h*31 + d.vars.HashCode()
	}
	if d.features != nil {
		h = h*31 + d.features.HashCode()
	}
	return h
}

var configV = vf.String(`config`)
var factsV = vf.String(`facts`)
var featuresV = vf.String(`features`)
var varsV = vf.String(`vars`)

func (d *data) init(input dgo.Map) {
	d.input = input
	if features, ok := input.Get(featuresV).(dgo.Array); ok {
		d.features = features
	}
	if config, ok := input.Get(configV).(dgo.Map); ok {
		d.config = config
	}
	if facts, ok := input.Get(factsV).(dgo.Map); ok {
		d.facts = facts
	}
	if vars, ok := input.Get(varsV).(dgo.Map); ok {
		d.vars = vars
	}
}

func (d *data) initMap() dgo.Map {
	im := vf.MutableMap()
	if d.config != nil {
		im.Put(configV, d.config)
	}
	if d.facts != nil {
		im.Put(factsV, d.facts)
	}
	if d.vars != nil {
		im.Put(varsV, d.vars)
	}
	if d.features != nil {
		im.Put(featuresV, d.features)
	}
	return im
}

type target struct {
	data
	uri dgo.String
}

var targetType = tf.NewNamed(
	`bolt.inventory.target`,
	func(value dgo.Value) dgo.Value { return createFunc(value, &target{}) },
	initMapFunc,
	reflect.TypeOf(&target{}),
	nil,
	nil,
)

func newTarget(input dgo.Map) *target {
	t := &target{}
	t.init(input)
	return t
}

var uriV = vf.String(`uri`)

func (t *target) AppendTo(w dgo.Indenter) {
	w.AppendValue(initMapFunc(t))
}

func (t *target) asMap() dgo.Map {
	return t.initMap(false)
}

func (t *target) Equals(other interface{}) bool {
	if ot, ok := other.(*target); ok {
		return t.data.equals(&ot.data) && eq(t.uri, ot.uri)
	}
	return false
}

func (t *target) get(key string) (dgo.Value, bool) {
	switch key {
	case `uri`:
		return t.uri, true
	default:
		return t.data.get(key)
	}
}

func (t *target) HashCode() int {
	h := t.data.hash()
	if t.uri != nil {
		h = h*31 + t.uri.HashCode()
	}
	return h
}

func (t *target) init(input dgo.Map) {
	t.data.init(input)
	if uri, ok := input.Get(uriV).(dgo.String); ok {
		t.uri = uri
	} else if !input.ContainsKey(nameV) {
		panic(fmt.Errorf(`no name or uri for target: %s`, input.String()))
	}
}

func (t *target) initMap(includeName bool) dgo.Map {
	im := t.data.initMap()
	if includeName {
		im.Put(nameV, t.name())
	}
	if t.uri != nil {
		im.Put(uriV, t.uri)
	}
	return im
}

func (t *target) name() dgo.String {
	n := t.data.name()
	if n == nil {
		n = t.uri
	}
	return n
}

func (t *target) set(key string, v dgo.Value) bool {
	switch key {
	case `uri`:
		return t.stringAssign(key, asciiPattern, v, false, &t.uri)
	default:
		return t.data.set(key, v)
	}
}

func (t *target) String() string {
	return util.ToIndentedString(t)
}

func (t *target) Type() dgo.Type {
	return targetType
}

type group struct {
	data
	groups        dgo.Map
	targets       dgo.Map
	aliases       dgo.Map
	stringTargets dgo.Array
}

var groupType = tf.NewNamed(
	`bolt.inventory.group`,
	func(value dgo.Value) dgo.Value { return createFunc(value, &group{}) },
	initMapFunc,
	reflect.TypeOf(&group{}),
	nil,
	nil,
)

var groupsV = vf.String(`groups`)
var targetsV = vf.String(`targets`)

func newGroup(input dgo.Map) *group {
	g := &group{}
	g.init(input)
	return g
}

func (g *group) addAliases(targetName dgo.String, aliases dgo.Array) {
	if g.aliases == nil {
		g.aliases = vf.MutableMap()
	}
	aliases.Each(func(al dgo.Value) {
		if found := g.aliases.Get(al); found != nil {
			panic(fmt.Errorf(`alias %s refers to multiple targets: %s and %s`, al, found, targetName))
		}
		g.aliases.Put(al, targetName)
	})
}

func (g *group) addTargetDefinition(tm dgo.Map) {
	t := newTarget(tm)
	tn := t.name()
	if g.hasTarget(tn) {
		logrus.Warnf(`ignoring duplicate target in %s: %s`, g.name(), tn)
		return
	}
	if alias := tm.Get(`alias`); alias != nil {
		aliases, ok := alias.(dgo.Array)
		if !ok {
			aliases = vf.Values(alias)
		}
		if aliases.Len() > 0 {
			g.addAliases(t.name(), aliases)
		}
	}
	g.targets.Put(tn, newTarget(tm))
}

func (g *group) allAliases() dgo.Map {
	return g.collect(g.aliases, func(sg *group) dgo.Map { return sg.allAliases() })
}

func (g *group) allGroups() dgo.Map {
	return g.collect(g.groups, func(sg *group) dgo.Map { return sg.allGroups() })
}

func (g *group) allTargets() dgo.Map {
	return g.collect(g.targets, func(sg *group) dgo.Map { return sg.allTargets() })
}

func (g *group) AppendTo(w dgo.Indenter) {
	w.AppendValue(g.initMap(false))
}

func (g *group) collect(all dgo.Map, sub func(sg *group) dgo.Map) dgo.Map {
	if g.groups.Len() == 0 {
		return all
	}
	all = all.Copy(false)
	g.groups.EachValue(func(sg dgo.Value) {
		all.PutAll(sub(sg.(*group)))
	})
	return all
}

func (g *group) Equals(other interface{}) bool {
	if og, ok := other.(*group); ok {
		return g.data.equals(&og.data) && eq(g.groups, og.groups) && eq(g.targets, og.targets)
	}
	return false
}

func (g *group) get(key string) (dgo.Value, bool) {
	switch key {
	case `targets`:
		return g.targets.Values(), true
	case `groups`:
		return g.groups.Values(), true
	}
	if v := g.groups.Get(key); v != nil {
		return v, true
	}
	if v := g.targets.Get(key); v != nil {
		return v, true
	}
	return g.data.get(key)
}

func (g *group) HashCode() int {
	h := g.data.hash()
	h = h*31 + g.groups.HashCode()
	h = h*31 + g.targets.HashCode()
	return h
}

func (g *group) hasTarget(target dgo.Value) bool {
	return g.targets.ContainsKey(target)
}

func (g *group) init(input dgo.Map) {
	g.data.init(input)
	_, ok := input.Get(nameV).(dgo.String)
	if !ok {
		panic(errors.New(`group does not have a name`))
	}
	g.targets = vf.MutableMap()
	g.stringTargets = vf.MutableValues()

	ts, ok := input.Get(targetsV).(dgo.Array)
	if ok {
		ts.Each(func(e dgo.Value) {
			switch e := e.(type) {
			case dgo.String:
				g.stringTargets.Add(e)
			case dgo.Map:
				g.addTargetDefinition(e)
			default:
				panic(fmt.Errorf(`a Target entry must be a String or Hash, not %s`, e.Type()))
			}
		})
	}
	if g.aliases == nil {
		g.aliases = vf.Map()
	}

	gs, ok := input.Get(groupsV).(dgo.Array)
	if ok {
		g.groups = vf.MutableMap()
		gs.Each(func(gm dgo.Value) {
			gr := newGroup(gm.(dgo.Map))
			g.groups.Put(gr.name(), gr)
		})
		g.groups.Freeze()
	} else {
		g.groups = vf.Map()
	}
}

func (g *group) asMap() dgo.Map {
	m := g.data.initMap()
	m.PutAll(g.groups.Map(func(e dgo.MapEntry) interface{} { return e.Value().(groupOrTarget).asMap() }))
	if g.targets.Len() > 0 {
		m.Put(targetsV, g.targets.Values().Map(func(t dgo.Value) interface{} {
			tg := t.(*target)
			tn := tg.name()
			tm := tg.asMap()
			if tm.Len() == 1 {
				if uri := tm.Get(uriV); uri != nil && tn.Equals(uri) {
					return uri
				}
			}
			tm.Put(nameV, tn)
			return tm
		}))
	}
	return m
}

func (g *group) initMap(includeName bool) dgo.Map {
	im := g.data.initMap()
	if includeName {
		im.Put(nameV, g.name())
	}
	if g.groups.Len() > 0 {
		im.Put(groupsV, g.groups.Values().Map(func(g dgo.Value) interface{} { return g.(*group).initMap(true) }))
	}
	if g.targets.Len() > 0 {
		im.Put(targetsV, g.targets.Values().Map(func(t dgo.Value) interface{} {
			tg := t.(*target)
			tn := tg.name()
			tm := tg.initMap(false)
			if tm.Len() == 1 {
				if uri := tm.Get(uriV); uri != nil && tn.Equals(uri) {
					return uri
				}
			}
			tm.Put(nameV, tn)
			return tm
		}))
	}
	return im
}

func (g *group) remove(key string) bool {
	// removal of group or target
	if sg := g.groups.Remove(key); sg != nil {
		// group removal
		ig := g.data.input.Get(groupsV).(dgo.Array)
		if i := indexOfName(key, ig); i >= 0 {
			ig.Remove(i)
		}
		return true
	}
	if t := g.targets.Remove(key); t != nil {
		// target removal
		ig := g.data.input.Get(targetsV).(dgo.Array)
		if i := indexOfName(key, ig); i >= 0 {
			ig.Remove(i)
		}
		return true
	}
	return false
}

func (g *group) resolveStringTargets(allAliases dgo.Map, allTargets dgo.Map) {
	g.stringTargets.Each(func(st dgo.Value) { g.resolveStringTarget(st.(dgo.String), allAliases, allTargets) })
	g.groups.EachValue(func(sg dgo.Value) { sg.(*group).resolveStringTargets(allAliases, allTargets) })
}

func (g *group) resolveStringTarget(stringTarget dgo.String, allAliases dgo.Map, allTargets dgo.Map) {
	switch {
	case allTargets.ContainsKey(stringTarget):
		g.targets.Put(stringTarget, newTarget(vf.Map(nameV, stringTarget)))
	case allAliases.ContainsKey(stringTarget):
		canonicalName := allAliases.Get(stringTarget)
		if g.hasTarget(canonicalName) {
			logrus.Warnf(`ignoring duplicate target in %s: %s`, g.name(), canonicalName)
		} else {
			g.targets.Put(stringTarget, newTarget(vf.Map(nameV, canonicalName)))
		}
	case g.hasTarget(stringTarget):
		logrus.Warnf(`ignoring duplicate target in %s: %s`, g.name(), stringTarget)
	default:
		g.targets.Put(stringTarget, newTarget(vf.Map(uriV, stringTarget)))
	}
}

func (g *group) set(key string, v dgo.Value) bool {
	if _, ok := g.data.get(key); ok {
		return g.data.set(key, v)
	}

	if v == nil {
		return g.remove(key)
	}

	upsert := func(mapName dgo.String, m dgo.Map) bool {
		if og := m.Put(key, v); v.Equals(og) {
			return false
		}
		gs, ok := g.data.input.Get(mapName).(dgo.Array)
		if ok {
			gs.Add(v)
		} else {
			g.data.input.Put(mapName, vf.MutableValues(v))
		}
		return true
	}

	switch v.(type) {
	case *group:
		return upsert(groupsV, g.groups)
	case *target:
		return upsert(targetsV, g.targets)
	default:
		return false
	}
}

func (g *group) String() string {
	return util.ToIndentedString(g)
}

func (g *group) Type() dgo.Type {
	return groupType
}

// IndexOfName returns the index of the first entry in the array that either equals the given name or
// is a map with a "name" entry whose value equals the given name. If no entry is found, the function
// returns -1
func indexOfName(n string, a dgo.Array) int {
	i := int64(-1)
	key := vf.String(n)
	ix := a.Find(func(sg dgo.Value) interface{} {
		i++
		switch sg := sg.(type) {
		case dgo.Map:
			if sg.Get(nameV).Equals(key) {
				return vf.Integer(i)
			}
		case dgo.String:
			if sg.Equals(key) {
				return vf.Integer(i)
			}
		}
		return nil
	})
	if ix == nil {
		return -1
	}
	return int(ix.(dgo.Integer).GoInt())
}

type storage struct {
	path     string
	age      time.Time
	contents *group  // the "all" group
	groups   dgo.Map // all groups
}

// NewStorage creates a new storage for the bolt inventory version 2 file at the given path
func NewStorage(path string) iapi.Storage {
	return &storage{path: path}
}

func (s *storage) Delete(key string) bool {
	panic("implement me")
}

func (s *storage) Get(key string) dgo.Value {
	s.refreshContents()
	value := s.dig(strings.Split(key, `.`), 0, s.contents)
	if got, ok := value.(groupOrTarget); ok {
		// Convert to map
		value = got.asMap()
	}
	return value
}

func (s *storage) dig(parts []string, depth int, c dgo.Value) dgo.Value {
	n := len(parts) - depth
	if n == 0 {
		return c
	}
	v := getAtKey(c, parts, depth)
	switch v.(type) {
	case dgo.Map, dgo.Array, groupOrTarget:
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
	switch c := c.(type) {
	case dgo.Array:
		if i, err := strconv.Atoi(k); err == nil {
			if i >= 0 && i < c.Len() {
				return c.Get(i)
			}
		}
	case dgo.Map:
		return c.Get(k)
	case groupOrTarget:
		if v, ok := c.get(k); ok {
			if v == nil {
				v = vf.Nil // entry exists but is not set
			}
			return v
		}
	}
	return nil
}

func (s *storage) Query(key string, q dgo.Map) query.Result {
	v := s.Get(key)
	if a, ok := v.(dgo.Array); ok {
		match := q.Get(`match`) // required parameter
		rx := regexp.MustCompile(regexp.QuoteMeta(match.(dgo.String).GoString()))
		qr := query.NewResult(false)
		a.EachWithIndex(func(e dgo.Value, idx int) {
			if got, ok := e.(groupOrTarget); ok && rx.FindString(got.name().GoString()) != `` {
				qr.Add(vf.Integer(int64(idx)), e)
			}
		})
		return qr
	}
	return nil
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
func (s *storage) refreshContents() {
	now := time.Now()
	if s.contents == nil {
		s.readInventory()
		s.age = now
		return
	}

	if now.Sub(s.age) < minRefresh {
		return
	}

	cs, err := os.Stat(s.path)
	if err != nil {
		panic(err)
	}

	if cs.ModTime().After(s.age) {
		now = time.Now()
		s.readInventory()
	}
	s.age = now
}

func (s *storage) readInventory() {
	data := yaml.Read(s.path)
	if !inventoryFileType.Instance(data) {
		panic(tf.IllegalAssignment(inventoryFileType, data))
	}
	all := newGroup(data.With(nameV, `all`))
	all.resolveStringTargets(all.allAliases(), all.allTargets())
	s.groups = all.allGroups()
	s.contents = all
}

func (s *storage) Set(key string, model dgo.Map) (changes dgo.Map, err error) {
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

	s.refreshContents()
	changes = vf.MutableMap()
	value := s.dig(strings.Split(key, `.`), 0, s.contents)
	if got, ok := value.(groupOrTarget); ok {
		model.EachEntry(func(e dgo.MapEntry) {
			if got.set(e.Key().(dgo.String).GoString(), e.Value()) {
				changes.Put(e.Key(), e.Value())
			}
		})
	}
	return
}
