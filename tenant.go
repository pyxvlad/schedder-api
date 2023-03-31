package schedder

import (
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type CreateTenantRequest struct {
	Name string `json:"name"`
}

type CreateTenantResponse struct {
	Response
	TenantID uuid.UUID `json:"tenant_id"`
}

type tenantsResponse struct {
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantName string    `json:"name"`
}

type GetTenantsResponse struct {
	Response
	Tenants []tenantsResponse `json:"tenants,omitempty"`
}

func (a *API) CreateTenant(w http.ResponseWriter, r *http.Request) {
	accountID := r.Context().Value(CtxAccountID).(uuid.UUID)
	request := r.Context().Value(CtxJSON).(*CreateTenantRequest)

	if len(request.Name) < 8 || len(request.Name) > 80 {
		jsonError(w, http.StatusBadRequest, "invalid name")
		return
	}

	var response CreateTenantResponse
	var err error
	response.TenantID, err = a.db.CreateTenantWithAccount(r.Context(), database.CreateTenantWithAccountParams{AccountID: accountID, TenantName: request.Name})
	if err != nil {
		jsonError(w, http.StatusBadRequest, "couldn't create tenant")
		return
	}

	jsonResp(w, http.StatusCreated, response)
}

func (a *API) GetTenants(w http.ResponseWriter, r *http.Request) {

	tenants, err := a.db.GetTenants(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't get tenants")
		return
	}

	var response GetTenantsResponse
	response.Tenants = make([]tenantsResponse, 0, len(tenants))

	for _, t := range tenants {
		response.Tenants = append(response.Tenants, tenantsResponse{TenantID: t.TenantID, TenantName: t.TenantName})
	}

	jsonResp(w, http.StatusOK, response)
}
