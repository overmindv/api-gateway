package content

import (
	"context"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type tlsCatalog struct {
	pb.UnimplementedContentServiceServer
}

func (tlsCatalog) ListContentCatalog(context.Context, *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error) {
	return &pb.ListContentCatalogResponse{}, nil
}

func TestContentClientVerifiesTLSBeforeSendingCredential(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	cfg := tlsCatalogConfig(t, func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		calls.Add(1)
		token := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(token) != 1 || token[0] != "Bearer "+fixtureServiceToken {
			return nil, status.Error(codes.Unauthenticated, "missing service token")
		}

		return next(ctx, req)
	})
	actor := Actor{UserID: "00000000-0000-4000-8000-000000000101"}
	trustedCA := cfg.CAFile
	cfg.CAFile = ""
	bad, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bad.Close() })
	var upstream *Error
	_, err = bad.List(t.Context(), actor, &pb.ListContentCatalogRequest{})
	if !errors.As(err, &upstream) || upstream.Code != codeUnavailable || calls.Load() != 0 {
		t.Error("untrusted connection reached the service", err, calls.Load())
	}

	cfg.CAFile = trustedCA
	good, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = good.Close() })
	if _, err := good.List(t.Context(), actor, &pb.ListContentCatalogRequest{}); err != nil || calls.Load() != 1 {
		t.Error("trusted connection did not reach the service", err, calls.Load())
	}

	cfg.Insecure = true
	if _, err := New(cfg); err == nil {
		t.Error("custom CA file accepted on a plaintext connection")
	}
}

const fixtureServiceToken = "fixture-content-tls-service-token"

func tlsCatalogConfig(t testing.TB, interceptor grpc.UnaryServerInterceptor) config.Content {
	t.Helper()
	fixture := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(fixture.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(fixture.TLS)), grpc.UnaryInterceptor(interceptor))
	pb.RegisterContentServiceServer(server, tlsCatalog{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}

	return config.Content{
		Address: listener.Addr().String(), ServiceToken: fixtureServiceToken, CAFile: caFile, Timeout: 2 * time.Second,
	}
}
