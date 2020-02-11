// Package change contains functions needed to generate Modifications when changing Maps and Arrays
package change

import (
	"strconv"
	"strings"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/vf"
)

// IsComplex returns true if the given value is a Map or an Array
func IsComplex(v dgo.Value) bool {
	switch v.(type) {
	case dgo.Map, dgo.Array:
		return true
	default:
		return false
	}
}

// Map will make map a equal to map b and append all modifications needed in order to do that
// in the given mods slice. The new slice is returned. The string p is the resource name of the map
// that is modified.
func Map(p string, a, b dgo.Map, mods []*Modification) []*Modification {
	changedProps := vf.MutableMap()
	sb := &keyBuilder{}
	var ktm dgo.Array
	a.EachEntry(func(e dgo.MapEntry) {
		if b.ContainsKey(e.Key()) {
			return
		}
		if IsComplex(e.Value()) {
			mods = append(mods, &Modification{ResourceName: sb.makeKey(p, e.Key().String()), Type: Delete})
		} else {
			changedProps.Put(e.Key(), Deleted)
		}
		if ktm == nil {
			ktm = vf.MutableValues()
		}
		ktm.Add(e.Key())
	})
	if ktm != nil {
		a.RemoveAll(ktm)
	}

	b.EachEntry(func(e dgo.MapEntry) {
		k := e.Key()
		v := e.Value()
		old := a.Get(k)
		if v.Equals(old) {
			return
		}
		sk := sb.makeKey(p, k.String())
		switch old := old.(type) {
		case nil:
			if IsComplex(e.Value()) {
				mods = append(mods, &Modification{ResourceName: sk, Value: v, Type: Create})
			} else {
				changedProps.Put(k, v)
			}
			a.Put(k, v)
		case dgo.Map:
			if nv, ok := v.(dgo.Map); ok {
				mods = Map(sk, old, nv, mods)
			} else {
				// Change from map to something else
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
				a.Put(k, v)
			}
		case dgo.Array:
			if nv, ok := v.(dgo.Array); ok {
				mods = Array(sk, old, nv, mods)
			} else {
				// Change from array to something else
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
				a.Put(k, v)
			}
		default:
			if IsComplex(e.Value()) {
				// Change from simple to complex
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
			} else {
				changedProps.Put(k, v)
			}
			a.Put(k, v)
		}
	})
	if changedProps.Len() > 0 {
		mods = append(mods, &Modification{ResourceName: p, Value: changedProps, Type: Change})
	}
	return mods
}

// Array will make array a equal to array b and append all modifications needed in order to do that
// in the given mods slice. The new slice is returned. The string p is the resource name of the array
// that is modified.
func Array(p string, a, b dgo.Array, mods []*Modification) []*Modification {
	sb := &keyBuilder{}
	if a.Len() > b.Len() {
		t := a.Len()
		for i := b.Len(); i < t; i++ {
			if IsComplex(a.Get(i)) {
				mods = append(mods, &Modification{ResourceName: sb.makeKey(p, strconv.Itoa(i)), Type: Delete})
			} else {
				mods = append(mods, &Modification{ResourceName: p, Index: i, Type: Remove})
			}
		}
	}

	b.EachWithIndex(func(v dgo.Value, i int) {
		var old dgo.Value
		if i < a.Len() {
			old = a.Get(i)
			if v.Equals(old) {
				return
			}
		}
		sk := sb.makeKey(p, strconv.Itoa(i))
		switch old := old.(type) {
		case nil:
			if IsComplex(a.Get(i)) {
				// Adding new resource
				mods = append(mods, &Modification{ResourceName: sk, Value: v, Type: Create})
			} else {
				mods = append(mods, &Modification{ResourceName: p, Index: i, Value: v, Type: Add})
			}
			a.Add(v)
		case dgo.Map:
			if nv, ok := v.(dgo.Map); ok {
				mods = Map(sk, old, nv, mods)
			} else {
				// Change from map to something else
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
				a.Set(i, v)
			}
		case dgo.Array:
			if nv, ok := v.(dgo.Array); ok {
				mods = Array(sk, old, nv, mods)
			} else {
				// Change from array to something else
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
				a.Set(i, v)
			}
		default:
			if IsComplex(v) {
				// Change from simple to complex
				mods = append(mods, &Modification{ResourceName: sk, Type: Reset})
			} else {
				mods = append(mods, &Modification{ResourceName: p, Index: i, Value: v, Type: Set})
			}
			a.Set(i, v)
		}
	})
	return mods
}

type keyBuilder struct {
	strings.Builder
}

func (sb *keyBuilder) makeKey(p string, k string) string {
	if p == `` {
		return k
	}
	sb.Reset()
	_, _ = sb.WriteString(p)
	_ = sb.WriteByte('.')
	_, _ = sb.WriteString(k)
	return sb.String()
}
