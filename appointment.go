package schedder

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type CreateAppointmentRequest struct {
	// Starting represents when the user wants to create an appointment.
	Starting time.Time `json:"starting"`
}

type CreateAppointmentResponse struct {
	Response
	AppointmentID uuid.UUID `json:"appointment_id"`
}

type TimetableRequest struct {
	// Date represents the date for which to get the timetable.
	Date time.Time `json:"date"`
}

type TimetableResponse struct {
	Response
	Times []time.Time `json:"times"`
}

func (a *API) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceID := ctx.Value(CtxServiceID).(uuid.UUID)
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*CreateAppointmentRequest)

	serviceData, err := a.db.GetServiceDurationAndPersonnel(ctx, serviceID)
	if err != nil {
		JsonError(w, http.StatusBadRequest, "invalid service")
		return
	}

	timetableParams := database.GetTimetableForDateParams{
		Weekday: request.Starting.Weekday(),
		PersonnelID: serviceData.AccountID,
		DesiredDate: request.Starting,
	}
	timetable, err := a.db.GetTimetableForDate(ctx, timetableParams)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}


	ok := false
	for _, value := range timetable {
		year, m, day := request.Starting.Date()
		t := value.Times.AddDate(year-2000, int(m)-1, day-1)
		if t.Equal(request.Starting) {
			ok = true
			break
		}
	}

	if !ok {
		JsonError(w, http.StatusBadRequest, "invalid time")
		return
	}

	params := database.CreateAppointmentParams{
		ServiceID: serviceID,
		AccountID: authenticatedID,
		Starting:  request.Starting,
	}
	appointmentID, err := a.db.CreateAppointment(ctx, params)
	if err != nil {
		JsonError(w, http.StatusBadRequest, "couldn't create appointment")
		return
	}

	JsonResp(w, http.StatusCreated, CreateAppointmentResponse{AppointmentID: appointmentID})
}

func (a *API) Timetable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceID := ctx.Value(CtxServiceID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*TimetableRequest)


	serviceData, err := a.db.GetServiceDurationAndPersonnel(ctx, serviceID)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "couldn't fetch service")
		return
	}

	var duration time.Duration
	err = serviceData.Duration.AssignTo(&duration)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "couldn't convert durations")
		return
	}


	rows, err := a.db.GetTimetableForDate(ctx, database.GetTimetableForDateParams{
		DesiredDate: request.Date,
		Weekday: request.Date.Weekday(),
		PersonnelID: serviceData.AccountID,
	})
	if err != nil {
		JsonError(w, http.StatusBadRequest, "bad request")
		return
	}

	var response TimetableResponse
	response.Times = make([]time.Time, 0, len(rows))

	delta := int(duration / (30 * time.Minute))

	count := 0
	for _, row := range rows {
		if row.IsBlocked {
			count = 0
		} else {
			count++
		}
		if count >= delta {
			response.Times = append(response.Times, row.Times)
		}
	}

	JsonResp(w, http.StatusOK, response)
}
