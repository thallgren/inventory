package inventory

import (
	"strconv"

	"github.com/puppetlabs/inventory/change"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/query"
)

// Convert an Array into a Resgate collection. All elements that are Arrays and Maps are
// converted into resource references based on the given path and their index.
func arrayToCollection(a dgo.Array, path string) []interface{} {
	s := make([]interface{}, a.Len())
	a.EachWithIndex(func(value dgo.Value, index int) {
		if change.IsComplex(value) {
			s[index] = res.Ref(path + strconv.Itoa(index))
		} else {
			vf.FromValue(value, &s[index])
		}
	})
	return s
}

// Convert a Map into a Resgate model. All values that are Arrays and Maps are
// converted into resource references based on the given path and their key in the map.
func mapToModel(m dgo.Map, path string) map[string]interface{} {
	ms := make(map[string]interface{}, m.Len())
	m.EachEntry(func(value dgo.MapEntry) {
		v := value.Value()
		ks := value.Key().(dgo.String).GoString()
		var is interface{}
		if change.IsComplex(v) {
			is = res.Ref(path + ks)
		} else {
			vf.FromValue(v, &is)
		}
		ms[ks] = is
	})
	return ms
}

// Convert an query result in array form into a Resgate collection. All elements that are Arrays and Maps are
// converted into resource references based on the given path and their index.
func queryToCollection(a query.Result, path string) []interface{} {
	st := streamer.New(nil, streamer.DefaultOptions())
	s := make([]interface{}, a.Len())
	a.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		dc := streamer.DataCollector()
		st.Stream(value, dc)
		value = dc.Value()
		if change.IsComplex(value) {
			s[index] = res.Ref(path + ref.String())
		} else {
			vf.FromValue(value, &s[index])
		}
	})
	return s
}

// Convert an query result in map form into a Resgate model. All values that are Arrays and Maps are
// converted into resource references based on the given path and their key in the map.
func queryToModel(a query.Result, path string) map[string]interface{} {
	st := streamer.New(nil, streamer.DefaultOptions())
	ms := make(map[string]interface{}, a.Len())
	a.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		rs := ref.(dgo.String).GoString()
		dc := streamer.DataCollector()
		st.Stream(value, dc)
		value = dc.Value()
		var is interface{}
		if change.IsComplex(value) {
			is = res.Ref(path + rs)
		} else {
			vf.FromValue(value, &is)
		}
		ms[rs] = is
	})
	return ms
}
