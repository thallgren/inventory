package inventory_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jirenius/go-res"
	"github.com/jirenius/go-res/logger"
	"github.com/jirenius/go-res/test"
	"github.com/lyraproj/dgo/dgo"
	require "github.com/lyraproj/dgo/dgo_test"
	"github.com/lyraproj/dgo/streamer"
	"github.com/lyraproj/dgo/vf"
	"github.com/lyraproj/dgoyaml/yaml"
	"github.com/puppetlabs/inventory/file"
	"github.com/puppetlabs/inventory/inventory"
)

func TestGetFact(t *testing.T) {
	s, cl := createSession(staticDir(), t)
	require.Equal(t, vf.Values(`first`, `second`), get("inventory.realmA.nodeA.a", s, t))
	shutdownSession(s, cl)
}

func TestListRealms(t *testing.T) {
	s, cl := createSession(staticDir(), t)
	require.Equal(t, vf.Map(`realmA`, `https://some.realm.com`), get("inventory.realms", s, t))
	shutdownSession(s, cl)
}

func TestListNodes(t *testing.T) {
	s, cl := createSession(staticDir(), t)
	require.Equal(t, vf.Map(`nodeA`, `Node A`, `nodeB`, `Node B`), get("inventory.realmA.nodes", s, t))
	shutdownSession(s, cl)
}

func TestGetFacts(t *testing.T) {
	s, cl := createSession(staticDir(), t)
	require.Equal(t, vf.Map(`a`, `value of a`, `b`, `value of b`), get("inventory.realmA.nodeB.facts", s, t))
	shutdownSession(s, cl)
}

func TestDeleteFact(t *testing.T) {
	createNode(`realmX`, `nodeA`, vf.Map(`a`, `value of a`), t)
	s, cl := createSession(volatileDir(), t)
	remove("inventory.realmX.nodeA.a", s, t)
	shutdownSession(s, cl)
	ensureNode(`realmX`, `nodeA`, vf.Map(), t)
}

func TestDeleteNode(t *testing.T) {
	createNode(`realmX`, `nodeD`, vf.Map(`a`, `value of a`), t)
	s, cl := createSession(volatileDir(), t)
	remove("inventory.realmX.nodeD", s, t)
	shutdownSession(s, cl)
	ensureNoNode(`realmX`, `nodeD`, t)
}

func TestSetFact(t *testing.T) {
	createNode(`realmY`, `nodeA`, vf.Map(`a`, `value of a`), t)
	s, cl := createSession(volatileDir(), t)
	set("inventory.realmY.nodeA", vf.Map(`n`, `value of n`), s, t)
	shutdownSession(s, cl)
	ensureNode(`realmY`, `nodeA`, vf.Map(`a`, `value of a`, `n`, `value of n`), t)
}

func TestNewNode(t *testing.T) {
	createNode(`realmY`, `nodeA`, vf.Map(`a`, `value of a`), t)
	deleteNode(`realmY`, `nodeB`, t)
	s, cl := createSession(volatileDir(), t)
	set("inventory.realmY.nodeB", vf.Map(`__value`, `Node B`), s, t)
	shutdownSession(s, cl)
	ensureNode(`realmY`, `nodeB`, vf.Map(), t)
}

func createSession(dir string, t *testing.T) (*test.Session, chan struct{}) {
	t.Helper()

	var s *test.Session
	c := test.NewTestConn(false)
	r := res.NewService("inventory")
	r.SetLogger(logger.NewMemLogger())

	s = &test.Session{
		MockConn: c,
		Service:  r,
	}
	cl := make(chan struct{})

	inventory.NewService(file.NewStorage(dir, `realms`, `nodes`, `facts`)).AddHandlers(r)

	go func() {
		defer s.StopServer()
		defer close(cl)
		if err := r.Serve(c); err != nil {
			panic("test: failed to start service: " + err.Error())
		}
	}()

	s.GetMsg(t).AssertSubject(t, "system.reset")

	return s, cl
}

const timeoutDuration = 5 * time.Second

func shutdownSession(s *test.Session, cl chan struct{}) {
	err := s.Shutdown()

	// Check error, as an error means that server hasn't had
	// time to start. We can then ignore waiting for the closing
	if err == nil {
		select {
		case <-cl:
		case <-time.After(timeoutDuration):
			panic("test: failed to shutdown service: timeout")
		}
	}
}

func parseMessage(msg *test.Msg, path string, s *test.Session, t *testing.T) dgo.Value {
	t.Helper()
	return resolveRefs(vf.Value(msg.PathPayload(t, path)), s, t, true)
}

func resolveRefs(v dgo.Value, s *test.Session, t *testing.T, top bool) dgo.Value {
	t.Helper()
	switch v := v.(type) {
	case dgo.Map:
		if v.Len() == 1 {
			if rid, ok := v.Get(`rid`).(dgo.String); ok {
				return get(rid.GoString(), s, t)
			}
			if top {
				if m, ok := v.Get(`model`).(dgo.Map); ok {
					if mv := m.Get(`value`); mv != nil && m.Len() == 1 {
						return resolveRefs(mv, s, t, true)
					}
					return resolveRefs(m, s, t, false)
				}
				if c := v.Get(`collection`); c != nil {
					return c
				}
			}
		}
		return v.Map(func(e dgo.MapEntry) interface{} { return resolveRefs(e.Value(), s, t, false) })
	case dgo.Array:
		return v.Map(func(e dgo.Value) interface{} { return resolveRefs(e, s, t, false) })
	default:
		return v
	}
}

func get(rid string, s *test.Session, t *testing.T) dgo.Value {
	t.Helper()
	inb := s.Request(`get.`+rid, &request{})
	msg := s.GetMsg(t)
	require.Equal(t, msg.Subject, inb)
	return parseMessage(msg, `result`, s, t)
}

type request struct {
	CID        string              `json:"cid,omitempty"`
	Params     json.RawMessage     `json:"params,omitempty"`
	Token      json.RawMessage     `json:"token,omitempty"`
	Header     map[string][]string `json:"header,omitempty"`
	Host       string              `json:"host,omitempty"`
	RemoteAddr string              `json:"remoteAddr,omitempty"`
	URI        string              `json:"uri,omitempty"`
	Query      string              `json:"query,omitempty"`
}

func set(rid string, v dgo.Map, s *test.Session, t *testing.T) {
	t.Helper()
	s.Request(`call.`+rid+`.set`, &request{Params: streamer.MarshalJSON(v, nil)})
	msg := s.GetMsg(t)
	require.Equal(t, msg.Subject, `event.`+rid+`.change`)
	require.Equal(t, v, parseMessage(msg, `values`, s, t))
}

func remove(rid string, s *test.Session, t *testing.T) {
	t.Helper()
	s.Request(`call.`+rid+`.delete`, &request{})
	msg := s.GetMsg(t)
	require.Equal(t, msg.Subject, `event.`+rid+`.delete`)
}

func createNode(realm, node string, facts dgo.Map, t *testing.T) {
	t.Helper()

	// ensure that there's nothing there
	realmDir := filepath.Join(volatileDir(), realm)
	createLevel(realmDir, vf.Map(`__value`, realm), t)
	createLevel(filepath.Join(realmDir, node), facts.Merge(vf.Map(`__value`, node)), t)
}

func createLevel(path string, data dgo.Value, t *testing.T) {
	err := os.MkdirAll(path, 0750)
	if err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	yml, err := yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(path, `data.yaml`), yml, 0640)
	if err != nil {
		t.Fatal(err)
	}
}

func deleteNode(realm, node string, t *testing.T) {
	err := os.RemoveAll(filepath.Join(volatileDir(), realm, node))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func ensureNode(realm, node string, facts dgo.Map, t *testing.T) {
	path := filepath.Join(volatileDir(), realm, node, `data.yaml`)
	/* #nosec */
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := yaml.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if dh, ok := dv.(dgo.Map); ok {
		require.Equal(t, facts, dh.Without(`__value`))
	} else {
		t.Fatalf(`the file %q does not contain a map of values`, path)
	}
}

func ensureNoNode(realm, node string, t *testing.T) {
	_, err := os.Stat(filepath.Join(volatileDir(), realm, node))
	if err == nil {
		t.Fatalf(`node %s.%s exists`, realm, node)
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func staticDir() string {
	return absTestDir(`static`)
}

func volatileDir() string {
	return absTestDir(`volatile`)
}

func absTestDir(dir string) string {
	path, err := filepath.Abs(filepath.Join(`..`, `testdata`, dir))
	if err != nil {
		panic(err)
	}
	return path
}
