// Package yaml contains functions to read and write yaml
package yaml

import (
	"fmt"
	"io/ioutil"

	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgoyaml/yaml"
)

// Write a yaml map to the given file. Panic if something goes wrong.
func Write(path string, pf dgo.Value) {
	yml, err := yaml.Marshal(pf)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(path, yml, 0640)
	if err != nil {
		panic(err)
	}
}

// Read a yaml map from the given file. Panic if something goes wrong.
func Read(path string) dgo.Map {
	/* #nosec */
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	dv, err := yaml.Unmarshal(data)
	if err != nil {
		panic(err)
	}
	if dh, ok := dv.(dgo.Map); ok {
		return dh
	}
	panic(fmt.Errorf(`the file %q does not contain a map of values`, path))
}
