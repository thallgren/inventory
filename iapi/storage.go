package iapi

import (
	"fmt"

	"github.com/puppetlabs/inventory/query"

	"github.com/lyraproj/dgo/dgo"
)

// NotFound is an error implementation used by Storage to provide information about a required
// key not being present in the storage.
type NotFound string

func (n NotFound) Error() string {
	return fmt.Sprintf(`key %q not found`, string(n))
}

// A Storage is some kind of database capable of storing a hierarchy of arbitrary depth. An item
// is associated with a dot delimited key. Elements in arrays are access using numeric segments in
// such keys.
type Storage interface {
	// Delete will make an attempt to delete the value with the given key from the storage. It
	// returns true on success and false when no such key was found.
	Delete(key string) bool

	// Get finds a value using a dot separated key. It returns the value or nil if no value
	// is found.
	Get(key string) dgo.Value

	// Query finds a value using the dot separate key and a map of query values It returns the value or nil if no value
	// is found.
	Query(key string, query dgo.Map) query.Result

	// QueryKeys returns the set of keys that can be used to query this storage at the given
	// key in a predictable order.
	QueryKeys(key string) []query.Param

	// Set stores the given model under the given key and returns a map of the entries that
	// were changed. The returned map will be equal to or a subset of the model.
	//
	// An attempt to store a model using a non existent key will result in a NotFound
	// error.
	Set(key string, model dgo.Map) (dgo.Map, error)
}
