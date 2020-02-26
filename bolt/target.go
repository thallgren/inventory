package bolt

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"

	"github.com/puppetlabs/inventory/change"

	"github.com/puppetlabs/inventory/iapi"

	"github.com/sirupsen/logrus"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/vf"
)

// A Target in the Bolt Inventory
type Target interface {
	Data
	iapi.Resource

	// Aliases returns an Array of strings denoting the aliases that can be used
	// to address this target. An empty Array is return if the target has no aliases.
	Aliases() dgo.Array

	// Config returns a deep merge of the config that this target and its parent groups
	// have declared. Mappings found in a child take precedence over mappings in parent.
	Config() dgo.Map

	// Facts returns a deep merge of the facts that this target and its parent groups
	// have declared. Mappings found in a child take precedence over mappings in parent.
	Facts() dgo.Map

	// Config returns a unique and sorted array of features that this target and its
	// parent groups have declared.
	Features() dgo.Array

	// Facts returns a shallow merge of the vars that this target and its parent groups
	// have declared. Mappings found in a child take precedence over mappings in parent.
	Vars() dgo.Map

	// HasName returns true if this target's name or uri matches the given name or if
	// it has an alias that matches the given name.
	HasName(name dgo.String) bool

	// URI returns the URI of this target
	URI() dgo.String
}

// data contains the properties that are common to both Group and Target
type trg struct {
	dta
}

var targetType = tf.NewNamed(
	`bolt.inventory.target`,
	nil,
	nil,
	reflect.TypeOf(&trg{}),
	reflect.TypeOf((*Target)(nil)).Elem(),
	nil,
)

var aliasV = vf.String(`alias`)
var uriV = vf.String(`uri`)

func (t *trg) Aliases() dgo.Array {
	switch alias := t.input.Get(aliasV).(type) {
	case dgo.Array:
		return alias
	case dgo.String:
		return vf.Values(alias)
	default:
		return vf.Values()
	}
}

// NewTarget creates a NewTarget for the given Group that is based on the given input.
func NewTarget(parent Group, input dgo.Map) Target {
	return &trg{dta: dta{parent: parent, input: input}}
}

func (t *trg) Config() dgo.Map {
	merged := vf.Map()
	for _, p := range t.AllParents() {
		merged = DeepMerge(merged, p.LocalConfig())
	}
	return DeepMerge(merged, t.LocalConfig())
}

func (t *trg) Equals(other interface{}) bool {
	if ot, ok := other.(*trg); ok {
		return t.input.Equals(ot.input)
	}
	return false
}

func (t *trg) HasName(name dgo.String) bool {
	return name.Equals(t.Name()) || name.Equals(t.URI()) || t.Aliases().IndexOf(name) >= 0
}

func (t *trg) Facts() dgo.Map {
	merged := vf.Map()
	for _, p := range t.AllParents() {
		merged = DeepMerge(merged, p.LocalFacts())
	}
	return DeepMerge(merged, t.LocalFacts())
}

func (t *trg) Features() dgo.Array {
	merged := vf.MutableValues()
	for _, p := range t.AllParents() {
		merged.AddAll(p.LocalFeatures())
	}
	merged.AddAll(t.LocalFeatures())
	return merged.Unique().Sort()
}

func (t *trg) registerAlias(all dgo.Map) {
	n := t.Name()
	if n == nil {
		n = t.URI()
	}
	t.Aliases().Each(func(a dgo.Value) { all.Put(a, n) })
}

func (t *trg) registerTarget(all dgo.Map) {
	n := t.Name()
	if n == nil {
		n = t.URI()
	}
	if a, ok := all.Get(n).(dgo.Array); ok {
		a.Add(t)
	} else {
		all.Put(n, vf.MutableValues(t))
	}
}

func (t *trg) DataMap() dgo.Map {
	return t.input
}

func (t *trg) ID() string {
	return makeID(t.input.Get(realmV), t.input.Get(nameV), t.input.Get(idV))
}

func (t *trg) RID(serviceName string) string {
	return serviceName + `.target.` + makeID(t.input.Get(realmV), t.input.Get(nameV), t.input.Get(idV))
}

func (t *trg) Type() dgo.Type {
	return targetType
}

func (t *trg) UpdateFrom(other change.Identifiable, mods []*change.Modification) []*change.Modification {
	return change.Map(`target.`+t.ID(), t.DataMap(), other.(*trg).DataMap(), mods)
}

func (t *trg) URI() dgo.String {
	if uri, ok := t.input.Get(uriV).(dgo.String); ok {
		return uri
	}
	return nil
}

func (t *trg) Vars() dgo.Map {
	merged := vf.MutableMap()
	for _, p := range t.AllParents() {
		merged.PutAll(p.LocalVars())
	}
	merged.PutAll(t.LocalVars())
	return merged
}

func makeID(rn, name, uri dgo.Value) string {
	if name == nil {
		if uri == nil {
			panic(fmt.Errorf(`target in realm '%s' has no name and no uri`, rn))
		}
		name = uri
	}
	b := bytes.NewBufferString(rn.String())
	_ = b.WriteByte('.')
	_, _ = b.WriteString(name.String())
	return base64.URLEncoding.EncodeToString(b.Bytes())
}

// mergeTargets creates a Map that contains the merged data from all given targets.
func (r *realm) mergeTargets(targets dgo.Array) Target {
	config := vf.Map()
	facts := vf.Map()
	features := vf.MutableValues()
	vars := vf.MutableMap()
	var name dgo.String
	var uri dgo.String
	targets.Each(func(tv dgo.Value) {
		t := tv.(Target)
		config = DeepMerge(config, t.Config())
		facts = DeepMerge(facts, t.Facts())
		features.AddAll(t.Features())
		vars.PutAll(t.Vars())
		if t.Name() != nil {
			if name == nil {
				name = t.Name()
			} else if !name.Equals(t.Name()) {
				logrus.Warnf(`target is using conflicting name's: %s != %s'`, name, t.Name())
			}
		}
		if t.URI() != nil {
			if uri == nil {
				uri = t.URI()
			} else if !uri.Equals(t.URI()) {
				logrus.Warnf(`target %s is using conflicting URI's: %s != %s'`, name, uri, t.URI())
			}
		}
	})
	rn := r.contents.Name()
	m := vf.MutableMap()
	m.Put(idV, makeID(rn, name, uri))
	m.Put(realmV, rn)
	if name != nil {
		m.Put(nameV, name)
	} else {
		m.Put(uriV, uri)
	}

	if uri != nil {
		m.Put(uriV, uri)
	}
	if config.Len() > 0 {
		m.Put(configV, config)
	}
	if facts.Len() > 0 {
		m.Put(factsV, facts)
	}
	if features.Len() > 0 {
		m.Put(featuresV, features)
	}
	if vars.Len() > 0 {
		m.Put(varsV, vars)
	}
	return NewTarget(nil, m)
}
