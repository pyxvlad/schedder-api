package schedder

import (
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type FavouritesResponse struct {
	Response
	TenantIDs []uuid.UUID `json:"tenant_ids"`
}

func (a *API) AddFavourite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	params := database.AddFavouriteParams{
		TenantID:  tenantID,
		AccountID: authenticatedID,
	}
	err := a.db.AddFavourite(ctx, params)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *API) Favourites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)

	favourites, err := a.db.GetFavourites(ctx, authenticatedID)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	var response FavouritesResponse
	response.TenantIDs = favourites
	JsonResp(w, http.StatusOK, response)
}

func (a *API) RemoveFavourite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	params := database.RemoveFavouriteParams{
		TenantID:  tenantID,
		AccountID: authenticatedID,
	}
	err := a.db.RemoveFavourite(ctx, params)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	w.WriteHeader(http.StatusOK)
}
