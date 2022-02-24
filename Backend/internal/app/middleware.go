package app

import (
	"context"
	"fmt"
	"forum/internal/user"
	"net/http"
	"strings"
)

type userContext struct {
	userID string
	login  string
	email  string
}

func (a *App) userIdentity(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		fmt.Println(c)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		xs := strings.SplitN(c.Value, "|", 2)
		if len(xs) != 2 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		cCode := xs[0]
		uID := xs[1]
		var u user.User
		if u, err = a.userService.CheckSession(cCode, uID); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// set context
		ctx := context.WithValue(r.Context(), "user", userContext{userID: u.ID, login: u.Login, email: u.Email})
		next(w, r.WithContext(ctx))
	})
}

func corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, *")
		if r.Method != http.MethodOptions {
			next.ServeHTTP(w, r)
		}
	})
}
