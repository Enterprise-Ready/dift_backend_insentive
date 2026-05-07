package app

import (
	"context"
	"net/http"
	"time"
)

type App struct {
	HTTPServer interface {
		Start() error
		Shutdown(context.Context) error
	}
	Closers []func() error
}

func (a *App) Start() error { return a.HTTPServer.Start() }
func (a *App) Shutdown(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if a.HTTPServer != nil {
		_ = a.HTTPServer.Shutdown(cctx)
	}
	for _, closeFn := range a.Closers {
		_ = closeFn()
	}
	return nil
}
