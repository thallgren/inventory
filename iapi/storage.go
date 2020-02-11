package iapi

import (
	"fmt"

	"github.com/lyraproj/dgo/vf"

	"github.com/puppetlabs/inventory/query"

	"github.com/lyraproj/dgo/dgo"
)

// NotFound is an error implementation used by Storage to provide information about a required
// key not being present in the storage.
type NotFound string

func (n NotFound) Error() string {
	return fmt.Sprintf(`key %q not found`, string(n))
}

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

// A Storage is some kind of database capable of storing a hierarchy of arbitrary depth. An item
// is associated with a dot delimited key. Elements in arrays are access using numeric segments in
// such keys.
type Storage interface {
	// Delete will make an attempt to delete the value with the given key from the storage. It
	// a slice of modifications and true on success and false when no such key was found.
	Delete(key string) ([]*Modification, bool)

	// Get finds a value using a dot separated key. It returns a slice of modifications that has been
	// made since the storage was last accessed together with the value or nil if no value is found.
	Get(key string) ([]*Modification, dgo.Value)

	// Query finds a value using the dot separate key and a map of query values It returns a slice of
	// modifications that has been made since the storage was last accessed together the result of
	// the query.
	Query(key string, query dgo.Map) ([]*Modification, query.Result)

	// QueryKeys returns the set of keys that can be used to query this storage at the given
	// key in a predictable order.
	QueryKeys(key string) []query.Param

	// Set stores the given model under the given key and returns a map of the entries that
	// were changed together with a slice of modifications that indicates this change and all
	// other changes made since the storage was last accessed.
	//
	// An attempt to store a model using a non existent key will result in a NotFound
	// error.
	Set(key string, model dgo.Map) ([]*Modification, error)
}
