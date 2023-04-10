// Package schedder implements the backend API for the schedder project.
package schedder

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"

	// Enable the stdlib adapter of pgx, used for goose migrations.
	_ "github.com/jackc/pgx/v4/stdlib"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

// CtxKey is used as the key for context values
type CtxKey int

const (
	CtxSessionID       = CtxKey(1)
	CtxAccountID       = CtxKey(2)
	CtxAuthenticatedID = CtxKey(3)
	CtxTenantID        = CtxKey(4)
	CtxJSON            = CtxKey(5)
)

// Struct keeping track of all the states, pretty much a singleton
type API struct {
	db            *database.Queries
	txlike        database.TxLike
	mux           *chi.Mux
	emailVerifier Verifier
	phoneVerifier Verifier
}

type Response struct {
	Error string `json:"error,omitempty"`
}

func New(
	txlike database.TxLike, emailVerifier Verifier, phoneVerifier Verifier,
) *API {
	api := new(API)
	api.txlike = txlike
	api.db = database.New(api.txlike)
	api.mux = chi.NewRouter()
	// api.mux.Use(WithCORS)
	api.mux.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"https://*", "http://*"},
		AllowedMethods: []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD",
		},
		AllowedHeaders: []string{
			"Accept", "Authorization", "Content-Type", "X-CSRF-Token",
		},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of browsers
	}))

	api.mux.Route("/accounts", func(r chi.Router) {
		r.With(WithJSON[PostAccountRequest]).Post("/", api.PostAccount)
		r.Route("/self", func(r chi.Router) {
			r.With(WithJSON[VerifyCodeRequest]).Post("/verify", api.VerifyCode)
			r.Route("/sessions", func(r chi.Router) {
				r.With(WithJSON[GenerateTokenRequest]).Post(
					"/", api.GenerateToken,
				)
				r.With(api.AuthenticatedEndpoint).Get(
					"/", api.GetSessionsForAccount,
				)
				r.Route("/{sessionID}", func(r chi.Router) {
					r.Use(api.AuthenticatedEndpoint)
					r.Use(api.WithSessionID)
					r.Delete("/", api.RevokeSession)
				})
			})
		})
		r.With(api.AuthenticatedEndpoint).With(api.AdminEndpoint).Get(
			"/by-email/{email}", api.GetAccountByEmailAsAdmin,
		)
		r.Route("/{accountID}", func(r chi.Router) {
			r.Use(
				api.WithAccountID,
				api.AuthenticatedEndpoint,
				api.AdminEndpoint,
			)
			r.With(WithJSON[SetAdminRequest]).Post("/admin", api.SetAdmin)
			r.With(WithJSON[SetBusinessRequest]).Post(
				"/business", api.SetBusiness,
			)
		})
		//r.Use(api.AuthenticatedEndpoint)
		//r.Get("/sessions", api.GetSessionsForAccount)
	})

	api.mux.Route("/tenants", func(r chi.Router) {
		r.With(WithJSON[CreateTenantRequest], api.AuthenticatedEndpoint).Post(
			"/", api.CreateTenant,
		)
		r.Get("/", api.GetTenants)
		r.Route("/{tenantID}", func(r chi.Router) {
			r.Use(
				api.WithTenantID,
				api.AuthenticatedEndpoint,
				api.TenantManagerEndpoint,
			)
			r.With(WithJSON[AddTenantMemberRequest]).Post(
				"/members", api.AddTenantMember,
			)
			r.Get("/members", api.GetTenantMembers)
		})
	})
	api.emailVerifier = emailVerifier
	api.phoneVerifier = phoneVerifier

	return api
}

func (a *API) GetMux() *chi.Mux {
	return a.mux
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *API) GetDB() *database.Queries {
	return a.db
}

func RequiredEnv(name string, example string) string {
	env, found := os.LookupEnv(name)
	if !found {
		msg := fmt.Sprintf(
			"env var %s not defined, example: %s=%s", name, name, example,
		)
		panic(msg)
	}
	return env
}

func Run() {
	postgresURI := RequiredEnv(
		"SCHEDDER_POSTGRES", "postgres://user@localhost/schedder_db",
	)
	log.Printf("INFO: connecting to Postgres using: %#v", postgresURI)
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

	emailVerifier := WriterVerifier{os.Stdout, "email"}
	phoneVerifier := WriterVerifier{os.Stdout, "phone"}

	api := New(conn, &emailVerifier, &phoneVerifier)

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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		authenticatedID, err := a.db.GetSessionAccount(r.Context(), token)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		newctx := context.WithValue(
			r.Context(), CtxAuthenticatedID, authenticatedID,
		)
		r = r.WithContext(newctx)
		next.ServeHTTP(w, r)
	})
}

func (a *API) AdminEndpoint(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
		admin, err := a.db.GetAdminForAccount(r.Context(), authenticatedID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "how?")
			return
		}
		if !admin {
			jsonError(w, http.StatusForbidden, "not admin")
			return
		}
		next.ServeHTTP(w, r)
	})
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

func (a *API) WithAccountID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountString := chi.URLParam(r, "accountID")
		if accountString == "" {
			jsonError(w, http.StatusNotFound, "invalid account")
			return
		}

		accountID, err := uuid.Parse(accountString)
		if err != nil {
			jsonError(w, http.StatusNotFound, "invalid account")
			return
		}

		ctx := context.WithValue(r.Context(), CtxAccountID, accountID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) WithTenantID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantString := chi.URLParam(r, "tenantID")
		if tenantString == "" {
			jsonError(w, http.StatusNotFound, "invalid tenant")
			return
		}

		tenantID, err := uuid.Parse(tenantString)
		if err != nil {
			jsonError(w, http.StatusNotFound, "invalid tenant")
			return
		}

		ctx := context.WithValue(r.Context(), CtxTenantID, tenantID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) TenantManagerEndpoint(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
		tenantID := r.Context().Value(CtxTenantID).(uuid.UUID)
		params := database.IsTenantManagerParams{
			TenantID: tenantID, AccountID: authenticatedID,
		}

		isManager, err := a.db.IsTenantManager(r.Context(), params)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "yes")
			return
		}
		if !isManager {
			jsonError(w, http.StatusForbidden, "not manager")
		}

		next.ServeHTTP(w, r)
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
