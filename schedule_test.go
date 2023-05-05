package schedder_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gitlab.com/vlad.anghel/schedder-api"
)

func NewJSONRequest[T any](method string, target string, data T) (*http.Request, error) {
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(data)
	if err != nil {
		return nil, err
	}
	return httptest.NewRequest(method, target, &buff), nil
}

func TestSetSchedule(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "test@example.com"
	password := "hackmenow"

	tenantName := "Zâna Măseluță"
	weekday := time.Monday

	tenantID := api.createTenantAndAccount(email, password, tenantName)
	accountID := api.findAccountByEmail(email)

	starting := time.Time{}.Add(10 * time.Hour)
	ending := time.Time{}.Add(18 * time.Hour)

	endpoint := fmt.Sprintf(
		"/tenants/%s/personnel/%s/schedule", tenantID, accountID,
	)

	t.Log(endpoint)

	req, err := NewJSONRequest(
		http.MethodPost,
		endpoint,
		schedder.SetScheduleRequest{
			Weekday: weekday,
			Starting: starting,
			Ending: ending,
		},
	)
	req.Header.Add(
		"Authorization", "Bearer " + api.generateToken(email, password),
	)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Log(resp.Status)
		var response schedder.Response
		io.Copy(os.Stdout, resp.Body)
		err := json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Result: %s: %s", resp.Status, response.Error)
	}
}
