package schedder

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

// CreateTenantRequest represents a request for the tenant creation endpoint.
type CreateTenantRequest struct {
	// Name represents the name of the newly created tenant.
	Name string `json:"name"`
}

// CreateTenantResponse represents the response of the tenant creation
// endpoint.
type CreateTenantResponse struct {
	Response
	// TenantID represents the ID of the newly created tenant.
	TenantID uuid.UUID `json:"tenant_id"`
}

// tenantsResponseEntry represents a tenant.
type tenantsResponseEntry struct {
	// TenantID represents the ID of the tenant.
	TenantID uuid.UUID `json:"tenant_id"`
	// Name represents the name of the tenant.
	Name string `json:"name"`
}

// TenantsResponse represents the response of the tenant listing endpoint.
type TenantsResponse struct {
	Response
	// Tenants is a list of tenants.
	Tenants []tenantsResponseEntry `json:"tenants,omitempty"`
}

// AddTenantMemberRequest represents a request to add a member to a tenant.
type AddTenantMemberRequest struct {
	// AccountID represents the ID of the account that should be added.
	AccountID uuid.UUID `json:"account_id"`
}

// memberResponse represents an entry in the tenant member listing endpoint
// response.
type memberResponse struct {
	// AccountID represents the ID of the member.
	AccountID uuid.UUID `json:"account_id"`
	// Name represents the name of the member.
	Name string `json:"name"`
	// Email represents the email of the member.
	Email string `json:"email,omitempty"`
	// Phone represents the phone number of the member.
	Phone string `json:"phone,omitempty"`
	// IsManager represents whether the member is a manager.
	IsManager bool `json:"is_manager"`
}

// TenantMembersResponse represents a response for the tenant member listing
// endpoint.
type TenantMembersResponse struct {
	Response
	// Members represents the list of members.
	Members []memberResponse `json:"members,omitempty"`
}

// CreateTenant creates a new tenant.
func (a *API) CreateTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*CreateTenantRequest)

	runes := utf8.RuneCountInString(request.Name)

	if runes < 8 || runes > 80 {
		JsonError(w, http.StatusBadRequest, "invalid name")
		return
	}

	var response CreateTenantResponse
	var err error
	ctwap := database.CreateTenantWithAccountParams{
		AccountID:  accountID,
		TenantName: request.Name,
	}

	response.TenantID, err = a.db.CreateTenantWithAccount(ctx, ctwap)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		JsonError(w, http.StatusBadRequest, "couldn't create tenant")
		return
	}

	JsonResp(w, http.StatusCreated, response)
}

// Tenants lists all tenants.
func (a *API) Tenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := a.db.GetTenants(r.Context())
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "couldn't get tenants")
		return
	}

	var response TenantsResponse
	response.Tenants = make([]tenantsResponseEntry, 0, len(tenants))

	for _, t := range tenants {
		response.Tenants = append(
			response.Tenants,
			tenantsResponseEntry{TenantID: t.TenantID, Name: t.TenantName},
		)
	}

	JsonResp(w, http.StatusOK, response)
}

// AddTenantMember adds a member to the tenant.
func (a *API) AddTenantMember(w http.ResponseWriter, r *http.Request) {
	request := r.Context().Value(CtxJSON).(*AddTenantMemberRequest)
	tenantID := r.Context().Value(CtxTenantID).(uuid.UUID)
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	atmp := database.AddTenantMemberParams{
		TenantID:    tenantID,
		NewMemberID: request.AccountID,
		IsManager:   false,
		OwnerID:     authenticatedID,
	}

	err := a.db.AddTenantMember(
		r.Context(), atmp,
	)
	if err != nil {
		JsonError(w, http.StatusBadRequest, "already member")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// TenantMembers lists the members of a tenant.
func (a *API) TenantMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(CtxTenantID).(uuid.UUID)
	rows, err := a.db.GetTenantMembers(r.Context(), tenantID)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "hmm")
		return
	}

	var response TenantMembersResponse
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

	JsonResp(w, http.StatusOK, response)
}
