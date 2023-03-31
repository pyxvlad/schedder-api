// Package schedder implements the backend API for the schedder project.
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
	_ "github.com/jackc/pgx/v4/stdlib" // enable the stdlib adapter of pgx, used for goose migrations
	"gitlab.com/vlad.anghel/schedder-api/database"
)

// CtxKey is used as the key for context values
type CtxKey int

const (
	CtxSessionID = CtxKey(1)
	CtxAccountID = CtxKey(2)
	CtxJSON      = CtxKey(3)
)

// Struct keeping track of all the states, pretty much a singleton
type API struct {
	db   *database.Queries
	dbtx database.DBTX
	mux  *chi.Mux
}

type Response struct {
	Error string `json:"error,omitempty"`
}

func New(conn database.DBTX) *API {
	api := new(API)
	api.dbtx = conn
	api.db = database.New(api.dbtx)
	api.mux = chi.NewRouter()
	api.mux.Route("/accounts", func(r chi.Router) {
		r.With(WithJSON[PostAccountRequest]).Post("/", api.PostAccount)
		r.Route("/self", func(r chi.Router) {
			r.Route("/sessions", func(r chi.Router) {
				r.With(WithJSON[GenerateTokenRequest]).Post("/", api.GenerateToken)
				r.With(api.AuthenticatedEndpoint).Get("/", api.GetSessionsForAccount)
				r.Route("/{sessionID}", func(r chi.Router) {
					r.Use(api.AuthenticatedEndpoint)
					r.Use(api.WithSessionID)
					r.Delete("/", api.RevokeSession)
				})
			})
		})
		//r.Use(api.AuthenticatedEndpoint)
		//r.Get("/sessions", api.GetSessionsForAccount)
	})

	api.mux.Route("/tenants", func(r chi.Router) {
		r.With(WithJSON[CreateTenantRequest]).With(api.AuthenticatedEndpoint).Post("/", api.CreateTenant)
	})


	//b := bytes.Buffer{}

	//var f func(pattern string, r chi.Routes)

	//f = func(pattern string, r chi.Routes) {
	//	for _, route := range r.Routes() {
	//		pat := strings.TrimSuffix(route.Pattern, "/*")
	//		pat = pattern + pat
	//		if route.SubRoutes == nil {
	//			for k, v := range route.Handlers {
	//				fpn := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()
	//				splits := strings.Split(fpn, ".")
	//				fpn = splits[len(splits)-1]
	//				fpn = strings.TrimSuffix(fpn, "-fm")
	//				request := fmt.Sprintf("%s %s %s\n", k, fpn, pat)
	//				b.WriteString(request)
	//			}
	//		} else {
	//			sub := route.SubRoutes
	//			f(pat, sub)
	//		}
	//	}
	//}

	//f("", api.mux)

	//ioutil.WriteFile("/tmp/routes.txt", b.Bytes(), 0777)

	return api
}

func (a * API) GetMux() *chi.Mux{
	return a.mux
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
	postgresURI := RequiredEnv("SCHEDDER_POSTGRES", "postgres://user@localhost/schedder_db")
	stdDB, err := sql.Open("pgx", postgresURI)
	if err != nil {
		panic(err)
	}
	database.MigrateDB(stdDB)
	if err = stdDB.Close(); err != nil {
		panic(err)
	}

	conn, err := pgx.Connect(context.Background(), postgresURI)
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

func jsonError(w http.ResponseWriter, statusCode int, message string) {
	jsonResp(w, statusCode, Response{Error: message})
}

func jsonResp[T any](w http.ResponseWriter, statusCode int, response T) {
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (a *API) AuthenticatedEndpoint(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		parts := strings.Split(auth, " ")
		if parts[0] != "Bearer" {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		tokenString := parts[1]
		token, err := base64.RawStdEncoding.DecodeString(tokenString)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		accountID, err := a.db.GetSessionAccount(r.Context(), token)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), CtxAccountID, accountID))
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func (a *API) WithSessionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionString := chi.URLParam(r, "sessionID")
		if sessionString == "" {
			jsonError(w, http.StatusNotFound, "invalid session")
			return
		}

		sessionID, err := uuid.Parse(sessionString)
		if err != nil {
			jsonError(w, http.StatusNotFound, "invalid session")
			return
		}

		ctx := context.WithValue(r.Context(), CtxSessionID, sessionID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithJSON[T any](next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		request := new(T)
		err := decoder.Decode(request)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid json")
			return
		}
		ctx := context.WithValue(r.Context(), CtxJSON, request)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
