package schedder_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestCreateReview(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "somebody@example.com"
	password := "hackmenow"

	tenantName := "Zana Maseluta"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	message := strings.Repeat("lorem ipsum", 340)
	rating := 4
	t.Logf("len(message): %v\n", len(message))
	request := schedder.CreateReviewRequest{Message: message, Rating: rating}
	endpoint := fmt.Sprintf("/tenants/%s/reviews", tenantID)
	r, err := NewJSONRequest(http.MethodPost, endpoint, request)
	r.Header.Add("Authorization", "Bearer "+api.generateToken(email, password))
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)

	resp := w.Result()
	var response schedder.Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusCreated, resp.StatusCode)
}

func TestReviews(t *testing.T) {
	api := BeginTx(t)
	defer api.Rollback()

	email := "somebody@example.com"
	password := "hackmenow"

	tenantName := "Zana Maseluta"

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	token := api.generateToken(email, password)
	message := strings.Repeat("lorem ipsum", 340)
	rating := 4
	t.Logf("len(message): %v\n", len(message))
	api.createReview(token, tenantID, message, rating)

	endpoint := fmt.Sprintf("/tenants/%s/reviews", tenantID)
	r := httptest.NewRequest(http.MethodGet, endpoint, nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.ReviewsResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusOK, resp.StatusCode)

	// check if our account did an review
	accountID := api.findAccountByEmail(email)
	found := false
	for i := range response.Reviews {
		review := &response.Reviews[i]
		if review.AccountID == accountID {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Couldn't find review made by %s", accountID)
	}
}
