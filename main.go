// Package schedder implements the backend API for the schedder project.
package schedder

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v4"

	// Enable the stdlib adapter of pgx, used for goose migrations.
	_ "github.com/jackc/pgx/v4/stdlib"

	"gitlab.com/vlad.anghel/schedder-api/database"
)

// API is keeping track of all the states, pretty much a singleton.
type API struct {
	db            *database.Queries
	txlike        database.TxLike
	mux           *chi.Mux
	emailVerifier Verifier
	phoneVerifier Verifier
	photosPath    string
}

func (a *API) PhotosPath() string {
	return a.photosPath
}

// Response represents the base response. All other *Response types embed this
// type.
type Response struct {
	// Error represents the error, if one occured, otherwise the field will be
	// missing. If this field is set then ANY other fields should be IGNORED.
	Error string `json:"error,omitempty"`
}

// New creates a new API object.
func New(
	txlike database.TxLike,
	emailVerifier, phoneVerifier Verifier,
	photosPath string,
) *API {
	api := new(API)
	api.txlike = txlike
	api.db = database.New(api.txlike)
	api.mux = chi.NewRouter()
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
		r.With(WithJSON[AccountCreationRequest]).Post("/", api.CreateAccount)
		r.Route("/self", func(r chi.Router) {
			r.With(WithJSON[VerifyCodeRequest]).Post("/verify", api.VerifyCode)
			r.Group(func(r chi.Router) {
				r.Use(api.AuthenticatedEndpoint)
				r.Post("/photo", api.SetProfilePhoto)
				r.Get("/photo", api.DownloadProfilePhoto)
				r.Delete("/photo", api.DeleteProfilePhoto)
			})

			r.Route("/sessions", func(r chi.Router) {
				r.With(WithJSON[TokenGenerationRequest]).Post(
					"/", api.GenerateToken,
				)
				r.With(api.AuthenticatedEndpoint).Get(
					"/", api.SessionsForAccount,
				)
				r.Route("/{sessionID}", func(r chi.Router) {
					r.Use(api.AuthenticatedEndpoint)
					r.Use(api.WithSessionID)
					r.Delete("/", api.RevokeSession)
				})
			})
		})
		r.With(api.AuthenticatedEndpoint, api.AdminEndpoint).Get(
			"/by-email/{email}", api.AccountByEmailAsAdmin,
		)
		r.Route("/{accountID}", func(r chi.Router) {
			r.Use(
				api.WithAccountID,
				api.AuthenticatedEndpoint,
				api.AdminEndpoint,
			)
			r.With(WithJSON[AdminSettingRequest]).Post("/admin", api.SetAdmin)
			r.With(WithJSON[BusinessSettingRequest]).Post(
				"/business", api.SetBusiness,
			)
		})
	})

	api.mux.Route("/tenants", func(r chi.Router) {
		r.With(WithJSON[CreateTenantRequest], api.AuthenticatedEndpoint).Post(
			"/", api.CreateTenant,
		)
		r.Get("/", api.Tenants)
		r.Route("/{tenantID}", func(r chi.Router) {
			r.Use(api.WithTenantID)
			r.Group(func(r chi.Router) {
				r.Use(
					api.AuthenticatedEndpoint,
					api.TenantManagerEndpoint,
				)
				r.With(WithJSON[AddTenantMemberRequest]).Post(
					"/members", api.AddTenantMember,
				)
				r.Get("/members", api.TenantMembers)
				r.Post("/photos", api.AddTenantPhoto)
				r.With(api.WithPhotoID).Delete(
					"/photos/by-id/{photoID}", api.DeleteTenantPhoto,
				)
			})
			r.Get("/photos", api.ListTenantPhotos)
			r.With(api.WithPhotoID).Get(
				"/photos/by-id/{photoID}", api.DownloadTenantPhoto,
			)
		})
	})
	api.emailVerifier = emailVerifier
	api.phoneVerifier = phoneVerifier

	api.photosPath = photosPath
	os.MkdirAll(api.photosPath, 0777)

	return api
}

// ServeHTTP serves the API.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// DB returns the internal database.Queries object.
func (a *API) DB() *database.Queries {
	return a.db
}

func (a *API) Mux() *chi.Mux {
	return a.mux
}

// RequiredEnv checks for the environment variable "name". If it doesn't exist,
// then it panics with a message requesting the user to define it in the form
// of name=example.
func RequiredEnv(name, example string) string {
	env, found := os.LookupEnv(name)
	if !found {
		msg := fmt.Sprintf(
			"env var %s not defined, example: %s=%s", name, name, example,
		)
		panic(msg)
	}
	return env
}

// Run connects to Postgres, does migrations and then serves the API. Kind of
// like a main function for this whole module.
func Run() {
	postgresURI := RequiredEnv(
		"SCHEDDER_POSTGRES", "postgres://user@localhost/schedder_db",
	)

	photosPath := RequiredEnv("SCHEDDER_PHOTOS", "./data/photos")
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

	api := New(conn, &emailVerifier, &phoneVerifier, photosPath)

	server := &http.Server{
		Addr:              ":2023",
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           api,
	}

	if err := server.ListenAndServe(); err != nil {
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}
