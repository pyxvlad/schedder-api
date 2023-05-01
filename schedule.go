package schedder

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type SetScheduleRequest struct {
	Weekday  string
	Starting time.Time
	Ending   time.Time
}

func (a *API) SetSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := ctx.Value(CtxAccountID).(uuid.UUID)
	request := ctx.Value(CtxJSON).(*SetScheduleRequest)
	weekdays := [...]database.Weekdays{
		database.WeekdaysMonday,
		database.WeekdaysTuesday,
		database.WeekdaysWednesday,
		database.WeekdaysThursday,
		database.WeekdaysFriday,
		database.WeekdaysSaturday,
		database.WeekdaysSunday,
	}

	found := false
	for _, v := range weekdays {
		if request.Weekday == string(v) {
			found = true
			break
		}
	}
	if !found {
		JsonError(w, http.StatusBadRequest, "invalid weekday")
		return
	}
	ssp := database.SetScheduleParams{
		AccountID:    accountID,
		Weekday:      database.Weekdays(request.Weekday),
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
