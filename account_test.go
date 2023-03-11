package schedder_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestRegisterWithEmail(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackmetoday"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s", response.Error)

	expect(t, http.StatusCreated, resp.StatusCode)
	expect(t, "", response.Error)
	expect(t, email, response.Email)
}

func TestRegisterWithoutJson(t *testing.T) {
	t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	var buffer bytes.Buffer
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", response.Error)

	expect(t, "invalid json", response.Error)
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterWithoutEmailOrPhone(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer
	json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Password: "hackmenow"})
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.PostAccountResponse
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
	testdata := [][2]string{{"+40723123123", "+40723123123"}, {"0723123123", "+40723123123"}, {"+4 0 7 2 3 1 2 3 1 2 3", "+40723123123"}}

	for _, v := range testdata {
		loop_data := new([2]string)
		copy(loop_data[:], v[:])
		t.Run(v[0], func(t *testing.T) {
			data := *loop_data
			t.Parallel()

			api := BeginTx(t)
			defer api.Rollback()

			var buffer bytes.Buffer

			phone := data[0]
			expected := data[1]
			password := "hackmenow"
			err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/accounts", &buffer)
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			resp := w.Result()

			var response schedder.PostAccountResponse
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
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	data := make(map[string]string)
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	json_err := data["error"]
	t.Logf("%#v", json_err)

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "phone too short/long", json_err)
	expect(t, 1, len(data))
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
		password := "hackme"
		err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Email: email, Password: password})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/accounts", &buffer)
		w := httptest.NewRecorder()

		api.ServeHTTP(w, req)

		resp := w.Result()

		data := make(map[string]string)
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			t.Fatal(err)
		}

		json_err := data["error"]
		t.Logf("%#v", json_err)

		unexpect(t, "", json_err)
		expect(t, 1, len(data))
	})
}

func TestGenerateTokenWithEmail(t *testing.T) {
	t.Parallel()
	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackmenow"
	device := "schedder test"
	err := json.NewEncoder(&buffer).Encode(schedder.GenerateTokenRequest{Email: email, Password: password, Device: device})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/accounts/self/sessions", &buffer)
	req.RemoteAddr = "127.0.0.1"
	w := httptest.NewRecorder()

	api.register_user_by_email(email, password)

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response Response
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

	if response.Email != email && response.Phone != "" {
		t.Fatalf("expected %s, got %s", email, response.Email)
	}
}

func TestGenerateTokenWithPhone(t *testing.T) {
	t.Parallel()
	type Response struct {
		schedder.GenerateTokenResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackmenow"
	device := "schedder test"
	err := json.NewEncoder(&buffer).Encode(schedder.GenerateTokenRequest{Phone: phone, Password: password, Device: device})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/accounts/self/sessions", &buffer)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	api.register_user_by_phone(phone, password)

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
	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackmenow"
	device := "schedder test"
	err := json.NewEncoder(&buffer).Encode(schedder.GenerateTokenRequest{Phone: phone, Password: password, Device: device})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/accounts/self/sessions", &buffer)
	req.RemoteAddr = "127.0.0.1"
	w := httptest.NewRecorder()

	api.register_user_by_phone(phone, password+"bad")

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
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
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	testdata := []TestData{
		{err: "expected phone or email"},
		{phone: "invalid_phone", err: "no user with phone"},
		{email: "invalid_email", err: "no user with email"},
	}

	for _, td := range testdata {
		test_name := fmt.Sprintf("TestGenerateTokenWith: email=%s phone=%s err=%s", td.email, td.phone, td.err)
		t.Run(test_name, func(t *testing.T) {

			api := BeginTx(t)
			defer api.Rollback()

			var buffer bytes.Buffer

			err := json.NewEncoder(&buffer).Encode(schedder.GenerateTokenRequest{Phone: td.phone, Email: td.email, Password: "hackme", Device: "schedder test"})
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest("POST", "/accounts/self/sessions", &buffer)
			req.RemoteAddr = "127.0.0.1"
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)
			resp := w.Result()

			var response Response
			err = json.NewDecoder(resp.Body).Decode(&response)
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
	api.register_user_by_email(email, password)
	token := api.generate_token(email, password)

	endpoint := api.AuthenticatedEndpoint(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	endpoint.ServeHTTP(w, req)
	resp := w.Result()

	data := make(map[string]string)
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err.Error() != "EOF" {
		t.Fatal(err)
	}
	expect(t, "", data["error"])
	expect(t, http.StatusOK, resp.StatusCode)
}

func TestAuthMiddlewareWithBadToken(t *testing.T) {
	t.Parallel()
	testdata := []string{"bad_token", "k6CVEMpWIHDkaZ+fmmZl4ApE+KfpO3DDGHdR7B3Ql6Uwt4zJpnUnlmNHPPVlDoHYTTWnWoEQcC1tyYjKD89mmw", "not-bearer"}

	for _, v := range testdata {
		t.Run("TestAuthMiddlewareWithBadToken: "+v, func(t *testing.T) {
			api := BeginTx(t)
			defer api.Rollback()

			token := v

			endpoint := api.AuthenticatedEndpoint(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

			req := httptest.NewRequest("POST", "/test", nil)
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
			if err != nil && err.Error() != "EOF" {
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
		schedder.GetSessionsResponse
		Error string
	}
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.register_user_by_email(email, password)
	token := api.generate_token(email, password)

	req := httptest.NewRequest("GET", "/accounts/self/sessions", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	expect(t, http.StatusOK, resp.StatusCode)

	var response GetSessionResponse
	json.NewDecoder(resp.Body).Decode(&response.Sessions)

	expect(t, "", response.Error)

	for _, s := range response.Sessions {
		if s.ExpirationDate.Sub(time.Now()) < (7 * 24 * time.Hour) {
			t.Fatalf("session %s doesn't expire in 7 days", s.ID)
		}

		t.Logf("Session %s expires on %s", s.ID, s.ExpirationDate)
	}
}

func TestRevokeSession(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"
	api.register_user_by_email(email, password)
	token := api.generate_token(email, password)
	sessions := api.get_sessions(token)

	req := httptest.NewRequest("DELETE", "/accounts/self/sessions/"+sessions[0].String(), nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err.Error() != "EOF" {
		t.Fatal(err)
	}
	expect(t, "", data.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	req = httptest.NewRequest("DELETE", "/accounts/self/sessions/"+sessions[0].String(), nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp = w.Result()

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err.Error() != "EOF" {
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
	api.register_user_by_email(email, password)
	token := api.generate_token(email, password)

	session_id := "361e5d4f-4092-4d0b-8155-837b113c25ab"

	req := httptest.NewRequest("DELETE", "/accounts/self/sessions/"+session_id, nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var data schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil && err.Error() != "EOF" {
		t.Fatal(err)
	}

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "invalid session", data.Error)
}
