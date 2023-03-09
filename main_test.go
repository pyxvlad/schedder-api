package schedder_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"runtime"

	"github.com/google/uuid"
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
		file = strings.TrimPrefix(file, wd + "/")

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
		file = strings.TrimPrefix(file, wd + "/")

		t.Fatalf("%s:%d: unexpected %#v, got %#v\n", file, line, unexpected, got)
	}
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

func (a *ApiTx) register_user_by_email(email string, password string) {
	req := httptest.NewRequest("POST", "/accounts", strings.NewReader("{\"email\": \""+email+"\", \"password\": \""+password+"\"}"))
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)

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
	a.ServeHTTP(w, req)

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
	req_data := schedder.GenerateTokenRequest{Email: email, Password: password, Device: "schedder testing"}

	var b bytes.Buffer

	err := json.NewEncoder(&b).Encode(req_data)
	if err != nil {
		a.t.Fatalf("generate_token: couldn't generate json")
	}

	req := httptest.NewRequest("POST", "/sessions", &b)

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


func (a *ApiTx) get_sessions(token string) (session_ids []uuid.UUID) {
	req := httptest.NewRequest("GET", "/accounts/self/sessions", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	a.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.GetSessionsResponse
	json.NewDecoder(resp.Body).Decode(&response)

	session_ids = make([]uuid.UUID, 0)
	for _, s := range response.Sessions {
		session_ids = append(session_ids, s.ID)
	}
	return session_ids
}


func TestWithInvalidJson(t *testing.T) {
	testdata := [][]string{
		{"POST", "/accounts"},
		{"POST", "/sessions"},
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
