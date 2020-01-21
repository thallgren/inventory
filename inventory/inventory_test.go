package inventory_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/puppetlabs/inventory/inventory"

	logger2 "github.com/jirenius/go-res/logger"
	"github.com/jirenius/go-res/test"

	"github.com/lyraproj/dgo/vf"

	"github.com/jirenius/go-res"
	"github.com/lyraproj/dgo/dgo"
	require "github.com/lyraproj/dgo/dgo_test"
	"github.com/lyraproj/dgo/streamer"
)

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

func TestListRealms(t *testing.T) {
	s, cl := createSession(t)
	require.Equal(t, vf.Map(`realmA`, `https://some.realm.com`), get("inventory.lookup.realms", s, t))
	shutdownSession(s, cl)
}

func TestListNodes(t *testing.T) {
	s, cl := createSession(t)
	require.Equal(t, vf.Map(`nodeA`, `Node A`, `nodeB`, `Node B`), get("inventory.lookup.realmA.nodes", s, t))
	shutdownSession(s, cl)
}

func TestGetFact(t *testing.T) {
	s, cl := createSession(t)
	require.Equal(t, vf.Values(`first`, `second`), get("inventory.lookup.realmA.nodeA.a", s, t))
	shutdownSession(s, cl)
}

func TestSetFact(t *testing.T) {
	s, cl := createSession(t)
	delete("inventory.lookup.realmA.nodeA.n", s, t)
	shutdownSession(s, cl)

	s, cl = createSession(t)
	set("inventory.lookup.realmA.nodeA.n", vf.Map(`value`, `value of n`), s, t)
	shutdownSession(s, cl)
}

func TestGetFacts(t *testing.T) {
	s, cl := createSession(t)
	require.Equal(t, vf.Map(`a`, `value of a`, `b`, `value of b`), get("inventory.lookup.realmA.nodeB.facts", s, t))
	shutdownSession(s, cl)
}

func createSession(t *testing.T) (*test.Session, chan struct{}) {
	t.Helper()

	var s *test.Session
	c := test.NewTestConn(false)
	r := res.NewService("inventory")
	logger := logger2.NewMemLogger()
	r.SetLogger(logger)

	s = &test.Session{
		MockConn: c,
		Service:  r,
	}
	cl := make(chan struct{})

	inventory.NewService("testdata", `realms`, `nodes`, `facts`).AddHandlers(r)

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

func set(rid string, v dgo.Map, s *test.Session, t *testing.T) {
	t.Helper()
	s.Request(`call.`+rid+`.set`, &request{Params: streamer.MarshalJSON(v, nil)})
	msg := s.GetMsg(t)
	require.Equal(t, msg.Subject, `event.`+rid+`.change`)
	require.Equal(t, v, parseMessage(msg, `values`, s, t))
}

func delete(rid string, s *test.Session, t *testing.T) {
	t.Helper()
	s.Request(`call.`+rid+`.delete`, &request{})
	msg := s.GetMsg(t)
	require.Equal(t, msg.Subject, `event.`+rid+`.delete`)
}
