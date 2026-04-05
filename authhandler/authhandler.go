package authhandler

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"github.com/golang-jwt/jwt/v5"
)

var secrets struct {
	JWKSURL string
}

// AuthData contains authenticated user information extracted from the JWT.
type AuthData struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Authenticate validates JWTs from any OIDC-compliant provider (Supabase, Clerk, Keycloak).
//
//encore:authhandler
func Authenticate(ctx context.Context, token string) (auth.UID, *AuthData, error) {
	keys, err := getCachedJWKS()
	if err != nil {
		return "", nil, fmt.Errorf("fetch JWKS: %w", err)
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key ID: %s", kid)
		}

		switch t.Method.(type) {
		case *jwt.SigningMethodRSA:
			if pk, ok := key.(*rsa.PublicKey); ok {
				return pk, nil
			}
			return nil, fmt.Errorf("key %s is not RSA", kid)
		case *jwt.SigningMethodECDSA:
			if pk, ok := key.(*ecdsa.PublicKey); ok {
				return pk, nil
			}
			return nil, fmt.Errorf("key %s is not ECDSA", kid)
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
	}, jwt.WithValidMethods([]string{"RS256", "ES256", "ES384", "ES512"}))
	if err != nil {
		return "", nil, &errs.Error{Code: errs.Unauthenticated, Message: "invalid token"}
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return "", nil, &errs.Error{Code: errs.Unauthenticated, Message: "invalid claims"}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", nil, &errs.Error{Code: errs.Unauthenticated, Message: "missing sub claim"}
	}

	email, _ := claims["email"].(string)

	var roles []string
	if r, ok := claims["role"].(string); ok && r != "" {
		roles = []string{r}
	}
	if r, ok := claims["app_metadata"].(map[string]interface{}); ok {
		if role, ok := r["role"].(string); ok {
			roles = append(roles, role)
		}
	}

	return auth.UID(sub), &AuthData{
		Email: email,
		Roles: roles,
	}, nil
}

// JWKS cache with TTL
var (
	jwksMu    sync.Mutex
	jwksCache map[string]crypto.PublicKey
	jwksTime  time.Time
	jwksTTL   = 10 * time.Minute
)

func getCachedJWKS() (map[string]crypto.PublicKey, error) {
	jwksMu.Lock()
	defer jwksMu.Unlock()
	if jwksCache != nil && time.Since(jwksTime) < jwksTTL {
		return jwksCache, nil
	}
	keys, err := fetchJWKS(secrets.JWKSURL)
	if err != nil {
		return nil, err
	}
	jwksCache = keys
	jwksTime = time.Now()
	return keys, nil
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	// RSA fields
	N string `json:"n"`
	E string `json:"e"`
	// EC fields
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func fetchJWKS(url string) (map[string]crypto.PublicKey, error) {
	url = strings.TrimPrefix(url, "Bearer ")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		switch k.KTY {
		case "RSA":
			pk, err := parseRSAKey(k)
			if err != nil {
				return nil, fmt.Errorf("parse RSA key kid=%s: %w", k.KID, err)
			}
			keys[k.KID] = pk
		case "EC":
			pk, err := parseECKey(k)
			if err != nil {
				return nil, fmt.Errorf("parse EC key kid=%s: %w", k.KID, err)
			}
			keys[k.KID] = pk
		}
	}

	return keys, nil
}

func parseRSAKey(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func parseECKey(k jwkKey) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.CRV {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", k.CRV)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}
