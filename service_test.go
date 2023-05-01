package schedder_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/vlad.anghel/schedder-api"
)

func TestCreateService(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "example@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	accountID := api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	api.forceBusiness(email, true)
	token := api.generateToken(email, password)
	tenantID := api.createTenant(token, tenantName)

	serviceName := "control_rutina"
	request := schedder.CreateServiceRequest{
		ServiceName: serviceName,
		Price: 4.20,
		Duration: 1 * time.Hour,
	}

	endpoint := fmt.Sprintf("/tenants/%s/personnel/%s/services", tenantID, accountID)

	req, err := NewJSONRequest(http.MethodPost, endpoint, request)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Authorization", "Bearer " + token)

	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	var response schedder.CreateServiceResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", response.Error)
	expect(t, http.StatusCreated, resp.StatusCode)
}

func TestServicesForPersonnel(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()

	email := "example@example.com"
	password := "hackmenow"
	tenantName := "Zâna Măseluță"

	accountID := api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	api.forceBusiness(email, true)
	token := api.generateToken(email, password)
	tenantID := api.createTenant(token, tenantName)

	serviceID := api.createService(token, tenantID, accountID, "service1", 4.20, 1 * time.Hour)

	endpoint := fmt.Sprintf("/tenants/%s/personnel/%s/services", tenantID, accountID)
	req := httptest.NewRequest(http.MethodGet, endpoint, nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	resp := w.Result()

	expect(t, http.StatusOK, resp.StatusCode)

	var response schedder.ServicesResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, "", response.Error)

	found := false
	for _, service := range response.Services {
		if service.PersonnelID != accountID {
			t.Fatal("service wasn't for this personnal")
		}

		if service.ServiceID == serviceID {
			found = true
		}
	}

	if !found {
		t.Fatal("couldn't find service ", serviceID)
	}
	
}
