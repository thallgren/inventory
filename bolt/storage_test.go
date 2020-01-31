package bolt_test

import (
	"path/filepath"
	"testing"

	require "github.com/lyraproj/dgo/dgo_test"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/bolt"
)

func TestNewBoltStorage(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	require.Equal(t, b.Get(`win_nodes.config.transport`), `winrm`)
}

func TestNewBoltStorage_group(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	require.Equal(t, b.Get(`ssh_nodes.memcached`),
		vf.Map(
			`targets`, vf.Values(
				vf.Map(`name`, `mc1`, `uri`, `192.168.101.50`),
				vf.Map(`name`, `mc2`, `uri`, `192.168.101.60`)),
			`config`, vf.Map(
				`ssh`, vf.Map(`user`, `root`))))
}

func TestNewBoltStorage_target(t *testing.T) {
	b := bolt.NewStorage(filepath.Join(staticDir(), `bolt_inventory.yaml`))
	require.Equal(t, b.Get(`ssh_nodes.memcached.mc1`), vf.Map(`uri`, `192.168.101.50`))
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
