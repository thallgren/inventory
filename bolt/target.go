package bolt

import (
	"reflect"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/vf"
)

// A Target in the Bolt Inventory
type Target interface {
	Data

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

func (t *trg) Type() dgo.Type {
	return targetType
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
