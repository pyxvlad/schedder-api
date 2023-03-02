package schedder

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type API struct {
	db   *database.Queries
	dbtx database.DBTX
	mux  *chi.Mux
}

func New(conn database.DBTX) *API {
	api := new(API)
	api.dbtx = conn
	api.db = database.New(api.dbtx)
	api.mux = chi.NewRouter()
	api.mux.Route("/accounts", func(r chi.Router) {
		r.Post("/", api.PostAccount)
		r.Route("/self", func(r chi.Router) {
			r.Use(api.AuthenticatedEndpoint)
			r.Get("/sessions", api.GetSessionsForAccount)
		})
	})
	api.mux.Route("/sessions", func(r chi.Router) {
		r.Post("/", api.GenerateToken)
		r.Route("/{session_id}", func(r chi.Router) {
			r.Use(api.AuthenticatedEndpoint)
			r.Use(api.WithSessionId)
			r.Delete("/", api.RevokeSession)
		})
	})
	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func RequiredEnv(name string, example string) string {
	env, found := os.LookupEnv(name)
	if !found {
		panic("env var " + name + " not defined, example: " + name + "=" + example)
	}
	return env
}

func Run() {
	pg_uri := RequiredEnv("SCHEDDER_POSTGRES", "postgres://user@localhost/schedder_db")
	std_db, err := sql.Open("pgx", pg_uri)
	if err != nil {
		panic(err)
	}
	database.MigrateDB(std_db)
	if err := std_db.Close(); err != nil {
		panic(err)
	}

	conn, err := pgx.Connect(context.Background(), pg_uri)
	if err != nil {
		panic(err)
	}

	api := New(conn)

	if err := http.ListenAndServe(":2023", api); err != nil {
		panic(err)
	}
	if err := conn.Close(context.Background()); err != nil {
		panic(err)
	}
}

type ResponseError struct {
	Error string `json:"error,omitempty"`
}

func json_error(w http.ResponseWriter, statusCode int, message string) error {
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	return encoder.Encode(ResponseError{message})
}

func (a *API) AuthenticatedEndpoint(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		parts := strings.Split(auth, " ")
		if parts[0] != "Bearer" {
			json_error(w, http.StatusUnauthorized, "invalid token")
			return
		}

		token_string := parts[1]
		token, err := base64.RawStdEncoding.DecodeString(token_string)
		if err != nil {
			json_error(w, http.StatusUnauthorized, "invalid token")
			return
		}
		account_id, err := a.db.GetSessionAccount(r.Context(), token)
		if err != nil {
			json_error(w, http.StatusUnauthorized, "invalid token")
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), "account_id", account_id))
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func (a *API) WithSessionId(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session_string := chi.URLParam(r, "session_id")
		if session_string == "" {
			json_error(w, http.StatusNotFound, "invalid session")
			return
		}

		session_id, err := uuid.Parse(session_string)
		if err != nil {
			json_error(w, http.StatusNotFound, "invalid session")
			return
		}

		ctx := context.WithValue(r.Context(), "session_id", session_id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
