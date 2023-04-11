package schedder

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

// VerifyCodeRequest represents a request to verify a code.
type VerifyCodeRequest struct {
	// Email represents the email of the user.
	Email string `json:"email,omitempty"`
	// Phone represents the phone number of the user.
	Phone string `json:"phone,omitempty"`
	// Code represents the code that needs to be verified.
	Code string `json:"code"`
}

// VerifyCodeResponse represents the response to the VerifyCode endpoint.
type VerifyCodeResponse struct {
	Response
	// Email represents the email of the user.
	Email string `json:"email,omitempty"`
	// Phone represents the phone number of the user.
	Phone string `json:"phone,omitempty"`
	// Scope represents the use of the code, i.e. register, magic login, etc.
	Scope string `json:"scope"`
}

func findAccountByEmailOrPhone(
	ctx context.Context, queries *database.Queries, email, phone string,
) (accountID uuid.UUID, errorMessage string) {
	var account database.Account
	var err error

	if email == "" && phone == "" {
		return uuid.Nil, "missing email and phone"
	}

	if email != "" {
		// TODO: I need just the account ID, not the whole thing
		account, err = queries.FindAccountByEmail(
			ctx, sql.NullString{String: email, Valid: true},
		)
		if err != nil {
			return uuid.Nil, "invalid email"
		}
	} else if phone != "" {
		// TODO: I need just the account ID, not the whole thing
		account, err = queries.FindAccountByPhone(
			ctx, sql.NullString{String: phone, Valid: true},
		)
		if err != nil {
			return uuid.Nil, "invalid phone"
		}
	}

	return account.AccountID, ""
}

// VerifyCode verifies a code and returns it's scope.
func (a *API) VerifyCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	request := ctx.Value(CtxJSON).(*VerifyCodeRequest)

	var params database.GetVerificationCodeScopeParams

	// findAccountByEmailOrPhone :=
	var errorMessage string

	params.AccountID, errorMessage = findAccountByEmailOrPhone(
		ctx, a.db, request.Email, request.Phone,
	)
	if params.AccountID == uuid.Nil || errorMessage != "" {
		jsonError(w, http.StatusBadRequest, errorMessage)
		return
	}

	params.VerificationCode = request.Code
	scope, err := a.db.GetVerificationCodeScope(ctx, params)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid code")
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

// Verifier represents something that can send a verification code to an ID
// like email or phone number.
type Verifier interface {
	// SendVerification sends the verification code to the id.
	SendVerification(id, code string) error
}

// WriterVerifier is a Verifier that instead of actually sending the code
// writes it in the console, or into a file.
type WriterVerifier struct {
	// Writer represents where to write the message, like stdout or a file.
	Writer io.Writer
	// Kind represents the kind of the verification, like Email or Phone.
	Kind string
}

// SendVerification writes a message containing the code to the internal
// writer.
func (v *WriterVerifier) SendVerification(id, code string) error {
	_, err := fmt.Fprintf(
		v.Writer, "DEVELOPMENT: Verify %s %s using %s\n", v.Kind, id, code,
	)
	return err
}
