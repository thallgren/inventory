// Package file contains the storage for the directory/file based hierarchical storage
package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/puppetlabs/inventory/change"

	"github.com/puppetlabs/inventory/query"

	"github.com/gofrs/flock"
	"github.com/lyraproj/dgo/dgo"
	"github.com/lyraproj/dgo/vf"
	"github.com/puppetlabs/inventory/iapi"
	"github.com/puppetlabs/inventory/yaml"
)

const valueKey = `__value`

type fileStorage struct {
	dataDir string
	hns     []string
}

// NewStorage creates a Storage that is using the file system to persist data
func NewStorage(dataDir string, hierarchyNames ...string) iapi.Storage {
	return &fileStorage{dataDir: dataDir, hns: hierarchyNames}
}

func (f *fileStorage) Delete(key string) ([]*change.Modification, bool) {
	parts := strings.Split(key, `.`)
	lp := len(parts) - 1
	if lp < 1 {
		return nil, false
	}
	if f.deleteChild(parts) {
		return nil, true
	}

	// Delete from data.yaml
	key = parts[lp]
	parts = parts[:lp]
	path := filepath.Join(f.dataDir, filepath.Join(parts...), `data.yaml`)
	lock := flock.New(path)
	err := lock.RLock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		panic(err)
	}

	defer func() {
		_ = lock.Close()
	}()
	pf := yaml.Read(path).Copy(false) // read, then thaw frozen map
	if pf.Remove(key) != nil {
		yaml.Write(path, pf)
		return nil, true
	}
	return nil, false
}

func (f *fileStorage) Get(key string) ([]*change.Modification, dgo.Value) {
	parts := strings.Split(key, `.`)
	pf := f.readData(parts)
	if pf != nil {
		return nil, pf.Get(valueKey)
	}
	lp := len(parts) - 1
	last := parts[lp]
	parts = parts[:lp]
	if lp > 0 {
		if pf = f.readData(parts); pf == nil {
			return nil, nil
		}
		if v := pf.Get(last); v != nil {
			return nil, v
		}
	}
	if lp < len(f.hns) && last == f.hns[lp] {
		// Collect names of subdirectories.
		children := f.readChildMap(parts)
		if pf != nil {
			children = children.Merge(pf.WithoutAll(vf.Values(valueKey)))
		}
		return nil, children
	}
	return nil, nil
}

func (f *fileStorage) Query(key string, _ dgo.Map) (mods []*change.Modification, qr query.Result) {
	mods, v := f.Get(key)
	switch v := v.(type) {
	case nil:
	case dgo.Array:
		qr = query.NewResult(false)
		v.EachWithIndex(func(e dgo.Value, idx int) {
			qr.Add(vf.Integer(int64(idx)), e)
		})
	case dgo.Map:
		qr = query.NewResult(true)
		v.EachEntry(func(e dgo.MapEntry) {
			qr.Add(e.Key(), e.Value())
		})
	default:
		qr = query.NewSingleResult(v)
	}
	return mods, qr
}

func (f *fileStorage) QueryKeys(_ string) []query.Param {
	return []query.Param{} // Not queryable at this time
}

func (f *fileStorage) Refresh() []*change.Modification {
	return []*change.Modification{}
}

func (f *fileStorage) Set(key string, model dgo.Map) ([]*change.Modification, error) {
	if model.Len() == 0 {
		return nil, nil
	}
	parts := strings.Split(key, `.`)
	lp := len(parts) - 1
	if lp < 0 {
		return nil, iapi.NotFound(``)
	}
	path := filepath.Join(f.dataDir, filepath.Join(parts...), `data.yaml`)

	var mods []*change.Modification
	var pf dgo.Map
	lock := flock.New(path)
	err := lock.RLock()
	if err == nil {
		defer func() {
			_ = lock.Close()
		}()
		pf = yaml.Read(path).Copy(false) // read, then thaw frozen map
		mods = change.Map(key, pf, pf.Merge(model), mods)
	} else {
		if !os.IsNotExist(err) {
			panic(err)
		}

		// A non existing data.yaml is OK if this is an attempt to create a new hierarchy entry. Such
		// an attempt is only allowed if the model is a one element map with keyed by the valueKey
		if value := model.Get(valueKey); value != nil && model.Len() == 1 {
			f.createChild(parts)
			pf = model
			mods = append(mods, &change.Modification{ResourceName: key, Type: change.Create, Value: model})
		} else {
			return nil, iapi.NotFound(key)
		}
	}
	yaml.Write(path, pf)
	return mods, nil
}

func (f *fileStorage) createChild(parts []string) {
	dirPath := filepath.Join(f.dataDir, filepath.Join(parts...))
	_, err := os.Stat(dirPath)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		// Directory does not exist. Ensure that parent directory does. It's always an
		// error if the parent doesn't exist (can't add a node to a non existing realm).
		pParts := parts[:len(parts)-1]
		pDir := filepath.Join(f.dataDir, filepath.Join(pParts...))
		pd, err := os.Stat(pDir)
		if err != nil {
			if !os.IsNotExist(err) {
				panic(err)
			}
			panic(iapi.NotFound(strings.Join(pParts, `.`)))
		}
		if !pd.IsDir() {
			panic(fmt.Errorf(`%q is not a directory`, pDir))
		}

		// Parent exists, so create the directory that represents the child.
		err = os.Mkdir(dirPath, 0750)
		if err != nil {
			panic(err)
		}
	}
}

func (f *fileStorage) deleteChild(parts []string) bool {
	path := filepath.Join(f.dataDir, filepath.Join(parts...))
	ds, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	if ds.IsDir() {
		if err = os.RemoveAll(path); err != nil {
			panic(err)
		}
		return true
	}
	panic(fmt.Errorf(`%q is not a directory`, path))
}

func (f *fileStorage) readChildMap(parts []string) dgo.Map {
	dir := filepath.Join(f.dataDir, filepath.Join(parts...))
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		panic(err)
	}

	children := vf.MutableMap()
	for _, file := range files {
		if file.IsDir() {
			dh := f.readData(append(parts, file.Name()))
			children.Put(file.Name(), dh.Get(valueKey))
		}
	}
	return children
}

func (f *fileStorage) readData(parts []string) dgo.Map {
	path := filepath.Join(f.dataDir, filepath.Join(parts...), `data.yaml`)
	lock := flock.New(path)
	if err := lock.RLock(); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		panic(err)
	}
	defer func() {
		_ = lock.Close()
	}()
	return yaml.Read(path)
}
