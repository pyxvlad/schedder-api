package schedder_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api"
)

func TestGetTimetable(t *testing.T) {
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
	starting := time.Time{}.Add(10 * time.Hour)
	ending := starting.Add(8 * time.Hour)

	api.setSchedule(token, accountID, tenantID, starting, ending , time.Now().Weekday())
	endpoint := fmt.Sprintf(
		"/tenants/%s/services/%s/timetable",
		tenantID, serviceID,
	)
	r, err := NewJSONRequest(http.MethodGet, endpoint, schedder.TimetableRequest{Date: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.TimetableResponse
	json.NewDecoder(resp.Body).Decode(&response)

	t.Log(response.Error)
	expect(t, http.StatusOK, resp.StatusCode)
	expect(t, "",  response.Error)

	// TODO: fix this ugly hack caused by not having a Time struct without date.
	// Perhaps use pgtype.Time?
	starting = starting.AddDate(1999, 0, 0)
	ending = ending.AddDate(1999, 0, 0)

	entries := ( ending.Sub(starting) / (30 * time.Minute))
	for _, timestamp := range response.Times {
		deltaFromStart := timestamp.Sub(starting)
		index := deltaFromStart/(30 * time.Minute)
		if index < 0 || index >= entries {
			t.Fatalf(
				"Found time %s, corresponding to %d which exceeds %d",
				timestamp, index, entries,
			)
		}
		
	}
}

func TestGetTimetableWithoutSchedule(t *testing.T) {
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
		"/tenants/%s/services/%s/timetable",
		tenantID, serviceID,
	)
	r, err := NewJSONRequest(http.MethodGet, endpoint, schedder.TimetableRequest{Date: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.TimetableResponse
	json.NewDecoder(resp.Body).Decode(&response)

	t.Log(response.Error)
	expect(t, http.StatusOK, resp.StatusCode)
	expect(t, "",  response.Error)
}

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

	today := time.Now().Truncate(24 * time.Hour)
	starting := today.Add(10 * time.Hour)
	ending := starting.Add(8 * time.Hour)
	desired := starting.Add(2 * time.Hour)
	fmt.Printf("desired: %v\n", desired)
	fmt.Printf("starting: %v\n", starting)
	fmt.Printf("ending: %v\n", ending)
	api.setSchedule(token, accountID, tenantID, starting, ending , time.Now().Weekday())

	endpoint := fmt.Sprintf(
		"/tenants/%s/services/%s/schedule",
		tenantID, serviceID,
	)
	request := schedder.CreateAppointmentRequest{
		Starting: desired,
	}
	r, err := NewJSONRequest(http.MethodPost, endpoint, request)
	r.Header.Add("Authorization", "Bearer " + token)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	resp := w.Result()
	var response schedder.CreateAppointmentResponse
	json.NewDecoder(resp.Body).Decode(&response)


	// api.tx.Commit(context.TODO())
	expect(t, "", response.Error)
	expect(t, http.StatusCreated, resp.StatusCode)

	unexpect(t, uuid.Nil, response.AppointmentID)
}
