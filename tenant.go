package schedder

import (
	"net/http"
	"unicode/utf8"

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

type AddTenantMemberRequest struct {
	AccountID uuid.UUID `json:"account_id"`
}

type memberResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	IsManager bool      `json:"is_manager"`
}

type GetTenantMembersResponse struct {
	Response
	Members []memberResponse `json:"members,omitempty"`
}

func (a *API) CreateTenant(w http.ResponseWriter, r *http.Request) {
	accountID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	request := r.Context().Value(CtxJSON).(*CreateTenantRequest)

	runes := utf8.RuneCountInString(request.Name)

	if runes < 8 || runes > 80 {
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

func (a *API) AddTenantMember(w http.ResponseWriter, r *http.Request) {
	request := r.Context().Value(CtxJSON).(*AddTenantMemberRequest)
	tenantID := r.Context().Value(CtxTenantID).(uuid.UUID)
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)

	err := a.db.AddTenantMember(r.Context(), database.AddTenantMemberParams{TenantID: tenantID, NewMemberID: request.AccountID, IsManager: false, OwnerID: authenticatedID})
	if err != nil {
		jsonError(w, http.StatusBadRequest, "already member")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) GetTenantMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(CtxTenantID).(uuid.UUID)
	rows, err := a.db.GetTenantMembers(r.Context(), tenantID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hmm")
		return
	}

	var response GetTenantMembersResponse
	response.Members = make([]memberResponse, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		var member memberResponse
		member.AccountID = row.AccountID
		member.Name = row.AccountName
		if row.Email.Valid {
			member.Email = row.Email.String
		}
		if row.Phone.Valid {
			member.Phone = row.Phone.String
		}
		member.IsManager = row.IsManager
		response.Members = append(response.Members, member)
	}

	jsonResp(w, http.StatusOK, response)
}
