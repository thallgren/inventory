// Package query contains the query parameter and result
package query

import (
	"errors"

	"github.com/lyraproj/dgo/vf"

	"github.com/lyraproj/dgo/dgo"
)

// Result contains the result of a query
type Result interface {
	// Add adds a new tuple to the result. An attempt to add to a singleton result will cause a panic.
	Add(ref, value dgo.Value)

	// EachWithRefAndIndex calls the given function with each value in this result together with its
	// associated reference and index
	EachWithRefAndIndex(func(value, ref dgo.Value, index int))

	// IsMap returns true if the result represents a map (i.e. references are strings. If this method
	// returns false, the references are integers
	IsMap() bool

	// Len returns the number of entries in this query result
	Len() int

	// Ref returns the reference to the value at the given index.
	Ref(int) dgo.Value

	// Singleton when this method returns true, the result, which is guaranteed to be of size one, represents a single
	// value and not a one element array.
	Singleton() bool

	// Value is the value at the given index.
	Value(int) dgo.Value
}

type result struct {
	values []dgo.Value
	refs   []dgo.Value
	single bool
	isMap  bool
}

// NewResult creates a new query result that contains a collection of values
func NewResult(isMap bool) Result {
	return &result{isMap: isMap}
}

// NewSingleResult creates a new query result that contains one single value
// with an empty reference
func NewSingleResult(value dgo.Value) Result {
	return &result{refs: []dgo.Value{vf.Nil}, values: []dgo.Value{value}, single: true}
}

func (r *result) Add(ref, value dgo.Value) {
	if r.single {
		panic(errors.New(`attempt to add to single query.Result'`))
	}
	r.refs = append(r.refs, ref)
	r.values = append(r.values, value)
}

func (r *result) EachWithRefAndIndex(f func(value, ref dgo.Value, index int)) {
	for idx, ref := range r.refs {
		f(r.values[idx], ref, idx)
	}
}

func (r *result) IsMap() bool {
	return r.isMap
}

func (r *result) Singleton() bool {
	return r.single
}

func (r *result) Len() int {
	return len(r.refs)
}

func (r *result) Ref(idx int) dgo.Value {
	return r.refs[idx]
}

func (r *result) Value(idx int) dgo.Value {
	return r.values[idx]
}
