package schedder_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	"gitlab.com/vlad.anghel/schedder-api"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

var conn *pgx.Conn

func TestMain(m *testing.M) {
	var err error

	pg_uri := schedder.RequiredEnv("SCHEDDER_TEST_POSTGRES", "postgres://test_user@localhost/schedder_test")
	std_db, err := sql.Open("pgx", pg_uri)
	if err != nil {
		panic(err)
	}
	database.ResetDB(std_db)
	database.MigrateDB(std_db)
	if err := std_db.Close(); err != nil {
		panic(err)
	}

	conn, err = pgx.Connect(context.Background(), pg_uri)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

type ApiTx struct {
	*schedder.API
	tx pgx.Tx
	t  *testing.T
}

func BeginTx(t *testing.T) ApiTx {
	tx, err := conn.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatalf("testing: BeginTx: %e", err)
	}

	var api ApiTx
	api.API = schedder.New(tx)
	api.tx = tx
	api.t = t
	return api
}

func (a *ApiTx) Rollback() {
	err := a.tx.Rollback(context.Background())
	if err != nil {
		a.t.Fatalf("testing: RollbackTx: %e", err)
	}
}

func TestRegisterWithEmail(t *testing.T) {

	api := BeginTx(t)
	defer api.Rollback()

	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackme"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.PostAccount(w, req)

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

	api := BeginTx(t)
	defer api.Rollback()

	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	var buffer bytes.Buffer
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.PostAccount(w, req)

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

func expect[T comparable](t *testing.T, expected T, got T) {
	if expected != got {
		t.Fatalf("expected %#v, got %#v", expected, got)
	}
}

func unexpect[T comparable](t *testing.T, unexpected T, got T) {
	if unexpected == got {
		t.Fatalf("unexpected %#v", unexpected)
	}
}

func TestRegisterWithoutEmailOrPhone(t *testing.T) {

	api := BeginTx(t)
	defer api.Rollback()

	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	req := httptest.NewRequest("POST", "/accounts", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	api.PostAccount(w, req)

	resp := w.Result()

	var response Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", response.Error)
	expect(t, "expected phone or email", response.Error)
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterWithPhone(t *testing.T) {
	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	testdata := [][2]string{{"+40723123123", "+40723123123"}, {"0723123123", "+40723123123"}, {"+4 0 7 2 3 1 2 3 1 2 3", "+40723123123"}}

	for _, v := range testdata {
		t.Run("TestPostAccountWithPhone: "+v[0], func(t *testing.T) {
			//t.Parallel()

			api := BeginTx(t)
			defer api.Rollback()

			var buffer bytes.Buffer

			phone := v[0]
			expected := v[1]
			password := "hackme"
			err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/accounts", &buffer)
			w := httptest.NewRecorder()

			api.PostAccount(w, req)

			resp := w.Result()

			var response Response
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
	//t.Parallel()

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+4 0712 123 1 3"
	password := "hackme"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/accounts", &buffer)
	w := httptest.NewRecorder()

	api.PostAccount(w, req)

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

		api.PostAccount(w, req)

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

func (a *ApiTx) register_user_by_email(email string, password string) {
	req := httptest.NewRequest("POST", "/accounts", strings.NewReader("{\"email\": \""+email+"\", \"password\": \""+password+"\"}"))
	w := httptest.NewRecorder()
	a.PostAccount(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	data := make(map[string]string)
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	if json_err, has_error := data["error"]; has_error {
		a.t.Fatalf("register_user: %s", json_err)
	}
}

func (a *ApiTx) register_user_by_phone(phone string, password string) {
	req := httptest.NewRequest("POST", "/accounts", strings.NewReader("{\"phone\": \""+phone+"\", \"password\": \""+password+"\"}"))
	w := httptest.NewRecorder()
	a.PostAccount(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	data := make(map[string]string)
	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data["error"])
}

func (a *ApiTx) generate_token(email string, password string) (token string) {
	data := map[string]string{"email": email, "password": password}

	var b bytes.Buffer

	err := json.NewEncoder(&b).Encode(data)
	if err != nil {
		a.t.Fatalf("generate_token: couldn't generate json")
	}

	req := httptest.NewRequest("POST", "/token", &b)
	w := httptest.NewRecorder()

	a.GenerateToken(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("register_user: got status %s", resp.Status)
	}

	data = make(map[string]string)
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		a.t.Fatal(err)
	}

	expect(a.t, "", data["error"])

	token = data["token"]
	return
}

func TestGenerateTokenWithEmail(t *testing.T) {
	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	email := "mail@gmail.com"
	password := "hackme"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/token", &buffer)
	w := httptest.NewRecorder()

	api.register_user_by_email(email, password)

	api.ServeHTTP(w, req)
	body, _ := ioutil.ReadAll(req.Body)
	t.Logf(string(body))

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
	type Response struct {
		schedder.GenerateTokenResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackme"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/token", &buffer)
	w := httptest.NewRecorder()

	api.register_user_by_phone(phone, password)

	api.ServeHTTP(w, req)
	body, _ := ioutil.ReadAll(req.Body)
	t.Logf(string(body))

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
	type Response struct {
		schedder.PostAccountResponse
		Error string `json:"error,omitempty"`
	}

	api := BeginTx(t)
	defer api.Rollback()

	var buffer bytes.Buffer

	phone := "+40743123123"
	password := "hackme"
	err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: phone, Password: password})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/token", &buffer)
	w := httptest.NewRecorder()

	api.register_user_by_phone(phone, password+"bad")

	api.ServeHTTP(w, req)
	resp := w.Result()

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	expect(t, response.Error, "invalid password")
	expect(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGenerateTokenWithoutEmailOrPhone(t *testing.T) {
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

			err := json.NewEncoder(&buffer).Encode(schedder.PostAccountRequest{Phone: td.phone, Email: td.email, Password: "hackme"})
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest("POST", "/token", &buffer)
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
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackme"
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
