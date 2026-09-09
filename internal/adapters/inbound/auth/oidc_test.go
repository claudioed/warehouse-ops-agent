package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_VerifiesDiscoveredIssuerJWKSAndScopes(t *testing.T) {
	issuer, key := newOIDCTestIssuer(t)
	middleware, err := New(context.Background(), issuer.URL, "warehouse-console")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	protected := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	for _, tc := range []struct {
		name, method, token string
		want                int
	}{
		{"valid read token permits GET", http.MethodGet, key.token(t, issuer.URL, "warehouse-console", time.Now().Add(time.Hour), "warehouse-ops-agent.read"), http.StatusNoContent},
		{"write scope permits mutation", http.MethodPost, key.token(t, issuer.URL, "warehouse-console", time.Now().Add(time.Hour), "warehouse-ops-agent.write"), http.StatusNoContent},
		{"expired token rejected", http.MethodGet, key.token(t, issuer.URL, "warehouse-console", time.Now().Add(-time.Hour), "warehouse-ops-agent.read"), http.StatusUnauthorized},
		{"wrong issuer rejected", http.MethodGet, key.token(t, issuer.URL+"/wrong", "warehouse-console", time.Now().Add(time.Hour), "warehouse-ops-agent.read"), http.StatusUnauthorized},
		{"wrong audience rejected", http.MethodGet, key.token(t, issuer.URL, "another-client", time.Now().Add(time.Hour), "warehouse-ops-agent.read"), http.StatusUnauthorized},
		{"missing scope rejected", http.MethodGet, key.token(t, issuer.URL, "warehouse-console", time.Now().Add(time.Hour), "profile"), http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://service.test/resource", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusUnauthorized && !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer") {
				t.Fatal("401 missing RFC 6750 Bearer challenge")
			}
			if tc.want != http.StatusNoContent && rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}

type issuerKey struct {
	private *rsa.PrivateKey
	kid     string
}

func newOIDCTestIssuer(t *testing.T) (*httptest.Server, issuerKey) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key := issuerKey{private: private, kid: "test-key"}
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]string{"issuer": issuer.URL, "jwks_uri": issuer.URL + "/keys"})
		case "/keys":
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": key.kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(private.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(private.PublicKey.E)).Bytes())}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(issuer.Close)
	return issuer, key
}
func (k issuerKey) token(t *testing.T, issuer, audience string, expiry time.Time, scope string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": k.kid})
	claims, _ := json.Marshal(map[string]any{"iss": issuer, "aud": audience, "exp": expiry.Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "scope": scope})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.private, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}
