package schedder_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestRegisterWithEmail(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackmetoday"
	err := json.NewEncoder(&buffer).Encode(
		schedder.AccountCreationRequest{Email: email, Password: password},
	)

	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.AccountCreationResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	t.Logf("%s", response.Error)

	expect(t, http.StatusCreated, resp.StatusCode)
	expect(t, "", response.Error)
	expect(t, email, response.Email)
}

func TestRegisterWithShortPassword(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer
	acr := schedder.AccountCreationRequest{Email: "some@gmail.com", Password: "meow"}
	json.NewEncoder(&buffer).Encode(acr)
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.AccountCreationResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", response.Error)
	expect(t, "password too short", response.Error)
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterWithoutEmailOrPhone(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer
	json.NewEncoder(&buffer).Encode(schedder.AccountCreationRequest{Password: "hackmenow"})
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.AccountCreationResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", response.Error)
	expect(t, "expected phone or email", response.Error)
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterWithPhone(t *testing.T) {
	t.Parallel()
	testdata := [][2]string{
		{"+40723123123", "+40723123123"},
		{"0723123123", "+40723123123"},
		{"+4 0 7 2 3 1 2 3 1 2 3", "+40723123123"},
	}

	for _, v := range testdata {
		testData := new([2]string)
		copy(testData[:], v[:])
		t.Run(v[0], func(t *testing.T) {
			data := *testData
			t.Parallel()

			api := BeginTx(t)
			defer api.Rollback()

			var buffer bytes.Buffer

			phone := data[0]
			expected := data[1]
			password := "hackmenow"
			acr := schedder.AccountCreationRequest{
				Phone:    phone,
				Password: password,
			}
			err := json.NewEncoder(&buffer).Encode(acr)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			resp := w.Result()

			var response schedder.AccountCreationResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			expect(t, "", response.Error)
			expect(t, http.StatusCreated, resp.StatusCode)
			expect(t, expected, response.Phone)
		})
	}
}

func TestRegisterWithShortPhone(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+4 0712 123 1 3"
	password := "hackmenow"
	acr := schedder.AccountCreationRequest{Phone: phone, Password: password}
	err := json.NewEncoder(&buffer).Encode(acr)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	response := schedder.Response{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "phone too short/long", response.Error)
}

func TestRegisterWithInvalidEmail(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "totally.not.an.email"
	password := "hackmenow"
	acr := schedder.AccountCreationRequest{Email: email, Password: password}
	err := json.NewEncoder(&buffer).Encode(acr)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	response := schedder.Response{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "invalid email", response.Error)
}

func FuzzRegister_BadEmails(f *testing.F) {
	corpus := []string{"mail", "mail@", "@gmail.com", "john#doe"}
	for _, seed := range corpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, email string) {
		t.Logf("running for %#v", email)

		api := BeginTx(t)
		defer api.Rollback()

		var buffer bytes.Buffer
		password := "hackmetoday"
		acr := schedder.AccountCreationRequest{
			Email:    email,
			Password: password,
		}
		err := json.NewEncoder(&buffer).Encode(acr)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
		w := httptest.NewRecorder()

		api.ServeHTTP(w, req)

		resp := w.Result()

		response := schedder.Response{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		unexpect(t, "", response.Error)
	})
}

func TestRegisterWithDuplicateEmail(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()
	email := "mail@example.com"
	password := "hackmetoday"

	api.registerUserByEmail(email, password)

	var buffer bytes.Buffer

	acr := schedder.AccountCreationRequest{Email: email, Password: password}
	err := json.NewEncoder(&buffer).Encode(acr)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	r := w.Result()

	var response schedder.Response
	err = json.NewDecoder(r.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "email already used", response.Error)
}

func TestRegisterWithDuplicatePhone(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()
	phone := "+40743123123"
	password := "hackmetoday"

	api.registerUserByPhone(phone, password)

	var buffer bytes.Buffer

	acr := schedder.AccountCreationRequest{Phone: phone, Password: password}
	err := json.NewEncoder(&buffer).Encode(acr)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	r := w.Result()

	var response schedder.Response
	err = json.NewDecoder(r.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "phone already used", response.Error)
}

func TestGenerateTokenWithEmail(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackmenow"
	device := "schedder test"
	tgr := schedder.TokenGenerationRequest{
		Email:    email,
		Password: password,
		Device:   device,
	}
	err := json.NewEncoder(&buffer).Encode(tgr)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost, "/accounts/self/sessions", &buffer,
	)
	req.RemoteAddr = "127.0.0.1"
	w := httptest.NewRecorder()

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response schedder.TokenGenerationResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	t.Logf("\t\t\t%#v\n", response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusCreated || response.Error != "" {
		t.Logf("expected %d, got %s", http.StatusCreated, resp.Status)
		t.Logf("got error: %s", response.Error)
		t.FailNow()
	}
}

func TestGenerateTokenWithShortDeviceName(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackmenow"
	device := "short"

	tgr := schedder.TokenGenerationRequest{
		Email:    email,
		Password: password,
		Device:   device,
	}
	err := json.NewEncoder(&buffer).Encode(tgr)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost, "/accounts/self/sessions", &buffer,
	)
	req.RemoteAddr = "127.0.0.1"
	w := httptest.NewRecorder()

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response schedder.TokenGenerationResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	t.Logf("\t\t\t%#v\n", response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest ||
		response.Error != "device name too short" {
		t.Logf("expected %d, got %s", http.StatusCreated, resp.Status)
		t.Logf("got error: %s", response.Error)
		t.FailNow()
	}
}

func TestGenerateTokenWithPhone(t *testing.T) {
	t.Parallel()
	type Response struct {
		schedder.TokenGenerationResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackmenow"
	device := "schedder test"
	tgr := schedder.TokenGenerationRequest{
		Phone:    phone,
		Password: password,
		Device:   device,
	}
	err := json.NewEncoder(&buffer).Encode(tgr)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost, "/accounts/self/sessions", &buffer,
	)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	api.registerUserByPhone(phone, password)
	api.activateUserByPhone(phone)

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusCreated, resp.StatusCode)
	expect(t, "", response.Error)
}

func TestGenerateTokenWithBadPassword(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackmenow"
	device := "schedder test"
	tgr := schedder.TokenGenerationRequest{
		Phone:    phone,
		Password: password,
		Device:   device,
	}
	err := json.NewEncoder(&buffer).Encode(tgr)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost, "/accounts/self/sessions", &buffer,
	)
	req.RemoteAddr = "127.0.0.1"
	w := httptest.NewRecorder()

	api.registerUserByPhone(phone, password+"bad")
	api.activateUserByPhone(phone)

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, response.Error, "invalid password")
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGenerateTokenWithoutEmailOrPhone(t *testing.T) {
	t.Parallel()
	type TestData struct {
		phone string
		email string
		err   string
	}

	type Response struct {
		schedder.AccountCreationResponse
		Error string `json:"error,omitempty"`
	}

	testdata := []TestData{
		{err: "expected phone or email"},
		{phone: "invalid_phone", err: "no user with phone"},
		{email: "invalid_email", err: "no user with email"},
	}

	for _, td := range testdata {
		testName := fmt.Sprintf(
			"TestGenerateTokenWith: email=%s phone=%s err=%s",
			td.email, td.phone, td.err,
		)
		t.Run(testName, func(t *testing.T) {
			api := BeginTx(t)
			defer api.Rollback()

			var buffer bytes.Buffer
			tgr := schedder.TokenGenerationRequest{
				Phone:    td.phone,
				Email:    td.email,
				Password: "hackme",
				Device:   "schedder test",
			}

			err := json.NewEncoder(&buffer).Encode(tgr)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(
				http.MethodPost, "/accounts/self/sessions", &buffer,
			)
			req.RemoteAddr = "127.0.0.1"
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)
			resp := w.Result()

			var response Response
			err = json.NewDecoder(resp.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}
			expect(t, response.Error, td.err)
			expect(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	endpoint := api.AuthenticatedEndpoint(http.HandlerFunc(handler))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	endpoint.ServeHTTP(w, req)
	resp := w.Result()

	data := make(map[string]string)
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	expect(t, "", data["error"])
	expect(t, http.StatusOK, resp.StatusCode)
}

func TestAuthMiddlewareWithBadToken(t *testing.T) {
	const invalidToken = `k6CVEMpWIHDkaZ+fmmZl4ApE+KfpO3DDGHdR7B3Ql6Uwt4zJpnUnlmNHPPVlDoHYTTWnWoEQcC1tyYjKD89mmw`
	t.Parallel()
	testdata := []string{
		"bad_token",
		invalidToken,
		"not-bearer",
	}

	for _, v := range testdata {
		t.Run("TestAuthMiddlewareWithBadToken: "+v, func(t *testing.T) {
			api := BeginTx(t)
			defer api.Rollback()

			token := v
			handler := func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}

			endpoint := api.AuthenticatedEndpoint(http.HandlerFunc(handler))

			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			scheme := "Bearer"
			if token == "not-bearer" {
				scheme = "Basic"
			}
			req.Header.Add("Authorization", scheme+" "+token)
			w := httptest.NewRecorder()

			endpoint.ServeHTTP(w, req)
			resp := w.Result()

			data := make(map[string]string)
			err := json.NewDecoder(resp.Body).Decode(&data)
			if err != nil {
				t.Fatal(err)
			}
			expect(t, "invalid token", data["error"])
			expect(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestGetSessionsForAccount(t *testing.T) {
	t.Parallel()
	type GetSessionResponse struct {
		schedder.SessionsForAccountResponse
		Error string
	}
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	req := httptest.NewRequest(http.MethodGet, "/accounts/self/sessions", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	expect(t, http.StatusOK, resp.StatusCode)

	var response GetSessionResponse
	json.NewDecoder(resp.Body).Decode(&response.Sessions)

	expect(t, "", response.Error)

	for _, s := range response.Sessions {
		if time.Until(s.ExpirationDate) < (7 * 24 * time.Hour) {
			t.Fatalf("session %s doesn't expire in 7 days", s.SessionID)
		}

		t.Logf("Session %s expires on %s", s.SessionID, s.ExpirationDate)
	}
}

func TestRevokeSession(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)
	sessions := api.getSessions(token)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/accounts/self/sessions/"+sessions[0].String(),
		nil,
	)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	expect(t, "", data.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	req = httptest.NewRequest(
		http.MethodDelete,
		"/accounts/self/sessions/"+sessions[0].String(),
		nil,
	)
	req.Header.Add("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp = w.Result()

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusUnauthorized, resp.StatusCode)
	expect(t, "invalid token", data.Error)
}

func TestRevokeSessionWithBadSessionId(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	otherEmail := "other@example.com"
	otherPassword := "hackmenow"
	api.registerUserByEmail(otherEmail, otherPassword)
	api.activateUserByEmail(otherEmail)
	otherToken := api.generateToken(otherEmail, otherPassword)

	sessionID := "361e5d4f-4092-4d0b-8155-837b113c25ab"

	req := httptest.NewRequest(
		http.MethodDelete, "/accounts/self/sessions/"+sessionID, nil,
	)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "invalid session", data.Error)

	otherSessions := api.getSessions(otherToken)
	expect(t, 1, len(otherSessions))
}

func TestGetAccountByEmailAsAdmin(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "user@gmail.com"
	password := "hackmenow"

	otherEmail := "other@example.com"
	otherPassword := "hackmenow_other"

	api.registerUserByEmail(email, password)
	api.registerUserByEmail(otherEmail, otherPassword)
	api.activateUserByEmail(email)
	api.activateUserByEmail(otherEmail)
	token := api.generateToken(email, password)

	api.forceAdmin(email, true)

	r := httptest.NewRequest(
		http.MethodGet, "/accounts/by-email/"+otherEmail, nil,
	)
	r.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", data.Error)
	expect(t, http.StatusOK, resp.StatusCode)
}

func TestGetAccountByEmailAsAdminWithoutBeingAdmin(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "user@example.com"
	password := "hackmenow"

	otherEmail := "other@example.com"
	otherPassword := "hackmenow_other"

	api.registerUserByEmail(email, password)
	api.registerUserByEmail(otherEmail, otherPassword)
	api.activateUserByEmail(email)
	api.activateUserByEmail(otherEmail)

	r := httptest.NewRequest(http.MethodGet, "/accounts/by-email/"+otherEmail, nil)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "not admin", data.Error)
	expect(t, http.StatusForbidden, resp.StatusCode)
}

func TestGetAccountByEmailAsAdminWithInvalidEmail(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "user@example.com"
	password := "hackmenow"

	otherEmail := "other@example.com"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	api.forceAdmin(email, true)

	r := httptest.NewRequest(http.MethodGet, "/accounts/by-email/"+otherEmail, nil)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err.Error() != "EOF" {
		t.Fatal(err)
	}

	expect(t, "invalid email", data.Error)
	expect(t, http.StatusNotFound, resp.StatusCode)
}

func TestSetAdmin(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "user@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)

	otherEmail := "other@example.com"
	otherPassword := "hackmenow_other"

	api.registerUserByEmail(otherEmail, otherPassword)
	api.activateUserByEmail(otherEmail)

	token := api.generateToken(email, password)

	api.forceAdmin(email, true)

	accountID := api.findAccountByEmail(otherEmail)
	r := httptest.NewRequest(
		http.MethodPost,
		"/accounts/"+accountID.String()+"/admin",
		strings.NewReader("{\"admin\": true}"),
	)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+token)

	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusOK, resp.StatusCode)
}

func TestSetBusiness(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "user@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)

	otherEmail := "other@example.com"
	otherPassword := "hackmenow_other"

	api.registerUserByEmail(otherEmail, otherPassword)
	api.activateUserByEmail(otherEmail)

	token := api.generateToken(email, password)

	api.forceAdmin(email, true)

	accountID := api.findAccountByEmail(otherEmail)
	r := httptest.NewRequest(
		http.MethodPost,
		"/accounts/"+accountID.String()+"/business",
		strings.NewReader("{\"business\": true}"),
	)
	w := httptest.NewRecorder()
	r.Header.Add("Authorization", "Bearer "+token)

	api.ServeHTTP(w, r)

	resp := w.Result()
	expect(t, http.StatusOK, resp.StatusCode)
}
