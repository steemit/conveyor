package jsonrpc

// methodEntry holds a registered handler and whether it requires __signed
// authentication.
type methodEntry struct {
	handler Handler
	auth    bool
}

// Server is a registry of JSON-RPC methods. It dispatches incoming requests
// to registered handlers and produces JSON-RPC 2.0 responses, mirroring
// @steemit/koa-jsonrpc's JsonRpc class.
type Server struct {
	methods map[string]methodEntry
	auth    Authenticator
}

// NewServer creates an empty method registry.
func NewServer() *Server {
	return &Server{methods: make(map[string]methodEntry)}
}

// Register adds a public handler for the given method name. Panics on
// duplicate registration (matching koa-jsonrpc's assert).
func (s *Server) Register(name string, h Handler) {
	if _, exists := s.methods[name]; exists {
		panic("jsonrpc: method already exists: " + name)
	}
	s.methods[name] = methodEntry{handler: h, auth: false}
}

// RegisterAuthenticated adds a handler that requires a __signed request.
// Panics on duplicate registration. The Authenticator must be set via
// SetAuthenticator before dispatch processes any authenticated method.
func (s *Server) RegisterAuthenticated(name string, h Handler) {
	if _, exists := s.methods[name]; exists {
		panic("jsonrpc: method already exists: " + name)
	}
	s.methods[name] = methodEntry{handler: h, auth: true}
}

// SetAuthenticator injects the signature verifier used by authenticated
// methods. Must be called before the server starts handling requests.
func (s *Server) SetAuthenticator(a Authenticator) {
	s.auth = a
}
