package rpc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
)

func TestCreateHTTPServerUsesDefaultErrorEncoder(t *testing.T) {
	srv := CreateHTTPServer(testBootstrapConfig(), log.DefaultLogger)
	srv.Route("/").GET("/boom", func(_ kratosHttp.Context) error {
		return errors.New("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCreateHTTPServerWithOptionsInjectsErrorEncoder(t *testing.T) {
	srv := CreateHTTPServerWithOptions(
		testBootstrapConfig(),
		log.DefaultLogger,
		[]kratosHttp.ServerOption{
			kratosHttp.ErrorEncoder(func(w http.ResponseWriter, _ *http.Request, _ error) {
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	)
	srv.Route("/").GET("/boom", func(_ kratosHttp.Context) error {
		return errors.New("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func testBootstrapConfig() *conf.Bootstrap {
	return &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{
				Middleware: &conf.Middleware{},
			},
		},
	}
}
