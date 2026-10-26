package api

import (
	"context"
	"net"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/testutil"
)

func TestShutdownClosesPool(t *testing.T) {
	db := testutil.Open(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	app := &App{
		logger: zap.NewNop(),
		db:     db,
		server: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- app.server.Serve(listener)
	}()

	if err := app.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("expected closed database pool")
	}
	if err := <-serveErr; err != nil && err != http.ErrServerClosed {
		t.Fatalf("serve: %v", err)
	}
}
