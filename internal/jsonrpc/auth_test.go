package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// mockAuthenticator is a test double for the Authenticator interface. It
// returns canned values without any cryptographic verification.
type mockAuthenticator struct {
	account  string
	decoded  json.RawMessage
	err      *Error
	called   bool
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, string, *Error) {
	m.called = true
	if m.err != nil {
		return nil, m.account, m.err
	}
	return m.decoded, m.account, nil
}

func TestHasSignedWrapper(t *testing.T) {
	cases := []struct {
		name   string
		params string
		want   bool
	}{
		{"object with __signed", `{"__signed":{"account":"foo"}}`, true},
		{"object without __signed", `{"account":"foo"}`, false},
		{"empty params", ``, false},
		{"array params", `["foo"]`, false},
		{"__signed null", `{"__signed":null}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := hasSignedWrapper(json.RawMessage(c.params))
			if got != c.want {
				t.Errorf("hasSignedWrapper(%s) = %v, want %v", c.params, got, c.want)
			}
		})
	}
}

func TestAuth_NoSignedWrapper_InvalidParams(t *testing.T) {
	s := NewServer()
	s.SetAuthenticator(&mockAuthenticator{account: "foo", decoded: json.RawMessage(`{}`)})
	s.RegisterAuthenticated("secret", func(ctx *Context, req *Request) (any, error) {
		t.Fatal("handler should not be called without __signed")
		return nil, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"secret","params":{"command":"hi"}}`)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error == nil || resp.Error.Code != InvalidParams {
		t.Fatalf("expected InvalidParams (-32602), got: %+v", resp.Error)
	}
}

func TestAuth_SignedRequest_Success(t *testing.T) {
	mock := &mockAuthenticator{
		account: "testuser",
		decoded: json.RawMessage(`{"command":"make me a sandwich"}`),
	}
	s := NewServer()
	s.SetAuthenticator(mock)
	var receivedAccount string
	var receivedCommand string
	s.RegisterAuthenticated("sudo", func(ctx *Context, req *Request) (any, error) {
		receivedAccount = ctx.Account
		var p struct{ Command string `json:"command"` }
		_ = req.UnmarshalParams(&p)
		receivedCommand = p.Command
		return "sudo " + p.Command, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"sudo","params":{"__signed":{"account":"testuser"}}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected success, got: %+v", resp)
	}
	if !mock.called {
		t.Fatal("Authenticator was not called")
	}
	if receivedAccount != "testuser" {
		t.Errorf("expected account 'testuser', got '%s'", receivedAccount)
	}
	if receivedCommand != "make me a sandwich" {
		t.Errorf("expected command 'make me a sandwich', got '%s'", receivedCommand)
	}
}

func TestAuth_AuthenticatorError_Propagated(t *testing.T) {
	authErr := NewError(401, nil, "Unauthorized: Invalid signature")
	s := NewServer()
	s.SetAuthenticator(&mockAuthenticator{account: "foo", err: authErr})
	s.RegisterAuthenticated("secret", func(ctx *Context, req *Request) (any, error) {
		t.Fatal("handler should not be called on auth failure")
		return nil, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"secret","params":{"__signed":{"account":"foo"}}}`)
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != 401 {
		t.Errorf("expected code 401, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Invalid signature") {
		t.Errorf("expected 'Invalid signature' in message, got: %s", resp.Error.Message)
	}
}

func TestAuth_PublicMethod_Unaffected(t *testing.T) {
	s := NewServer()
	s.SetAuthenticator(&mockAuthenticator{})
	s.Register("hello", func(ctx *Context, req *Request) (any, error) {
		return "hi", nil
	})
	// A plain params object (no __signed) should work fine for public methods.
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"hello","params":{"name":"dave"}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected success, got: %+v", resp)
	}
}

func TestAuth_Notification_NoResponse(t *testing.T) {
	s := NewServer()
	s.SetAuthenticator(&mockAuthenticator{account: "foo", decoded: json.RawMessage(`{}`)})
	s.RegisterAuthenticated("secret", func(ctx *Context, req *Request) (any, error) {
		return "ok", nil
	})
	// Notification (no id) + __signed → should not produce a response.
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","method":"secret","params":{"__signed":{"account":"foo"}}}`)
	if resp != nil {
		t.Fatalf("notification should return nil, got %+v", resp)
	}
}

func TestAuth_Notification_NoSigned_NoResponse(t *testing.T) {
	s := NewServer()
	s.SetAuthenticator(&mockAuthenticator{})
	s.RegisterAuthenticated("secret", func(ctx *Context, req *Request) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	// Notification without __signed → InvalidParams but no response (notification).
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","method":"secret","params":{"x":"y"}}`)
	if resp != nil {
		t.Fatalf("notification should return nil even on InvalidParams, got %+v", resp)
	}
}

func TestAuth_AuthNotConfigured(t *testing.T) {
	s := NewServer()
	// No SetAuthenticator call.
	s.RegisterAuthenticated("secret", func(ctx *Context, req *Request) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"secret","params":{"__signed":{"account":"foo"}}}`)
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != InternalError {
		t.Errorf("expected InternalError (-32603), got %d", resp.Error.Code)
	}
}
