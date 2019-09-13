package muxfs

import (
	"context"
	"fmt"

	"github.com/qri-io/qfs"
)

// NewMux creates a new path muxer
func NewMux(handlers map[string]qfs.PathResolver) *Mux {
	return &Mux{handlers: handlers}
}

// Mux is a filesystem that combines PathResolvers using path multiplexing.
// It's a way to use multiple filesystem implementations as a single PathResolver.
type Mux struct {
	handlers map[string]qfs.PathResolver
}

// SetHandler designates the resolver for a given path kind string
func (m *Mux) SetHandler(pathKind string, resolver qfs.PathResolver) {
	if m.handlers == nil {
		m.handlers = map[string]qfs.PathResolver{}
	}
	m.handlers[pathKind] = resolver
}

// Get impoements the qfs.PathResolver interface
func (m Mux) Get(ctx context.Context, path string) (qfs.File, error) {
	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return nil, fmt.Errorf("cannot resolve paths of kind '%s'. path: %s", kind, path)
	}

	return handler.Get(ctx, path)
}
