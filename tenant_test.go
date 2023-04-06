package schedder_test

import (
	"bytes"
	"encoding/json"
	"io"
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

func TestAddTenantMember(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenant_name)

	other_email := "other@example.com"
	other_password := "some_password"
	otherAccountID := api.registerUserByEmail(other_email, other_password)

	var request schedder.AddTenantMemberRequest
	request.AccountID = otherAccountID
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/tenants/" + tenantID.String() + "/members", &b)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	t.Log(response.Error)

	expect(t, http.StatusOK, resp.StatusCode)
}

func TestGetTenantMembers(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"

	accountID := api.registerUserByEmail(email, password)
	token := api.generateToken(email, password)
	api.forceBusiness(email, true)
	tenantID := api.createTenant(token, tenant_name)

	other_email := "other@example.com"
	other_password := "some_password"
	otherAccountID := api.registerUserByEmail(other_email, other_password)

	r := httptest.NewRequest("GET", "/tenants/" + tenantID.String() + "/members", nil)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.GetTenantMembersResponse
	t.Log(resp.Status)
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	// there should be 2 members in the tenant (the manager and the other one)
	members := 0
	for _, mr := range response.Members {
		if mr.AccountID == accountID  || mr.AccountID == otherAccountID {
			members++
		}
		t.Logf("member email:%s uuid:%s", mr.Email, mr.AccountID)
	}

	expect(t, 2, len(response.Members))
	expect(t, 2, members)
}
