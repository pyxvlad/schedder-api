package schedder

import (
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type CreateServiceRequest struct {
	ServiceName string        `json:"service_name"`
	Price       float64       `json:"price"`
	Duration    time.Duration `json:"duration"`
}

type CreateServiceResponse struct {
	Response
	ServiceID uuid.UUID `json:"service_id"`
}

type serviceResponse struct {
	PersonnelID uuid.UUID     `json:"personnel_id"`
	ServiceID   uuid.UUID     `json:"service_id"`
	ServiceName string        `json:"service_name"`
	Price       float64       `json:"price"`
	Duration    time.Duration `json:"duration"`
}

type ServicesResponse struct {
	Response
	Services []serviceResponse `json:"services"`
}

func (a *API) CreateService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	accountID := ctx.Value(CtxAccountID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*CreateServiceRequest)

	if request.Price < 0 || request.Price > 1000000 {
		JsonError(w, http.StatusBadRequest, "invalid price")
	}

	request.Price = math.Round(request.Price*100) / 100

	if request.Duration%time.Minute != 0 {
		JsonError(w, http.StatusBadRequest, "invalid duration")
		return
	}

	params := database.CreateServiceParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		ServiceName: request.ServiceName,
	}
	params.Price.Set(request.Price)
	params.Duration.Set(request.Duration)

	serviceID, err := a.db.CreateService(ctx, params)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	response := CreateServiceResponse{ServiceID: serviceID}
	JsonResp(w, http.StatusCreated, response)
}

func (a *API) ServicesForPersonnel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	personnelID := ctx.Value(CtxAccountID).(uuid.UUID)

	params := database.GetServicesParams{
		TenantID:  tenantID,
		AccountID: personnelID,
	}
	rows, err := a.db.GetServices(ctx, params)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	var response ServicesResponse
	response.Services = make([]serviceResponse, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		service := serviceResponse{
			PersonnelID: personnelID,
			ServiceName: row.ServiceName,
			ServiceID: row.ServiceID,
		}
		err := row.Price.AssignTo(&service.Price)
		if err != nil {
			JsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		err = row.Duration.AssignTo(&service.Duration)
		if err != nil {
			JsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		response.Services = append(response.Services, service)
	}

	JsonResp(w, http.StatusOK, response)
}

func (a *API) ServicesForTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)

	rows, err := a.db.GetServicesForTenant(ctx, tenantID)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	var response ServicesResponse
	response.Services = make([]serviceResponse, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		service := serviceResponse{
			PersonnelID: row.AccountID,
			ServiceName: row.ServiceName,
			ServiceID: row.ServiceID,
		}
		err := row.Price.AssignTo(&service.Price)
		if err != nil {
			JsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		err = row.Duration.AssignTo(&service.Duration)
		if err != nil {
			JsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		response.Services = append(response.Services, service)
	}

	JsonResp(w, http.StatusOK, response)
}
