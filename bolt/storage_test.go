package bolt_test

import (
	"path/filepath"
	"testing"

	"github.com/lyraproj/dgo/dgo"
	"github.com/puppetlabs/inventory/query"

	require "github.com/lyraproj/dgo/dgo_test"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/bolt"
)

func TestGet_deep(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	_, v := b.Get(`mc1.config.transport`)
	require.Equal(t, v, `ssh`)
}

func TestQuery_group(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	_, qr := b.Query(`targets`, vf.Map(`group`, `memcached`))
	require.Equal(t,
		vf.Values(
			vf.Map(
				`name`, `mc1`,
				`uri`, `192.168.101.50`,
				`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`))),
			vf.Map(`name`, `mc2`,
				`uri`, `192.168.101.60`,
				`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`)))),
		queryResult(qr))
}

func TestQuery_match(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	_, qr := b.Query(`targets`, vf.Map(`match`, `172.16`))
	require.Equal(t,
		vf.Values(
			vf.Map(
				`uri`, `172.16.219.20`,
				`config`, vf.Map(`transport`, `winrm`, `winrm`, vf.Map(`realm`, `MYDOMAIN`, `ssl`, false))),
			vf.Map(
				`uri`, `172.16.219.30`,
				`config`, vf.Map(`transport`, `winrm`, `winrm`, vf.Map(`realm`, `MYDOMAIN`, `ssl`, false)))),
		queryResult(qr))
}

func TestGet_target(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	_, v := b.Get(`mc1`)
	require.Equal(t,
		vf.Map(
			`name`, `mc1`,
			`uri`, `192.168.101.50`,
			`config`, vf.Map(`transport`, `ssh`, `ssh`, vf.Map(`user`, `root`))),
		v)
}

func staticDir() string {
	return absTestDir(`static`)
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
		return qr.Value(0)
	}
	if qr.IsMap() {
		m := vf.MapWithCapacity(qr.Len(), nil)
		qr.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
			m.Put(ref, value)
		})
		return m
	}

	a := vf.ArrayWithCapacity(nil, qr.Len())
	qr.EachWithRefAndIndex(func(value, ref dgo.Value, index int) {
		a.Add(value)
	})
	return a
}
