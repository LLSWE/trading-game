package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type ContextKey string

const (
	ProviderIDKey ContextKey = "providerId"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "invalid authorization header format, expected Bearer <token>")
			return
		}

		tokenString := parts[1]

		providerID, err := extractProviderIDFromToken(tokenString)
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("invalid or expired token: %v", err))
			return
		}

		ctx := context.WithValue(r.Context(), ProviderIDKey, providerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractProviderIDFromToken(tokenString string) (string, error) {
	segments := strings.Split(tokenString, ".")
	if len(segments) != 3 {
		return "", errors.New("invalid jwt structure")
	}

	payloadBytes, err := base64UrlDecode(segments[1])
	if err != nil {
		return "", errors.New("failed to decode jwt payload")
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", errors.New("failed to parse jwt claims")
	}

	if clientID, ok := claims["clientId"].(string); ok && clientID != "" {
		return clientID, nil
	}
	if azp, ok := claims["azp"].(string); ok && azp != "" {
		return azp, nil
	}
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub, nil
	}

	return "", errors.New("provider identity (clientId/sub) not found in token claims")
}

func base64UrlDecode(s string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(s)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
