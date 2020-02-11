package bolt

import (
	"github.com/lyraproj/dgo/dgo"
)

// DeepMerge will create a new map that contains all keys from both a and b. The value of b takes
// precedence for identical keys unless both values are maps in which case this function is called
// recursively.
func DeepMerge(a, b dgo.Map) dgo.Map {
	if b.Len() == 0 {
		return a
	}
	if a.Len() == 0 {
		return b
	}
	a = a.Copy(false)
	b.EachEntry(func(e dgo.MapEntry) {
		v := e.Value()
		if hb, ok := v.(dgo.Map); ok {
			if ha, ok := a.Get(e.Key).(dgo.Map); ok {
				a.Put(e.Key(), DeepMerge(ha, hb))
				return
			}
		}
		a.Put(e.Key(), v)
	})
	return a
}
