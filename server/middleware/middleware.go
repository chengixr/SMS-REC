package middleware

import (
	"context"
	"net/http"
	"strings"

	"sms-server/auth"
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token := strings.TrimPrefix(header, "Bearer ")
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			claims, err := auth.ParseToken(token, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "username", claims.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

