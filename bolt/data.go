package bolt

import (
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/vf"
)

var configV = vf.String(`config`)
var factsV = vf.String(`facts`)
var featuresV = vf.String(`features`)
var idV = vf.String(`id`)
var nameV = vf.String(`name`)
var varsV = vf.String(`vars`)

// A Data interface is implemented by group and target
type Data interface {
	dgo.Value

	// AllParents returns the all the parents from top-level down to this instance
	AllParents() []Group

	// HasParent returns true if any of its parents is the given Group
	HasParent(Group) bool

	// LocalConfig returns the config from the input map held by this instance or
	// an empty Map if no such config exists
	LocalConfig() dgo.Map

	// LocalFacts returns facts from the input map held by this instance or
	// an empty Map if no such facts exists
	LocalFacts() dgo.Map

	// LocalFeatures returns features from the input map held by this instance or
	// an empty Array if no such features exists
	LocalFeatures() dgo.Array

	// LocalVars returns vars from the input map held by this instance or
	// an empty Map if no vars facts exists
	LocalVars() dgo.Map

	// Name returns the name of this instance
	Name() dgo.String
}

type dta struct {
	input  dgo.Map
	parent Group
}

func (d *dta) AllParents() []Group {
	if d.parent == nil {
		return []Group{}
	}
	return append(d.parent.AllParents(), d.parent)
}

func (d *dta) HasParent(p Group) bool {
	if d.parent == nil {
		return false
	}
	return d.parent == p || d.parent.HasParent(p)
}

func (d *dta) Equals(other interface{}) bool {
	if od, ok := other.(*dta); ok {
		return d.input.Equals(od.input)
	}
	return false
}

func (d *dta) HashCode() int {
	return d.input.HashCode()
}

// LocalFeatures returns an immutable array of the features contained in this data
func (d *dta) LocalFeatures() dgo.Array {
	if fa, ok := d.input.Get(featuresV).(dgo.Array); ok {
		return fa
	}
	return vf.Values()
}

// LocalConfig returns an immutable map of the config contained in this data
func (d *dta) LocalConfig() dgo.Map {
	if cm, ok := d.input.Get(configV).(dgo.Map); ok {
		return cm
	}
	return vf.Map()
}

// LocalFacts returns an immutable map of the facts contained in this data
func (d *dta) LocalFacts() dgo.Map {
	if cm, ok := d.input.Get(factsV).(dgo.Map); ok {
		return cm
	}
	return vf.Map()
}

// LocalVars returns an immutable map of the vars contained in this data
func (d *dta) LocalVars() dgo.Map {
	if cm, ok := d.input.Get(varsV).(dgo.Map); ok {
		return cm
	}
	return vf.Map()
}

func (d *dta) Name() dgo.String {
	if n, ok := d.input.Get(nameV).(dgo.String); ok {
		return n
	}
	return nil
}

func (d *dta) String() string {
	return d.Name().GoString()
}

func (d *dta) Type() dgo.Type {
	panic("implement me")
}
