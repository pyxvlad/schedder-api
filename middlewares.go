package schedder

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

// AuthenticatedEndpoint is a middleware that ensures an user is authenticated.
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

// AdminEndpoint is a middleware that ensure the user is an admin.
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

// WithSessionID is a middleware that ensures the sessionID URL parameter is
// present and makes it available as an UUID in the context using CtxSessionID.
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

// WithAccountID is a middleware that ensures the accountID URL parameter is
// present and makes it available as an UUID in the context using CtxAccountID.
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

// WithTenantID is a middleware that ensures the tenantID URL parameter is
// present and makes it available as an UUID in the context using CtxTenantID.
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

// TenantManagerEndpoint is a middleware that ensures that the authenticated
// user is a tenant manager. NOTE: it depends on the WithTenantID middleware.
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

// WithJSON is a middleware that decodes the body of the request as T using
// json, and then puts the object in the context at CtxJSON.
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
