package schedder

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type CreateAppointmentRequest struct {
	Starting time.Time `json:"starting"`
}

type CreateAppointmentResponse struct {
	AppointmentID uuid.UUID `json:"appointment_id"`
}

func (a *API) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceID := ctx.Value(CtxServiceID).(uuid.UUID)
	personnelID := ctx.Value(CtxAccountID).(uuid.UUID)
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*CreateAppointmentRequest)

	params := database.CreateAppointmentParams{
		ServiceID:   serviceID,
		PersonnelID: personnelID,
		AccountID:   authenticatedID,
		Starting:    request.Starting,
	}
	appointmentID, err := a.db.CreateAppointment(ctx, params)
	if err != nil {
		JsonError(w, http.StatusBadRequest, "couldn't create appointment")
		return
	}

	JsonResp(w, http.StatusCreated, CreateAppointmentResponse{appointmentID})
}
