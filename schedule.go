package schedder

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type SetScheduleRequest struct {
	// Weekday represents the day of the week for which to set the schedule.
	// Valid values are: 0 (Sunday), 1 (Monday) ..., 6 (Saturday)
	Weekday  time.Weekday
	Starting time.Time
	Ending   time.Time
}

func (a *API) SetSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := ctx.Value(CtxAccountID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*SetScheduleRequest)

	if request.Weekday < time.Sunday || request.Weekday > time.Saturday  {
		JsonError(w, http.StatusBadRequest, "invalid weekday")
		return
	}
	ssp := database.SetScheduleParams{
		AccountID:    accountID,
		Weekday:      request.Weekday,
		StartingTime: request.Starting,
		EndingTime:   request.Ending,
	}

	err := a.db.SetSchedule(ctx, ssp)
	if err != nil {
		JsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
