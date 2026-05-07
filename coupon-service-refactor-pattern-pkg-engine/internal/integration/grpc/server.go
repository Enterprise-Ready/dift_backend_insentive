package grpcserver

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc"
)

type Server struct {
	server  *grpc.Server
	address string
}

func NewServer(address string) *Server {
	return &Server{
		server:  grpc.NewServer(),
		address: normalizeAddress(address),
	}
}

func (s *Server) Register(register func(*grpc.Server)) {
	register(s.server)
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}

	return s.server.Serve(lis)
}

func (s *Server) Stop(ctx context.Context) {
	_ = ctx
	s.server.GracefulStop()
}

func normalizeAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return address
	}

	return ":" + address
}
