package bolt

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/tf"
	"github.com/lyraproj/dgo/vf"
	"github.com/sirupsen/logrus"
)

// A Group interface is implemented by the group
type Group interface {
	Data

	// CollectTargets collects all Target aliases into a map where the key is a alias and the value
	// is the name of the target declaring that alias
	CollectAliases(dgo.Map)

	// CollectTargets collects all Target instances into a map where the key is a name and the value is
	// an Array of all Target declarations found by that key.
	CollectTargets(dgo.Map)

	// LocalGroups returns an Array of Group instances that is parented by this Group
	LocalGroups() dgo.Array

	// LocalTargets returns an Array of Target instances that is parented by this Group
	LocalTargets() dgo.Array

	// ResolveStringTargets resolves all StringTargets found in this group and all Groups
	// beneath it. New resolved target instances are added to the allTargets Map.
	ResolveStringTargets(allAlias, allTargets dgo.Map)

	// Find all groups with a name that matches the given regexp
	FindGroups(rx *regexp.Regexp) []Group
}

type group struct {
	dta
	groups        dgo.Array
	targets       dgo.Array
	stringTargets dgo.Array
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

// NewGroup creates a new Group based on the given input
func NewGroup(parent Group, input dgo.Map) Group {
	g := &group{dta: dta{input: input, parent: parent}}
	if targets, ok := g.input.Get(targetsV).(dgo.Array); ok {
		targets.Each(func(st dgo.Value) {
			if _, ok := st.(dgo.String); ok {
				if g.stringTargets == nil {
					g.stringTargets = vf.MutableValues()
				}
				g.stringTargets.Add(st)
			} else {
				if g.targets == nil {
					g.targets = vf.MutableValues()
				}
				g.targets.Add(NewTarget(g, st.(dgo.Map)))
			}
		})
	}
	if g.stringTargets == nil {
		g.stringTargets = vf.Values()
	}
	if g.targets == nil {
		g.targets = vf.Values()
	}

	if groups, ok := g.input.Get(groupsV).(dgo.Array); ok {
		g.groups = groups.Map(func(sg dgo.Value) interface{} { return NewGroup(g, sg.(dgo.Map)) })
	} else {
		g.groups = vf.Values()
	}
	return g
}

func (g *group) FindGroups(rx *regexp.Regexp) []Group {
	return g.matchGroups(rx, nil)
}

func (g *group) matchGroups(rx *regexp.Regexp, groups []Group) []Group {
	if rx.FindString(g.Name().String()) != `` {
		groups = append(groups, g)
	}
	g.LocalGroups().Each(func(gv dgo.Value) { groups = gv.(*group).matchGroups(rx, groups) })
	return groups
}

func (g *group) LocalGroups() dgo.Array {
	return g.groups
}

func (g *group) LocalTargets() dgo.Array {
	return g.targets
}

func (g *group) CollectTargets(all dgo.Map) {
	g.LocalTargets().Each(func(tv dgo.Value) { tv.(*trg).registerTarget(all) })
	g.LocalGroups().Each(func(gv dgo.Value) { gv.(Group).CollectTargets(all) })
}

func (g *group) CollectAliases(all dgo.Map) {
	g.LocalTargets().Each(func(tv dgo.Value) { tv.(*trg).registerAlias(all) })
	g.LocalGroups().Each(func(gv dgo.Value) { gv.(Group).CollectAliases(all) })
}

func (g *group) ResolveStringTargets(allAlias, allTargets dgo.Map) {
	g.stringTargets.Each(func(st dgo.Value) { g.resolveStringTarget(st.(dgo.String), allAlias, allTargets) })
	g.LocalGroups().Each(func(sg dgo.Value) { sg.(Group).ResolveStringTargets(allAlias, allTargets) })
}

func (g *group) resolveStringTarget(stringTarget dgo.String, allAlias, allTargets dgo.Map) {
	if alias, ok := allAlias.Get(stringTarget).(dgo.String); ok {
		stringTarget = alias
	}
	tgs, ok := allTargets.Get(stringTarget).(dgo.Array)
	if ok {
		if tgs.Any(func(t dgo.Value) bool { return t.(Data).HasParent(g) }) {
			logrus.Warnf(`ignoring duplicate target in %s: %s`, g.Name(), stringTarget)
		} else {
			tgs.Add(g.targetFromString(stringTarget))
		}
	} else {
		t := g.targetFromString(stringTarget)
		if t.URI() != nil {
			allTargets.Put(stringTarget, vf.MutableValues(t))
		} else {
			logrus.Warnf(`ignoring reference to non existing target in %s: %s`, g.Name(), stringTarget)
		}
	}
}

func (g *group) targetFromString(s dgo.String) Target {
	if namePattern.Instance(s) {
		return NewTarget(g, vf.Map(nameV, s))
	}
	if _, err := url.Parse(s.GoString()); err != nil {
		panic(fmt.Errorf(`the string '%s' is not a valid URI: %s`, s, err.Error()))
	}
	return NewTarget(g, vf.Map(uriV, s))
}

func (g *group) Type() dgo.Type {
	return groupType
}
