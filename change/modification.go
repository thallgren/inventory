package change

import (
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/vf"
)

// ModType denotes the type of modification that was made to some item in the storage
type ModType int

const (
	// Unchanged means that the value did not change
	Unchanged = ModType(iota)

	// Add means that a value was added to an array at some index
	Add

	// Change means that the properties of an existing resource was updated
	Change

	// Create means that a new resource was added to the storage
	Create

	// Delete means that an existing resource was removed from storage
	Delete

	// Remove means that a value was removed from array at some index
	Remove

	// Reset means that the properties of an existing resource was updated
	Reset

	// Set means that a value was replaced at some index
	Set
)

var Deleted = vf.Value(struct{ deleted bool }{true})

// A Modification contains information a named thing that changed in a container, how it was changed
// and what the new value was.
type Modification struct {
	ResourceName string
	Index        int
	Value        dgo.Value
	Type         ModType
}
