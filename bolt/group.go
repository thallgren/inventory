package bolt

import (
	"fmt"
	"net/url"
	"reflect"

	"github.com/lyraproj/dgo/typ"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/vf"
	"github.com/sirupsen/logrus"
)

// A Group interface is implemented by the group
type Group interface {
	Data

	// CollectTargets collects all Target instancesinto a map where the key is a name or an
	// alias and the value is an Array of all Target declarations found by that key.
	CollectTargets(dgo.Map)

	// LocalGroups returns an Array of Group instances that is parented by this Group
	LocalGroups() dgo.Array

	// LocalTargets returns an Array of Target instances that is parented by this Group
	LocalTargets() dgo.Array

	// ResolveStringTargets resolves all StringTargets found in this group and all Groups
	// beneath it. New resolved target instances are added to the given Map.
	ResolveStringTargets(allTargets dgo.Map)
}

type group struct {
	dta
}

var groupType = tf.NewNamed(
	`bolt.inventory.group`,
	nil,
	nil,
	reflect.TypeOf(&group{}),
	reflect.TypeOf((*Group)(nil)).Elem(),
	nil,
)

var groupsV = vf.String(`groups`)
var targetsV = vf.String(`targets`)
var stringTargetsV = vf.String(`string_targets`)

// NewGroup creates a new Group based on the given input
func NewGroup(parent Group, input dgo.Map) Group {
	return &group{dta: dta{input: input, parent: parent}}
}

func (g *group) targetsOfType(t dgo.Type) dgo.Array {
	if targets, ok := g.input.Get(targetsV).(dgo.Array); ok {
		return targets.Select(func(ti dgo.Value) bool { return t.Instance(ti) })
	}
	return vf.Values()
}

func (g *group) LocalGroups() dgo.Array {
	if groups, ok := g.input.Get(groupsV).(dgo.Array); ok {
		return groups.Map(func(ti dgo.Value) interface{} { return NewGroup(g, ti.(dgo.Map)) })
	}
	return vf.Values()
}

func (g *group) LocalTargets() dgo.Array {
	return g.targetsOfType(typ.Map).Map(func(ti dgo.Value) interface{} { return NewTarget(g, ti.(dgo.Map)) })
}

func (g *group) CollectTargets(all dgo.Map) {
	g.LocalTargets().Each(func(tv dgo.Value) { tv.(*trg).registerTarget(all) })
	g.LocalGroups().Each(func(gv dgo.Value) { gv.(Group).CollectTargets(all) })
}

func (g *group) ResolveStringTargets(allTargets dgo.Map) {
	g.targetsOfType(typ.String).Each(func(st dgo.Value) { g.resolveStringTarget(st.(dgo.String), allTargets) })
	g.LocalGroups().Each(func(sg dgo.Value) { sg.(Group).ResolveStringTargets(allTargets) })
}

func (g *group) resolveStringTarget(stringTarget dgo.String, allTargets dgo.Map) {
	tgs, ok := allTargets.Get(stringTarget).(dgo.Array)
	if ok {
		if tgs.Any(func(t dgo.Value) bool { return t.(Data).HasParent(g) }) {
			logrus.Warnf(`ignoring duplicate target in %s: %s`, g.Name(), stringTarget)
		} else {
			tgs.Add(NewTarget(g, vf.Map(nameV, stringTarget)))
		}
		return
	}

	var t Target
	if namePattern.Instance(stringTarget) {
		panic(fmt.Errorf(`reference to non existent target '%s' in group '%s'`, stringTarget, g.Name()))
	}
	if _, err := url.Parse(stringTarget.GoString()); err != nil {
		panic(fmt.Errorf(`the string '%s' is not a valid URI: %s`, stringTarget, err.Error()))
	}
	t = NewTarget(g, vf.Map(uriV, stringTarget))
	allTargets.Put(t.Name(), vf.MutableValues(t))
}

func (g *group) Type() dgo.Type {
	return groupType
}
