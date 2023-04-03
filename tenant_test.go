package schedder_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestCreateTenant(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.forceBusiness(email, true)

	var request schedder.CreateTenantRequest

	request.Name = "Zâna Măseluță"

	buff := bytes.Buffer{}
	json.NewEncoder(&buff).Encode(request)

	r := httptest.NewRequest("POST", "/tenants", &buff)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))

	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusCreated, resp.StatusCode)
}

func TestGetTenants(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"

	api.createTenantAndAccount(email, password, tenant_name)

	r := httptest.NewRequest("GET", "/tenants", nil)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusOK, resp.StatusCode)

	var data schedder.GetTenantsResponse
	json.NewDecoder(resp.Body).Decode(&data)

	expect(t, "", data.Error)

	found := false
	for _, tenant := range data.Tenants {
		if tenant.TenantName == tenant_name {
			found = true
		}
	}
	expect(t, true, found)
}
