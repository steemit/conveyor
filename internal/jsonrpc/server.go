package jsonrpc

// Server is a registry of JSON-RPC methods. It dispatches incoming requests
// to registered handlers and produces JSON-RPC 2.0 responses, mirroring
// @steemit/koa-jsonrpc's JsonRpc class.
type Server struct {
	methods map[string]Handler
}

// NewServer creates an empty method registry.
func NewServer() *Server {
	return &Server{methods: make(map[string]Handler)}
}

// Register adds a handler for the given method name. Panics on duplicate
// registration (matching koa-jsonrpc's assert).
func (s *Server) Register(name string, h Handler) {
	if _, exists := s.methods[name]; exists {
		panic("jsonrpc: method already exists: " + name)
	}
	s.methods[name] = h
}
