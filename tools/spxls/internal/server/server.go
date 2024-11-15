package server

import (
	"errors"
	"io/fs"

	"github.com/goplus/builder/tools/spxls/internal/jsonrpc2"
)

type MessageReplier interface {
	// ReplyMessage sends a message back to the client.
	ReplyMessage(m jsonrpc2.Message) error
}

type Server struct {
	rootFS  fs.ReadDirFS
	replier MessageReplier
}

func New(rootFS fs.ReadDirFS, replier MessageReplier) *Server {
	return &Server{
		rootFS:  rootFS,
		replier: replier,
	}
}

func (s *Server) HandleMessage(m jsonrpc2.Message) error {
	return errors.ErrUnsupported
}
