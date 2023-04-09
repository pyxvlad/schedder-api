package schedder_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestWriterVerifier(t *testing.T) {

	b := bytes.Buffer{}

	verifier := schedder.WriterVerifier{&b, "tester"}
	id := "tester@example.com"
	code := "nothing"
	verifier.SendVerification(id, code)
	message := b.String()

	if !strings.Contains(message, id) {
		t.Fatalf("message doesn't contain %s", id)
	}
	
	if !strings.Contains(message, code) {
		t.Fatalf("message doesn't contain %s", code)
	}
}

func TestActivateAccount(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)

	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Email = email
	request.Code = api.codes[email]

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK || response.Error != "" {
		t.Fatalf("Expected %d without error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}

	expect(t, email, response.Email)
}

func TestVerifyWithoutEmailOrPhone(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)

	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Code = "123456"

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest || response.Error != "missing email and phone" {
		t.Fatalf("Expected %d without error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}
}

func TestActivateAccountWithInvalidCode(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)

	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Email = email
	request.Code = "123123"

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest || response.Error != "invalid code" {
		t.Fatalf("Expected %d without error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}
}

func TestActivateAccountWithInvalidEmail(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"

	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Email = email
	request.Code = "123123"

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest || response.Error != "invalid email" {
		t.Fatalf("Expected %d with 'invalid email' error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}
}

func TestActivateAccountWithInvalidPhone(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	phone := "+40743123123"

	b := bytes.Buffer{}
	var request schedder.VerifyCodeRequest
	request.Phone = phone
	request.Code = "123123"

	err := json.NewEncoder(&b).Encode(request)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/accounts/self/verify", &b)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.VerifyCodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest || response.Error != "invalid phone" {
		t.Fatalf("Expected %d with 'invalid phone' error, got %s with error %v", http.StatusOK, resp.Status, response.Error)
	}
}
