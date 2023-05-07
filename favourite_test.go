package schedder_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestAddFavourite(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	email := "somebody@exmaple.com"
	password := "hackmenow"

	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	endpoint := fmt.Sprintf("/accounts/self/favourites/%s", tenantID)
	r := httptest.NewRequest(http.MethodPost, endpoint, nil)
	w := httptest.NewRecorder()

	r.Header.Add("Authorization", "Bearer "+token)

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusCreated, resp.StatusCode)
}

func TestFavourites(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	email := "somebody@exmaple.com"
	password := "hackmenow"

	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	api.addFavourite(token, tenantID)

	r := httptest.NewRequest(http.MethodGet, "/accounts/self/favourites", nil)
	w := httptest.NewRecorder()

	r.Header.Add("Authorization", "Bearer " + token)

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.FavouritesResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	found := false
	for _, ID := range response.TenantIDs {
		if ID == tenantID {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Couldn't find favourite tenant %s in %v", tenantID, response.TenantIDs)
	}
}

func TestRemoveFavourite(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()
	email := "somebody@exmaple.com"
	password := "hackmenow"

	tenantName := "Zâna Măseluță"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)

	api.addFavourite(token, tenantID)

	endpoint := fmt.Sprintf("/accounts/self/favourites/%s", tenantID)
	r := httptest.NewRequest(http.MethodDelete, endpoint, nil)
	w := httptest.NewRecorder()

	r.Header.Add("Authorization", "Bearer "+token)

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	r = httptest.NewRequest(http.MethodGet, "/accounts/self/favourites", nil)
	w = httptest.NewRecorder()

	r.Header.Add("Authorization", "Bearer " + token)

	api.ServeHTTP(w, r)

	resp = w.Result()
	var favouritesResponse schedder.FavouritesResponse
	err = json.NewDecoder(resp.Body).Decode(&favouritesResponse)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	for _, ID := range favouritesResponse.TenantIDs {
		if ID == tenantID {
			t.Fatalf("Didn't remove %s", tenantID)
		}
	}
}
