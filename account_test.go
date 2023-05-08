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

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api"
)

func TestRegister(t *testing.T) {
	t.Parallel()
	email := "somebody@gmail.com"
	phone := "+40743420420"
	password := "hackmenow"

	createAccount := func(
		api *APITX, email string, phone string, password string,
	) (int, schedder.AccountCreationResponse) {
		api.t.Helper()

		var buffer bytes.Buffer

		err := json.NewEncoder(&buffer).Encode(
			schedder.AccountCreationRequest{
				Email:    email,
				Phone:    phone,
				Password: password,
			},
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
		return resp.StatusCode, response
	}

	t.Run("with email", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)

		statusCode, response := createAccount(api, email, "", password)

		t.Logf("%s", response.Error)

		expect(t, http.StatusCreated, statusCode)
		expect(t, "", response.Error)
		expect(t, email, response.Email)
	})

	t.Run("with short password", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		statusCode, response := createAccount(api, "mail@gmail.com", "", "meow")
		expect(t, "password too short", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("without email or phone", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		statusCode, response := createAccount(api, "", "", password)
		expect(t, "expected phone or email", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("register with phone", func(t *testing.T) {
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
				t.Parallel()
				api := BeginTx(t)
				data := *testData
				statusCode, response := createAccount(api, "", data[0], password)

				expect(t, "", response.Error)
				expect(t, http.StatusCreated, statusCode)
				expect(t, data[1], response.Phone)
			})
		}
	})
	t.Run("with short phone", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		statusCode, response := createAccount(api, "", "+4 0712 123 1 3", password)

		expect(t, "phone too short/long", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		statusCode, response := createAccount(api, "not.email", "", password)

		expect(t, http.StatusBadRequest, statusCode)
		expect(t, "invalid email", response.Error)
	})
	t.Run("duplicate email", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		api.registerUserByEmail(email, password)
		statusCode, response := createAccount(api, email, "", password)

		expect(t, "email already used", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("duplicate phone", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		api.registerUserByPhone(phone, password)

		statusCode, response := createAccount(api, "", phone, password)

		expect(t, "phone already used", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("passwordless phone", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		statusCode, response := createAccount(api, "", phone, "")

		expect(t, phone, response.Phone)
		expect(t, "", response.Error)
		expect(t, http.StatusCreated, statusCode)
	})
}

func TestGenerateToken(t *testing.T) {
	t.Parallel()

	email := "somebody@gmail.com"
	phone := "+40743420420"
	password := "hackmenow"
	device := "schedder test device"

	generateToken := func(
		api *APITX, email, phone, password, device string,
	) (int, schedder.TokenGenerationResponse) {
		api.t.Helper()
		var buffer bytes.Buffer
		tgr := schedder.TokenGenerationRequest{
			Email:    email,
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

		api.ServeHTTP(w, req)
		resp := w.Result()

		var response schedder.TokenGenerationResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, response
	}

	t.Run("with email", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)
		statusCode, response := generateToken(api, email, "", password, device)

		expect(t, "", response.Error)
		expect(t, http.StatusCreated, statusCode)
	})
	t.Run("short device name", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)
		statusCode, response := generateToken(api, email, "", password, "short")

		expect(t, "device name too short", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("with phone", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		api.registerUserByPhone(phone, password)
		api.activateUserByPhone(phone)

		statusCode, response := generateToken(api, "", phone, password, device)

		expect(t, http.StatusCreated, statusCode)
		expect(t, "", response.Error)
	})
	t.Run("bad password", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)

		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)

		statusCode, response := generateToken(api, email, "", "badpassword", device)
		expect(t, http.StatusBadRequest, statusCode)
		expect(t, "invalid password", response.Error)
	})
	t.Run("no user with email", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		statusCode, response := generateToken(api, "invalid_email", "", password, device)
		expect(t, "no user with email", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("no user with phone", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		statusCode, response := generateToken(api, "", "invalid_phone", password, device)
		expect(t, "no user with phone", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})
	t.Run("missing phone and email", func(t *testing.T) {
		t.Parallel()

		api := BeginTx(t)
		statusCode, response := generateToken(api, "", "", password, device)
		expect(t, "expected phone or email", response.Error)
		expect(t, http.StatusBadRequest, statusCode)
	})

}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	email := "test@example.com"
	password := "hackmenow"
	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	endpoint := api.AuthenticatedEndpoint(http.HandlerFunc(handler))
	request := func(scheme, token string) (int, schedder.Response) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Add("Authorization", fmt.Sprintf("%s %s", scheme, token))
		w := httptest.NewRecorder()

		endpoint.ServeHTTP(w, req)
		resp := w.Result()

		var response schedder.Response

		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		return resp.StatusCode, response
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		token := api.generateToken(email, password)
		statusCode, response := request("Bearer", token)

		expect(t, "", response.Error)
		expect(t, http.StatusOK, statusCode)

	})
	t.Run("bad token", func(t *testing.T) {
		t.Parallel()
		statusCode, response := request("Bearer", "bad_token")

		expect(t, "invalid token", response.Error)
		expect(t, http.StatusUnauthorized, statusCode)
	})
	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()
		const token = `k6CVEMpWIHDkaZ+fmmZl4ApE+KfpO3DDGHdR7B3Ql6Uwt4zJpnUnlmNHPPVlDoHYTTWnWoEQcC1tyYjKD89mmw`

		statusCode, response := request("Bearer", token)

		expect(t, "invalid token", response.Error)
		expect(t, http.StatusUnauthorized, statusCode)

	})
	t.Run("invalid scheme", func(t *testing.T) {
		t.Parallel()
		token := api.generateToken(email, password)
		statusCode, response := request("Basic", token)

		expect(t, "invalid token", response.Error)
		expect(t, http.StatusUnauthorized, statusCode)
	})
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

	email := "test@example.com"
	password := "hackmenow"
	revokeSession := func(api *APITX, token string, sessionID uuid.UUID) (int, schedder.Response) {
		endpoint := fmt.Sprintf("/accounts/self/sessions/%s", sessionID)
		req := httptest.NewRequest(http.MethodDelete, endpoint, nil)
		req.Header.Add("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		api.ServeHTTP(w, req)

		resp := w.Result()

		var response schedder.Response
		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}

		return resp.StatusCode, response
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		defer api.Rollback()

		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)
		token := api.generateToken(email, password)
		sessions := api.getSessions(token)

		statusCode, response := revokeSession(api, token, sessions[0])

		expect(t, "", response.Error)
		expect(t, http.StatusOK, statusCode)

		statusCode, response = revokeSession(api, token, sessions[0])
		expect(t, "invalid token", response.Error)
		expect(t, http.StatusUnauthorized, statusCode)
	})
	t.Run("bad session id", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)
		defer api.Rollback()

		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)
		token := api.generateToken(email, password)

		sessionID, err := uuid.NewRandom()
		if err != nil {
			t.Fatal(err)
		}

		statusCode, response := revokeSession(api, token, sessionID)

		expect(t, "invalid session", response.Error)
		expect(t, http.StatusBadRequest, statusCode)

		otherSessions := api.getSessions(token)
		expect(t, 1, len(otherSessions))
	})
}

func TestGetAccountByEmailAsAdmin(t *testing.T) {
	t.Parallel()
	email := "user@gmail.com"
	password := "hackmenow"

	otherEmail := "other@example.com"
	otherPassword := "hackmenow_other"

	accountByEmail := func(
		api *APITX, token string, otherEmail string,
	) (int, schedder.AccountByEmailAsAdminResponse) {
		r := httptest.NewRequest(
			http.MethodGet, "/accounts/by-email/"+otherEmail, nil,
		)
		r.Header.Add("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		api.ServeHTTP(w, r)

		resp := w.Result()
		var response schedder.AccountByEmailAsAdminResponse
		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, response
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		api.registerUserByEmail(email, password)
		api.registerUserByEmail(otherEmail, otherPassword)
		api.activateUserByEmail(email)
		api.activateUserByEmail(otherEmail)
		token := api.generateToken(email, password)

		api.forceAdmin(email, true)

		statusCode, response := accountByEmail(api, token, otherEmail)

		expect(t, "", response.Error)
		expect(t, http.StatusOK, statusCode)
	})
	t.Run("without being admin", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		api.registerUserByEmail(email, password)
		api.registerUserByEmail(otherEmail, otherPassword)
		api.activateUserByEmail(email)
		api.activateUserByEmail(otherEmail)

		token := api.generateToken(email, password)

		statusCode, response := accountByEmail(api, token, otherEmail)

		expect(t, "not admin", response.Error)
		expect(t, http.StatusForbidden, statusCode)
	})
	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()
		api := BeginTx(t)

		api.registerUserByEmail(email, password)
		api.activateUserByEmail(email)
		api.forceAdmin(email, true)
		token := api.generateToken(email, password)
		statusCode, response := accountByEmail(api, token, otherEmail)

		expect(t, "invalid email", response.Error)
		expect(t, http.StatusNotFound, statusCode)
	})
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
