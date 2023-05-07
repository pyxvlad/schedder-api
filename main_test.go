package schedder_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"gitlab.com/vlad.anghel/schedder-api"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

var conn *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error

	pgURI := schedder.RequiredEnv(
		"SCHEDDER_TEST_POSTGRES",
		"postgres://test_user@localhost/schedder_test",
	)
	stdDB, err := sql.Open("pgx", pgURI)
	if err != nil {
		panic(err)
	}
	err = database.ResetDB(stdDB)
	if err != nil {
		panic(err)
	}
	database.MigrateDB(stdDB)
	if err != nil {
		panic(err)
	}
	if err = stdDB.Close(); err != nil {
		panic(err)
	}

	conn, err = pgxpool.Connect(context.Background(), pgURI)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func expect[T comparable](t *testing.T, expected, got T) {
	t.Helper()
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

func unexpect[T comparable](t *testing.T, unexpected, got T) {
	t.Helper()
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

		t.Fatalf(
			"%s:%d: unexpected %#v, got %#v\n", file, line, unexpected, got,
		)
	}
}

type TestCodeStore map[string]string

func (cs TestCodeStore) SendVerification(id, code string) error {
	cs[id] = code
	return nil
}

type APITX struct {
	*schedder.API
	tx    pgx.Tx
	t     *testing.T
	codes TestCodeStore
}

func BeginTx(t *testing.T) APITX {
	t.Helper()
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatalf("testing: BeginTx: %e", err)
	}

	photosPath := t.TempDir() + "/photos"

	var api APITX
	api.codes = make(map[string]string)
	api.API = schedder.New(tx, api.codes, api.codes, photosPath)
	api.tx = tx
	api.t = t
	return api
}

func (a *APITX) Rollback() {
	err := a.tx.Rollback(context.Background())
	if err != nil {
		a.t.Fatalf("testing: RollbackTx: %e", err)
	}
}

func (a *APITX) registerUserByEmail(email, password string) uuid.UUID {
	reader := strings.NewReader(
		"{\"email\": \"" + email + "\", \"password\": \"" + password + "\"}",
	)
	req := httptest.NewRequest(http.MethodPost, "/accounts", reader)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Logf("register_user: got status %s", resp.Status)
	}

	data := schedder.AccountCreationResponse{}
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}
	expect(a.t, "", data.Error)

	return data.AccountID
}

func (a *APITX) registerUserByPhone(phone, password string) uuid.UUID {
	reader := strings.NewReader(
		"{\"phone\": \"" + phone + "\", \"password\": \"" + password + "\"}",
	)
	req := httptest.NewRequest(http.MethodPost, "/accounts", reader)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	var data schedder.AccountCreationResponse
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data.Error)
	return data.AccountID
}

func (a *APITX) registerPasswordlessUserByPhone(phone string) uuid.UUID {
	reader := strings.NewReader("{\"phone\": \"" + phone + "\"}")
	req := httptest.NewRequest(http.MethodPost, "/accounts", reader)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.AccountCreationResponse
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data.Error)
	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}
	return data.AccountID
}

func (a *APITX) activateUserByEmail(email string) {
	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Email = email
	request.Code = a.codes[email]
	request.Device = "schedder test"

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
		a.t.Fatalf(
			"Expected %d without error, got %s with error: %v",
			http.StatusOK, resp.Status, response.Error,
		)
	}

	expect(a.t, email, response.Email)
}

func (a *APITX) activateUserByPhone(phone string) {
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
		a.t.Fatalf(
			"Expected %d without error, got %s with error %v",
			http.StatusOK, resp.Status, response.Error,
		)
	}

	expect(a.t, phone, response.Phone)
}

func (a *APITX) generateToken(email, password string) (token string) {
	request := schedder.TokenGenerationRequest{
		Email:    email,
		Password: password,
		Device:   "schedder testing",
	}

	var b bytes.Buffer

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		a.t.Fatalf("generate_token: couldn't generate json")
	}

	req := httptest.NewRequest(http.MethodPost, "/accounts/self/sessions", &b)

	req.RemoteAddr = "127.0.0.1"

	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.TokenGenerationResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data.Error)
	expect(a.t, http.StatusCreated, resp.StatusCode)

	return data.Token
}

func (a *APITX) getSessions(token string) (sessionIDs []uuid.UUID) {
	req := httptest.NewRequest(http.MethodGet, "/accounts/self/sessions", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.SessionsForAccountResponse
	json.NewDecoder(resp.Body).Decode(&response)

	sessionIDs = make([]uuid.UUID, 0)
	for _, s := range response.Sessions {
		sessionIDs = append(sessionIDs, s.SessionID)
	}
	return sessionIDs
}

func (a *APITX) findAccountByEmail(email string) uuid.UUID {
	account, err := a.DB().FindAccountByEmail(
		context.Background(), sql.NullString{String: email, Valid: true},
	)
	if err != nil {
		panic(err)
	}
	return account.AccountID
}

func (a *APITX) forceAdmin(email string, admin bool) {
	db := a.DB()
	accountID := a.findAccountByEmail(email)
	safap := database.SetAdminForAccountParams{
		AccountID: accountID,
		IsAdmin:   admin,
	}
	err := db.SetAdminForAccount(context.Background(), safap)
	if err != nil {
		panic(err)
	}
}

func (a *APITX) forceBusiness(email string, business bool) {
	db := a.DB()
	accountID := a.findAccountByEmail(email)
	sbfap := database.SetBusinessForAccountParams{
		AccountID:  accountID,
		IsBusiness: business,
	}
	err := db.SetBusinessForAccount(context.Background(), sbfap)
	if err != nil {
		panic(err)
	}
}

func (a *APITX) createTenant(token, tenantName string) uuid.UUID {
	var request schedder.CreateTenantRequest

	request.Name = tenantName

	buff := bytes.Buffer{}
	err := json.NewEncoder(&buff).Encode(request)
	if err != nil {
		a.t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/tenants", &buff)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+token)

	a.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.CreateTenantResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}
	expect(a.t, http.StatusCreated, resp.StatusCode)
	return response.TenantID
}

func (a *APITX) createTenantAndAccount(
	email, password, tenantName string,
) uuid.UUID {
	a.registerUserByEmail(email, password)
	a.activateUserByEmail(email)
	a.forceBusiness(email, true)
	token := a.generateToken(email, password)
	return a.createTenant(token, tenantName)
}

func (a *APITX) addTenantMember(
	managerToken string, tenantID, accountID uuid.UUID,
) {
	var request schedder.AddTenantMemberRequest
	request.AccountID = accountID
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		a.t.Fatal(err)
	}

	r := httptest.NewRequest(
		http.MethodPost, "/tenants/"+tenantID.String()+"/members", &b,
	)
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

func (a *APITX) addTenantPhoto(managerToken string, tenantID uuid.UUID, photo io.Reader) uuid.UUID {
	endpoint := fmt.Sprintf("/tenants/%s/photos", tenantID)
	r := httptest.NewRequest(http.MethodPost, endpoint, photo)
	r.Header.Set("Authorization", "Bearer "+managerToken)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)

	resp := w.Result()

	a.t.Log(resp.Status)

	var response schedder.AddTenantPhotoResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		a.t.Fatal(err)
	}
	expect(a.t, http.StatusCreated, resp.StatusCode)
	return response.PhotoID
}

func (a *APITX) addProfilePhoto(token string, photo io.Reader) {
	r := httptest.NewRequest(http.MethodPost, "/accounts/self/photo", photo)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}

	a.t.Logf("Error: %s", response.Error)

	expect(a.t, "", response.Error)
	expect(a.t, http.StatusOK, resp.StatusCode)
}

func (a *APITX) createService(
	token string, tenantID uuid.UUID, personnelID uuid.UUID, name string,
	price float64, duration time.Duration,
) uuid.UUID {
	request := schedder.CreateServiceRequest{
		ServiceName: name,
		Price: price,
		Duration: duration,
	}

	endpoint := fmt.Sprintf("/tenants/%s/personnel/%s/services", tenantID, personnelID)

	req, err := NewJSONRequest(http.MethodPost, endpoint, request)
	if err != nil {
		a.t.Fatal(err)
	}

	req.Header.Add("Authorization", "Bearer " + token)

	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.CreateServiceResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", response.Error)
	expect(a.t, http.StatusCreated, resp.StatusCode)
	return response.ServiceID
}

func (a *APITX) setSchedule(token string, personnelID uuid.UUID, tenantID uuid.UUID, starting time.Time, ending time.Time, weekday time.Weekday) {
	endpoint := fmt.Sprintf(
		"/tenants/%s/personnel/%s/schedule", tenantID, personnelID,
	)

	a.t.Log(endpoint)

	req, err := NewJSONRequest(
		http.MethodPost,
		endpoint,
		schedder.SetScheduleRequest{
			Weekday: weekday,
			Starting: starting,
			Ending: ending,
		},
	)
	req.Header.Add( "Authorization", "Bearer " + token)
	if err != nil {
		a.t.Fatal(err)
	}
	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		a.t.Log(resp.Status)
		var response schedder.Response
		io.Copy(os.Stdout, resp.Body)
		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			a.t.Fatal(err)
		}

		a.t.Fatalf("Result: %s: %s", resp.Status, response.Error)
	}
}

func (a * APITX) createReview(token string, tenantID uuid.UUID, message string, rating int) {
	request := schedder.CreateReviewRequest{Message: message, Rating: rating}
	endpoint := fmt.Sprintf("/tenants/%s/reviews", tenantID)
	r, err := NewJSONRequest(http.MethodPost, endpoint, request)
	r.Header.Add("Authorization", "Bearer " + token)
	if err != nil {
		a.t.Fatal(err)
	}

	w := httptest.NewRecorder()

	a.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}

	expect(a.t, "", response.Error)
	expect(a.t, http.StatusCreated, resp.StatusCode)

}

func (a *APITX) addFavourite(token string, tenantID uuid.UUID) {
	endpoint := fmt.Sprintf("/accounts/self/favourites/%s", tenantID)
	r := httptest.NewRequest(http.MethodPost, endpoint, nil)
	w := httptest.NewRecorder()

	r.Header.Add("Authorization", "Bearer " + token)

	a.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		a.t.Fatal(err)
	}

	expect(a.t, "", response.Error)
	expect(a.t, http.StatusCreated, resp.StatusCode)
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

			req := httptest.NewRequest(
				method, endpoint, strings.NewReader("}totally-not-json{"),
			)
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
