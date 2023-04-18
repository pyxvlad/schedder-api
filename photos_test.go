package schedder_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api"
)

func TestAddTenantPhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	jpgFile, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}

	endpoint := fmt.Sprintf("/tenants/%s/photos", tenantID)
	r := httptest.NewRequest(http.MethodPost, endpoint, jpgFile)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	t.Log(resp.Status)

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusCreated, resp.StatusCode)
}

func TestListTenantPhotos(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}

	photoID := api.addTenantPhoto(token, tenantID, file)

	endpoint := fmt.Sprintf("/tenants/%s/photos", tenantID)
	r := httptest.NewRequest(http.MethodGet, endpoint, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	t.Log(resp.Status)

	var response schedder.ListTenantPhotosResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	found := false
	for _, ID := range response.Photos {
		t.Logf("photo %s", ID)
		if ID == photoID {
			found = true
		}
	}

	if !found {
		t.Fatalf("Didn't find photo %s", photoID)
	}
}

func TestDownloadTenantPhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}

	photoID := api.addTenantPhoto(token, tenantID, file)

	endpoint := fmt.Sprintf("/tenants/%s/photos/by-id/%s", tenantID, photoID)
	r := httptest.NewRequest(http.MethodGet, endpoint, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") == "application/json" {
		var response schedder.Response
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Error: %s", response.Error)
	}

	expect(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	_, err = os.Stat(api.PhotosPath() + checksum)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetProfilePhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/accounts/self/photo", file)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	t.Logf("Error: %s", response.Error)

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)
}

func TestDownloadProfilePhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}

	api.addProfilePhoto(token, file)
	r := httptest.NewRequest(http.MethodGet, "/accounts/self/photo", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") == "application/json" {
		var response schedder.Response
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Error: %s", response.Error)
	}

	expect(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	_, err = os.Stat(api.PhotosPath() + checksum)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadProfilePhotoWithoutAdding(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	r := httptest.NewRequest(http.MethodGet, "/accounts/self/photo", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatal("expected json")
	}
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, http.StatusBadRequest, resp.StatusCode)
	expect(t, "no photo", response.Error)
}

func TestDeleteProfilePhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"

	api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}
	api.addProfilePhoto(token, file)

	r := httptest.NewRequest(http.MethodDelete, "/accounts/self/photo", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") == "application/json" {
		t.Log("didn't expect json")
		var response schedder.Response
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		expect(t, "", response.Error)
	}

	expect(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteTenantPhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	file, err := os.Open("./testdata/1px.jpg")
	if err != nil {
		t.Fatal(err)
	}
	photoID := api.addTenantPhoto(token, tenantID, file)

	endpoint := fmt.Sprintf("/tenants/%s/photos/by-id/%s", tenantID, photoID)
	r := httptest.NewRequest(http.MethodDelete, endpoint, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") == "application/json" {
		t.Log("didn't expect json")
		var response schedder.Response
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		expect(t, "", response.Error)
	}

	expect(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteTenantPhotoWithoutPhoto(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "tester@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	photoID := uuid.New()
	endpoint := fmt.Sprintf("/tenants/%s/photos/by-id/%s", tenantID, photoID)
	r := httptest.NewRequest(http.MethodDelete, endpoint, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatal("expected json")
	}

	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "no photo", response.Error)

	expect(t, http.StatusNotFound, resp.StatusCode)
}
