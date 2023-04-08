package schedder

import (
	"database/sql"
	"net/http"

	"gitlab.com/vlad.anghel/schedder-api/database"
)

type VerifyCodeRequest struct {
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	Code string `json:"code"`
}

type VerifyCodeResponse struct {
	Response
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	Scope string `json:"scope"`
}

func (a *API) VerifyCode(w http.ResponseWriter, r * http.Request) {
	ctx := r.Context()
	request := ctx.Value(CtxJSON).(*VerifyCodeRequest)

	var params database.GetVerificationCodeScopeParams

	if request.Email != "" {
		// TODO: I need just the account ID, not the whole thing
		account, err := a.db.FindAccountByEmail(ctx, sql.NullString{String: request.Email, Valid: true})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		params.AccountID = account.AccountID
	} else if request.Phone != "" {
		// TODO: I need just the account ID, not the whole thing
		account, err := a.db.FindAccountByPhone(ctx, sql.NullString{String: request.Phone, Valid: true})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		params.AccountID = account.AccountID
	} else {
		jsonError(w, http.StatusBadRequest, "missing email and phone")
		return
	}

	params.VerificationCode = request.Code
	scope, err := a.db.GetVerificationCodeScope(ctx, params)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	switch scope {
	case database.VerificationScopeRegister:
		err = a.db.ActivateAccount(ctx, params.AccountID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
	default:
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	var response VerifyCodeResponse
	response.Email = request.Email
	response.Phone = request.Phone
	response.Scope = string(scope)

	jsonResp(w, http.StatusOK, response)
}
