package query

import "github.com/lyraproj/dgo/dgo"

// Param describes a query parameter that can be used when querying a specific resource in a Storage
type Param interface {
	// Name is the name of the parameter
	Name() string

	// Type is the type of the parameter value
	Type() dgo.Type

	// Required indicates that a query is unacceptable unless it includes a value for this parameter
	Required() bool
}

type param struct {
	n string
	t dgo.Type
	r bool
}

// NewParam creates a new query parameter
func NewParam(name string, typ dgo.Type, required bool) Param {
	return &param{n: name, t: typ, r: required}
}

func (q *param) Name() string {
	return q.n
}

func (q *param) Type() dgo.Type {
	return q.t
}

func (q *param) Required() bool {
	return q.r
}
