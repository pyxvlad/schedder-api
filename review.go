package schedder

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type CreateReviewRequest struct {
	Message string `json:"message"`
	Rating  int    `json:"rating"`
}

type reviewResponseEntry struct {
	ReviewID  uuid.UUID `json:"review_id"`
	AccountID uuid.UUID `json:"account_id"`
	Rating    int       `json:"rating"`
	Message   string    `json:"message"`
}
type ReviewsResponse struct {
	Response
	Reviews []reviewResponseEntry `json:"reviews"`
}

func (a *API) CreateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*CreateReviewRequest)

	if request.Rating < 0 || request.Rating > 5 {
		JsonError(w, http.StatusBadRequest, "invalid rating")
		return
	}

	crp := database.CreateReviewParams{AccountID: authenticatedID,
		TenantID: tenantID,
		Message:  request.Message,
		Rating:   int32(request.Rating),
	}
	err := a.db.CreateReview(ctx, crp)
	if err != nil {
		JsonError(w, http.StatusBadRequest, "couldn't create review")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *API) Reviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)

	reviews, err := a.db.Reviews(ctx, tenantID)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		JsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	var response ReviewsResponse
	response.Reviews = make([]reviewResponseEntry, 0, len(reviews))
	for i := range reviews {
		review := &reviews[i]
		response.Reviews = append(response.Reviews, reviewResponseEntry{
			ReviewID:  review.ReviewID,
			AccountID: review.AccountID,
			Rating:    int(review.Rating),
			Message:   review.Message,
		})
	}

	JsonResp(w, http.StatusOK, response)
}
