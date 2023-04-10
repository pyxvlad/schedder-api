package schedder_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"gitlab.com/vlad.anghel/schedder-api"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

var conn *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error

	pgURI := schedder.RequiredEnv("SCHEDDER_TEST_POSTGRES", "postgres://test_user@localhost/schedder_test")
	stdDB, err := sql.Open("pgx", pgURI)
	if err != nil {
		panic(err)
	}
	database.ResetDB(stdDB)
	database.MigrateDB(stdDB)
	if err = stdDB.Close(); err != nil {
		panic(err)
	}

	conn, err = pgxpool.Connect(context.Background(), pgURI)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func expect[T comparable](t *testing.T, expected T, got T) {
	if expected != got {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			panic("couldn't get caller")
		}

		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		file = strings.TrimPrefix(file, wd+"/")

		t.Fatalf("%s:%d: expected %#v, got %#v\n", file, line, expected, got)
	}
}

func unexpect[T comparable](t *testing.T, unexpected T, got T) {
	if unexpected == got {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			panic("couldn't get caller")
		}

		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		file = strings.TrimPrefix(file, wd+"/")

		t.Fatalf("%s:%d: unexpected %#v, got %#v\n", file, line, unexpected, got)
	}
}

type TestCodeStore map[string]string

func (cs TestCodeStore) SendVerification(id string, code string) error {
	cs[id] = code
	return nil
}

type ApiTX struct {
	*schedder.API
	tx pgx.Tx
	t  *testing.T
	codes TestCodeStore
}

func BeginTx(t *testing.T) ApiTX {
	t.Helper()
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatalf("testing: BeginTx: %e", err)
	}

	var api ApiTX
	api.codes = make(map[string]string)
	api.API = schedder.New(tx, api.codes, api.codes)
	api.tx = tx
	api.t = t
	return api
}

func (a *ApiTX) Rollback() {
	err := a.tx.Rollback(context.Background())
	if err != nil {
		a.t.Fatalf("testing: RollbackTx: %e", err)
	}
}

func (a *ApiTX) registerUserByEmail(email string, password string) uuid.UUID {
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader("{\"email\": \""+email+"\", \"password\": \""+password+"\"}"))
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	data := schedder.PostAccountResponse{}
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}
	expect(a.t, "", data.Error)

	return data.AccountID
}

func (a *ApiTX) registerUserByPhone(phone string, password string) uuid.UUID {
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader("{\"phone\": \""+phone+"\", \"password\": \""+password+"\"}"))
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	var data schedder.PostAccountResponse
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data.Error)
	return data.AccountID
}

func (a *ApiTX) activateUserByEmail(email string) {
	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Email = email
	request.Code = a.codes[email]

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		a.t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		a.t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK || response.Error != "" {
		a.t.Fatalf("Expected %d without error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}

	expect(a.t, email, response.Email)
}

func (a *ApiTX) activateUserByPhone(phone string) {
	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Phone = phone
	request.Code = a.codes[phone]

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		a.t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		a.t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK || response.Error != "" {
		a.t.Fatalf("Expected %d without error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}

	expect(a.t, phone, response.Phone)
}

func (a *ApiTX) generateToken(email string, password string) (token string) {
	req_data := schedder.GenerateTokenRequest{Email: email, Password: password, Device: "schedder testing"}

	var b bytes.Buffer

	err := json.NewEncoder(&b).Encode(req_data)
	if err != nil {
		a.t.Fatalf("generate_token: couldn't generate json")
	}

	req := httptest.NewRequest(http.MethodPost, "/accounts/self/sessions", &b)

	req.RemoteAddr = "127.0.0.1"

	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()
	expect(a.t, http.StatusCreated, resp.StatusCode)

	var data schedder.GenerateTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data.Error)

	token = data.Token
	return
}

func (a *ApiTX) getSessions(token string) (session_ids []uuid.UUID) {
	req := httptest.NewRequest(http.MethodGet, "/accounts/self/sessions", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.GetSessionsForAccountResponse
	json.NewDecoder(resp.Body).Decode(&response)

	session_ids = make([]uuid.UUID, 0)
	for _, s := range response.Sessions {
		session_ids = append(session_ids, s.SessionID)
	}
	return session_ids
}

func (a *ApiTX) findAccountByEmail(email string) uuid.UUID {
	account, err := a.GetDB().FindAccountByEmail(context.Background(), sql.NullString{String: email, Valid: true})
	if err != nil {
		panic(err)
	}
	return account.AccountID
}

func (a *ApiTX) forceAdmin(email string, admin bool) {
	db := a.GetDB()
	accountID := a.findAccountByEmail(email)
	err := db.SetAdminForAccount(context.Background(), database.SetAdminForAccountParams{AccountID: accountID, IsAdmin: admin})
	if err != nil {
		panic(err)
	}
}

func (a *ApiTX) forceBusiness(email string, business bool) {
	db := a.GetDB()
	accountID := a.findAccountByEmail(email)
	err := db.SetBusinessForAccount(context.Background(), database.SetBusinessForAccountParams{AccountID: accountID, IsBusiness: business})
	if err != nil {
		panic(err)
	}
}

func (a *ApiTX) createTenant(token string, tenant_name string) uuid.UUID {

	var request schedder.CreateTenantRequest

	request.Name = tenant_name

	buff := bytes.Buffer{}
	json.NewEncoder(&buff).Encode(request)

	r := httptest.NewRequest(http.MethodPost, "/tenants", &buff)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+token)

	a.ServeHTTP(w, r)

	resp := w.Result()
	expect(a.t, http.StatusCreated, resp.StatusCode)
	var response schedder.CreateTenantResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		a.t.Fatal(err)
	}
	return response.TenantID
}

func (a *ApiTX) createTenantAndAccount(email string, password string, tenant_name string) uuid.UUID {
	a.registerUserByEmail(email, password)
	a.activateUserByEmail(email)
	a.forceBusiness(email, true)
	token := a.generateToken(email, password)
	return a.createTenant(token, tenant_name)
}

func(a *ApiTX) addTenantMember(managerToken string, tenantID uuid.UUID, accountID uuid.UUID) {
	var request schedder.AddTenantMemberRequest
	request.AccountID = accountID
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		a.t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/tenants/" + tenantID.String() + "/members", &b)
	r.Header.Add("Authorization", "Bearer "+managerToken)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}

	a.t.Log(response.Error)

	expect(a.t, http.StatusOK, resp.StatusCode)
}

func TestWithInvalidJson(t *testing.T) {
	testdata := [][]string{
		{http.MethodPost, "/accounts"},
		{http.MethodPost, "/accounts/self/sessions"},
	}

	for _, v := range testdata {
		t.Run("TestWithInvalidJson: "+v[0]+" "+v[1], func(t *testing.T) {
			a := BeginTx(t)
			defer a.Rollback()

			method := v[0]
			endpoint := v[1]

			req := httptest.NewRequest(method, endpoint, strings.NewReader("}totally-not-json{"))
			w := httptest.NewRecorder()

			a.ServeHTTP(w, req)

			resp := w.Result()
			data := make(map[string]string)

			err := json.NewDecoder(resp.Body).Decode(&data)
			if err != nil {
				a.t.Fatal(err)
			}

			expect(a.t, "invalid json", data["error"])
		})
	}
}
