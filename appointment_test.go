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

func TestCreateAppointment(t *testing.T) {
	t.Parallel()
	api := BeginTx(t)
	defer api.Rollback()
	email := "tenant@example.com"
	password := "hackmenow"
	tenantName := "Zana Maseluta"

	accountID := api.registerUserByEmail(email, password)
	api.activateUserByEmail(email)
	api.forceBusiness(email, true)
	token := api.generateToken(email, password)
	tenantID := api.createTenant(token, tenantName)
	serviceID := api.createService(api.generateToken(email, password), tenantID, accountID , "control", 4.20, time.Hour)
	

	endpoint := fmt.Sprintf(
		"/tenants/%s/personnel/%s/servces/%s/schedule",
		tenantID, accountID, serviceID,
	)
	r, err := NewJSONRequest(http.MethodPost, endpoint, schedder.CreateAppointmentRequest{Starting: time.Now().Round(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.Response
	json.NewDecoder(resp.Body).Decode(&response)

	expect(t, "", response.Error)
}
