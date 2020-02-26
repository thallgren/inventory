package inventory

import (
	"strconv"

	"github.com/puppetlabs/inventory/iapi"

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
		switch value := value.(type) {
		case iapi.Resource:
			s[index] = res.Ref(value.RID(ServiceName))
		case dgo.Map, dgo.Array:
			s[index] = res.Ref(path + strconv.Itoa(index))
		default:
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
		ks := value.Key().(dgo.String).GoString()
		var is interface{}
		switch v := value.Value().(type) {
		case iapi.Resource:
			is = res.Ref(v.RID(ServiceName))
		case dgo.Map, dgo.Array:
			is = res.Ref(path + ks)
		default:
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
		switch value := dc.Value().(type) {
		case iapi.Resource:
			s[index] = res.Ref(value.RID(ServiceName))
		case dgo.Map, dgo.Array:
			s[index] = res.Ref(path + ref.String())
		default:
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
		var is interface{}
		switch value := dc.Value().(type) {
		case iapi.Resource:
			is = res.Ref(value.RID(ServiceName))
		case dgo.Map, dgo.Array:
			is = res.Ref(path + rs)
		default:
			vf.FromValue(value, &is)
		}
		ms[rs] = is
	})
	return ms
}
