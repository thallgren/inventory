// Package change contains functions needed to generate Modifications when changing Maps and Arrays
package change

import (
	"strconv"

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

func modifyEntry(p string, a dgo.Map, e dgo.MapEntry, changedProps dgo.Map, mods []*Modification) []*Modification {
	k := e.Key()
	sk := p + `.` + k.String()
	v := e.Value()
	old := a.Get(k)
	if old == nil {
		if IsComplex(v) {
			mods = append(mods, &Modification{ResourceName: p + `.` + k.String(), Value: v, Type: Create})
		}
		changedProps.Put(k, v)
		a.Put(k, v)
		return mods
	}

	if v.Equals(old) {
		return mods
	}
	switch old := old.(type) {
	case Identifiable:
		if nv, ok := v.(Identifiable); ok && old.ID() == nv.ID() {
			return old.UpdateFrom(nv, mods)
		}
	case dgo.Map:
		if nv, ok := v.(dgo.Map); ok {
			return Map(sk, old, nv, mods)
		}
	case dgo.Array:
		if nv, ok := v.(dgo.Array); ok {
			return Array(sk, old, nv, mods)
		}
	}
	a.Put(k, v)
	changedProps.Put(k, v)
	return mods
}

// Map will make map a equal to map b and append all modifications needed in order to do that
// in the given mods slice. The new slice is returned. The string p is the resource name of the map
// that is modified.
func Map(p string, a, b dgo.Map, mods []*Modification) []*Modification {
	changedProps := vf.MutableMap()
	var ktm dgo.Array
	a.EachEntry(func(e dgo.MapEntry) {
		k := e.Key()
		if !b.ContainsKey(k) {
			changedProps.Put(k, Deleted)
			if ktm == nil {
				ktm = vf.MutableValues()
			}
			ktm.Add(k)
		}
	})
	b.EachEntry(func(e dgo.MapEntry) { mods = modifyEntry(p, a, e, changedProps, mods) })
	if changedProps.Len() > 0 {
		mods = append(mods, &Modification{ResourceName: p, Value: changedProps, Type: Change})
	}
	if ktm != nil {
		ktm.Each(func(k dgo.Value) {
			if IsComplex(a.Get(k)) {
				mods = append(mods, &Modification{ResourceName: p + `.` + k.String(), Type: Delete})
			}
		})
		a.RemoveAll(ktm)
	}
	return mods
}

func modifyElement(p string, a dgo.Array, i int, v dgo.Value, mods []*Modification) []*Modification {
	if i >= a.Len() {
		a.Add(v)
		return append(mods, &Modification{ResourceName: p, Index: i, Value: v, Type: Add})
	}

	old := a.Get(i)
	if v.Equals(old) {
		return mods
	}
	switch old := old.(type) {
	case Identifiable:
		if nv, ok := v.(Identifiable); ok && old.ID() == nv.ID() {
			return old.UpdateFrom(nv, mods)
		}
	case dgo.Map:
		if nv, ok := v.(dgo.Map); ok {
			return Map(p+`.`+strconv.Itoa(i), old, nv, mods)
		}
	case dgo.Array:
		if nv, ok := v.(dgo.Array); ok {
			return Array(p+`.`+strconv.Itoa(i), old, nv, mods)
		}
	}
	a.Set(i, v)
	return append(mods, &Modification{ResourceName: p, Index: i, Value: v, Type: Set})
}

func sameElement(a, b dgo.Value) bool {
	if am, ok := a.(Identifiable); ok {
		if bm, ok := b.(Identifiable); ok {
			return am.ID() == bm.ID()
		}
	}
	return a.Equals(b)
}

// Array will make array a equal to array b and append all modifications needed in order to do that
// in the given mods slice. The new slice is returned. The string p is the resource name of the array
// that is modified.
func Array(p string, a, b dgo.Array, mods []*Modification) []*Modification {
	// Remove from end to start
	for ix := a.Len() - 1; ix >= 0; ix-- {
		ae := a.Get(ix)
		if !b.Any(func(be dgo.Value) bool { return sameElement(ae, be) }) {
			mods = append(mods, &Modification{ResourceName: p, Index: ix, Type: Remove})
			a.Remove(ix)
		}
	}

	b.EachWithIndex(func(be dgo.Value, i int) {
		if !a.Any(func(ae dgo.Value) bool { return sameElement(ae, be) }) {
			if i >= a.Len() {
				i = a.Len()
			}
			mods = append(mods, &Modification{ResourceName: p, Index: i, Value: be, Type: Add})
			a.Insert(i, be)
		}
	})

	a.EachWithIndex(func(ae dgo.Value, i1 int) {
		i2 := 0
		b.Find(func(be dgo.Value) interface{} {
			if i1 != i2 && sameElement(ae, be) {
				mods = append(mods, &Modification{ResourceName: p, Index: i1, Type: Remove})
				mods = append(mods, &Modification{ResourceName: p, Index: i2, Value: be, Type: Add})
				a.Remove(i1)
				a.Insert(i2, be)
			}
			i2++
			return nil
		})
	})

	b.EachWithIndex(func(v dgo.Value, i int) { mods = modifyElement(p, a, i, v, mods) })
	return mods
}
