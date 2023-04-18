package schedder_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api"
)

func TestAdminEndpointWithInvalidAccountID(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	handler := api.AdminEndpoint(nil)
	r := httptest.NewRequest("", "/", nil)
	w := httptest.NewRecorder()
	ctx := context.WithValue(
		r.Context(), schedder.CtxAuthenticatedID, uuid.New(),
	)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusInternalServerError, resp.StatusCode)
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "invalid state: invalid user is authenticated", response.Error)
}

func TestWithBadUUIDs(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	type Middleware func(http.Handler) http.Handler

	middlewares := map[string]Middleware{
		"accountID": api.WithAccountID,
		"photoID": api.WithPhotoID,
		"tenantID": api.WithTenantID,
		"sessionID": api.WithSessionID,
	}

	for param, mid := range middlewares {
		endpoint := fmt.Sprintf("/{%s}", param)
		mux := chi.NewMux()
		mux.With(mid).Get(endpoint, nil)
		r := httptest.NewRequest("", "/totally-not-an-uuid", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		resp := w.Result()
		expect(t, http.StatusNotFound, resp.StatusCode)

		message := fmt.Sprintf("invalid %s",strings.TrimSuffix(param, "ID"))

		var response schedder.Response
		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}
		expect(t, message, response.Error)
	}


}
