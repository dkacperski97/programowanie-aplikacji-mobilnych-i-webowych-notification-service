package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	models "github.com/dkacperski97/programowanie-aplikacji-mobilnych-i-webowych-models"
	"github.com/gorilla/mux"
)

type (
	tokenKey string
)

const key tokenKey = "ParsedToken"

func GetClaims(ctx context.Context) (*models.UserClaims, bool) {
	claims, ok := ctx.Value(key).(*models.UserClaims)
	return claims, ok
}

func JwtHandler(secret []byte, isTokenRequired bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var tokenHeader string
			if req.Header.Get("Authorization") != "" {
				tokenHeader = req.Header.Get("Authorization")
			} else {
				tokenHeader = req.Header.Get("Sec-WebSocket-Protocol")
			}
			tokenString := strings.Replace(tokenHeader, "Bearer ", "", 1)
			token, err := jwt.ParseWithClaims(tokenString, &models.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}

				return secret, nil
			})

			if err == nil {
				if claims, ok := token.Claims.(*models.UserClaims); ok && token.Valid {
					ctx := context.WithValue(req.Context(), key, claims)
					next.ServeHTTP(w, req.WithContext(ctx))
					return
				}
			}

			if !isTokenRequired || req.Method == http.MethodOptions {
				next.ServeHTTP(w, req)
				return
			}

			http.Error(w, "Incorrect authorization header", http.StatusBadRequest)
		})
	}
}

func HeadersHandler() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
			origin := req.Header.Get("Origin")
			if origin == os.Getenv("APP_1_ORIGIN") || origin == os.Getenv("APP_2_ORIGIN") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			if req.Method == http.MethodOptions && w.Header().Get("Access-Control-Allow-Methods") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
