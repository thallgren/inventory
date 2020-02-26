package bolt_test

import (
	"path/filepath"
	"testing"

	"github.com/puppetlabs/inventory/iapi"

	"github.com/lyraproj/dgo/dgo"
	"github.com/puppetlabs/inventory/query"

	require "github.com/lyraproj/dgo/dgo_test"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/bolt"
)

func TestGet_deep(t *testing.T) {
	b := bolt.NewStorage(staticDir())
	_, v := b.Get(`realm_a.mc1.config.transport`)
	require.Equal(t, v, `ssh`)
}

func TestQuery_group(t *testing.T) {
	b := bolt.NewStorage(staticDir())
	_, qr := b.Query(`targets`, vf.Map(`group`, `memcached`))
	require.Equal(t,
		vf.Values(
			vf.Map(
				`id`, `cmVhbG1fYS5tYzE=`,
				`name`, `mc1`,
				`realm`, `realm_a`,
				`uri`, `192.168.101.50`,
				`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`))),
			vf.Map(
				`id`, `cmVhbG1fYS5tYzI=`,
				`name`, `mc2`,
				`realm`, `realm_a`,
				`uri`, `192.168.101.60`,
				`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`)))),
		queryResult(qr))
}

func TestQuery_match(t *testing.T) {
	b := bolt.NewStorage(staticDir())
	_, qr := b.Query(`targets`, vf.Map(`target`, `172.16`))
	require.Equal(t,
		vf.Values(
			vf.Map(
				`id`, `cmVhbG1fYS4xNzIuMTYuMjE5LjIw`,
				`realm`, `realm_a`,
				`uri`, `172.16.219.20`,
				`config`, vf.Map(`transport`, `winrm`, `winrm`, vf.Map(`realm`, `MYDOMAIN`, `ssl`, false))),
			vf.Map(
				`id`, `cmVhbG1fYS4xNzIuMTYuMjE5LjMw`,
				`realm`, `realm_a`,
				`uri`, `172.16.219.30`,
				`config`, vf.Map(`transport`, `winrm`, `winrm`, vf.Map(`realm`, `MYDOMAIN`, `ssl`, false)))),
		queryResult(qr))
}

func TestGet_target(t *testing.T) {
	b := bolt.NewStorage(staticDir())
	_, trg := b.Get(`realm_a.mc1`)
	v, ok := trg.(iapi.Resource)
	require.True(t, ok)
	require.Equal(t,
		vf.Map(
			`id`, `cmVhbG1fYS5tYzE=`,
			`realm`, `realm_a`,
			`name`, `mc1`,
			`uri`, `192.168.101.50`,
			`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`))),
		v.DataMap())
}

func staticDir() string {
	return absTestDir(filepath.Join(`static`, `bolt`))
}

func absTestDir(dir string) string {
	path, err := filepath.Abs(filepath.Join(`..`, `testdata`, dir))
	if err != nil {
		panic(err)
	}
	return path
}

func queryResult(qr query.Result) dgo.Value {
	if qr == nil {
		return nil
	}
	if qr.Singleton() {
		value := qr.Value(0)
		if r, ok := value.(iapi.Resource); ok {
			value = r.DataMap()
		}
		return value
	}
	if qr.IsMap() {
		m := vf.MapWithCapacity(qr.Len())
		qr.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
			if r, ok := value.(iapi.Resource); ok {
				value = r.DataMap()
			}
			m.Put(ref, value)
		})
		return m
	}

	a := vf.ArrayWithCapacity(qr.Len())
	qr.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		if r, ok := value.(iapi.Resource); ok {
			value = r.DataMap()
		}
		a.Add(value)
	})
	return a
}
