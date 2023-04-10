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
	api.activateUserByEmail(email)
	api.forceBusiness(email, true)

	var request schedder.CreateTenantRequest

	request.Name = "Zâna Măseluță"

	buff := bytes.Buffer{}
	json.NewEncoder(&buff).Encode(request)

	r := httptest.NewRequest(http.MethodPost, "/tenants", &buff)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))

	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusCreated, resp.StatusCode)
}


func TestCreateTenantWithShortName(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	api.forceBusiness(email, true)

	var request schedder.CreateTenantRequest

	request.Name = "Măseluț"

	buff := bytes.Buffer{}
	json.NewEncoder(&buff).Encode(request)

	r := httptest.NewRequest(http.MethodPost, "/tenants", &buff)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "invalid name", response.Error)
}

func TestGetTenants(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"

	api.createTenantAndAccount(email, password, tenant_name)

	r := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusOK, resp.StatusCode)

	var data schedder.GetTenantsResponse

	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", data.Error)

	found := false

	for _, tenant := range data.Tenants {
		if tenant.Name == tenant_name {
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
	api.activateUserByEmail(other_email)

	var request schedder.AddTenantMemberRequest
	request.AccountID = otherAccountID
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/tenants/" + tenantID.String() + "/members", &b)
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

func TestAddTenantMemberWhenAlreadyAMember(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"
	tenantID := api.createTenantAndAccount(email, password, tenant_name)
	token := api.generateToken(email, password)

	other_email := "other@example.com"
	other_password := "some_password"
	otherAccountID := api.registerUserByEmail(other_email, other_password)
	api.activateUserByEmail(other_email)

	api.addTenantMember(token, tenantID, otherAccountID)

	var request schedder.AddTenantMemberRequest
	request.AccountID = otherAccountID
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/tenants/" + tenantID.String() + "/members", &b)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	if response.Error != "already member" || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Wanted 'already member' and 400 Bad Request, got %s and %s", response.Error, resp.Status)
	}
}


func TestGetTenantMembers(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	email := "tester@example.com"
	password := "hackmenow"
	tenant_name := "Zâna Măseluță"

	// this account represents the manager of the tenant
	accountID := api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)
	api.forceBusiness(email, true)
	tenantID := api.createTenant(token, tenant_name)

	// this account represents an added member
	other_phone := "+40743123123"
	other_password := "some_password"
	otherAccountID := api.registerUserByPhone(other_phone, other_password)
	api.activateUserByPhone(other_phone)

	api.addTenantMember(token, tenantID, otherAccountID)

	// this account is supposed to NOT be part of the member list
	another_email := "another@example.com"
	another_password := "some_password"
	_ = api.registerUserByEmail(another_email, another_password)
	api.activateUserByEmail(another_email)


	r := httptest.NewRequest(http.MethodGet, "/tenants/" + tenantID.String() + "/members", nil)
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
		} else {
			t.Fatalf("unexpected tenant member email:%s uuid:%s", mr.Email, mr.AccountID)
		}
		t.Logf("member email:%s uuid:%s", mr.Email, mr.AccountID)
	}

	expect(t, 2, len(response.Members))
	expect(t, 2, members)
}
