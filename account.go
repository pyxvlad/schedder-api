package schedder

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"gitlab.com/vlad.anghel/schedder-api/database"
	"golang.org/x/crypto/bcrypt"
)

// AccountCreationRequest represents the parameters that the account creation
// endpoint expects.
type AccountCreationRequest struct {
	// Email for the newly created account, i.e. user@example.com
	Email string `json:"email,omitempty"`
	// Phone number as a string, i.e. "+40743123123"
	Phone string `json:"phone,omitempty"`
	// The password that the user wants to use
	Password string `json:"password"`
}

// AccountCreationResponse represents the response that the account creation
// endpoint returns back.
type AccountCreationResponse struct {
	Response
	// AccountID represents the ID of the account just created.
	AccountID uuid.UUID `json:"account_id"`
	// Email represents the email the user used to sign up, otherwise it's
	// omitted.
	Email string `json:"email,omitempty"`
	// Phone represents the phone the user used to sign up, otherwise it's
	// omitted.
	Phone string `json:"phone,omitempty"`
}

// TokenGenerationResponse represents the response that the token generation
// endpoint returns back.
type TokenGenerationResponse struct {
	Response
	// AccountID represents the ID of the account for which the token was
	// generated.
	AccountID uuid.UUID `json:"account_id"`
	// Token represents the token generated, it MUST be used for all endpoints
	// that require authentication.
	Token string `json:"token"`
}

// TokenGenerationRequest represents the parameters that the token generation
// endpoint expects.
type TokenGenerationRequest struct {
	// Email represents the email of the user that wants a new session token.
	Email string `json:"email,omitempty"`
	// Phone represents the phone of the user that wants a new session token.
	Phone string `json:"phone,omitempty"`
	// Password represents the user's password.
	Password string `json:"password"`
	// Device represents the name of the frontend used, for example
	// "Schedder Flutter Android 6.6" or "Schedder Angular 4.2".
	Device string `json:"device"`
}

// sessionResponse represents a session.
type sessionResponse struct {
	// SessionID is the ID of the session.
	SessionID uuid.UUID `json:"session_id"`
	// ExpirationDate is the datetime when the session will expire.
	ExpirationDate time.Time `json:"expiration_date"`
	// IP is the IPv4 or IPv6 used for creating the session.
	IP net.IP `json:"ip"`
	// Device is the device used when creating the session.
	Device string `json:"device"`
}

// SessionsForAccountResponse represents a list of active sessions.
type SessionsForAccountResponse struct {
	Response
	// Sessions is the list of sessions.
	Sessions []sessionResponse `json:"sessions"`
}

// AccountByEmailAsAdminResponse represents an account from the viewpoint of
// an admin. Currently it used for AccountByEmailAsAdmin, thus the name.
// In the future when it will be used for other things it will be renamed.
type AccountByEmailAsAdminResponse struct {
	Response
	// AccountID represents the ID of the user.
	AccountID uuid.UUID `json:"account_id"`
	// Email represents the email of the user.
	Email string `json:"email,omitempty"`
	// Phone represents the phone number of the user.
	Phone string `json:"phone,omitempty"`
	// IsBusiness represents whether this is a business account.
	IsBusiness bool `json:"is_business"`
	// IsAdmin represents whether this is an admin account.
	IsAdmin bool `json:"is_admin"`
}

// AdminSettingRequest represents a request for setting an user's admin
// property.
type AdminSettingRequest struct {
	// Admin represents the new admin status of the account.
	Admin bool `json:"admin"`
}

// BusinessSettingRequest represents a request for setting an user's business
// property.
type BusinessSettingRequest struct {
	// Business represents the new business state of the account.
	Business bool `json:"business"`
}

// CreateAccount is the endpoint used for creating new user accounts.
func (a *API) CreateAccount(w http.ResponseWriter, r *http.Request) {
	// TODO: move this outside, but where?
	generateVerificationCode := func() (string, error) {
		const (
			max = 1000000
			min = 100000
		)
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(max-min))
		if err != nil {
			return "", err
		}
		i := randomNumber.Int64() + min
		return fmt.Sprint(i), nil
	}
	ctx := r.Context()
	request := ctx.Value(CtxJSON).(*AccountCreationRequest)
	rawPassword := []byte(request.Password)

	if (len(rawPassword) < 8) || (len(rawPassword) > 64) {
		jsonError(w, http.StatusBadRequest, "password too short")
		return
	}

	var resp AccountCreationResponse
	passwordBytes, err := bcrypt.GenerateFromPassword(
		[]byte(request.Password), BcryptRounds,
	)

	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	password := string(passwordBytes)
	if request.Email == "" && request.Phone == "" {
		jsonError(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	if request.Email != "" {
		_, err := mail.ParseAddress(request.Email)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid email")
			return
		}

		tx, err := a.txlike.Begin(ctx)
		defer tx.Rollback(ctx)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}

		queries := database.New(tx)

		cawep := database.CreateAccountWithEmailParams{
			Email:    sql.NullString{String: request.Email, Valid: true},
			Password: password,
		}

		row, err := queries.CreateAccountWithEmail(ctx, cawep)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "email already used")
			return
		}
		code, err := generateVerificationCode()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		cvcp := database.CreateVerificationCodeParams{
			AccountID:        row.AccountID,
			VerificationCode: code,
			Scope:            database.VerificationScopeRegister,
		}
		err = queries.CreateVerificationCode(ctx, cvcp)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String

		// assume that if the verification cannot be sended the user gave an
		// invalid email
		// TODO: refactor this so we also know when the verification service
		// is down
		err = a.emailVerifier.SendVerification(request.Email, code)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid email")
			return
		}
		tx.Commit(ctx)
	} else if request.Phone != "" {
		phone := strings.Map(func(r rune) rune {
			if r == '+' ||  unicode.IsDigit(r) {
				return r
			}
			return -1
		}, request.Phone)

		hasPlus := strings.HasPrefix(phone, "+")
		hasMobilePrefix := strings.HasPrefix(phone, "07")
		hasTelephonePrefix := strings.HasPrefix(phone, "02")

		if !hasPlus && (hasMobilePrefix || hasTelephonePrefix) {
			phone = "+4" + phone
		}

		if len(phone) != PhoneLength {
			jsonError(w, http.StatusBadRequest, "phone too short/long")
			return
		}

		tx, err := a.txlike.Begin(ctx)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
		defer tx.Rollback(ctx)
		queries := database.New(tx)

		cawpp := database.CreateAccountWithPhoneParams{
			Phone:    sql.NullString{String: phone, Valid: true},
			Password: password,
		}
		row, err := queries.CreateAccountWithPhone(ctx, cawpp)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "phone already used")
			return
		}

		code, err := generateVerificationCode()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}

		cvcp := database.CreateVerificationCodeParams{
			AccountID:        row.AccountID,
			VerificationCode: code,
			Scope:            database.VerificationScopeRegister,
		}
		err = queries.CreateVerificationCode(ctx, cvcp)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}

		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String

		// Assume that if the verification cannot be sended the user gave an
		// invalid email.
		// TODO: refactor this so we also know when the verification service is
		// down.
		err = a.phoneVerifier.SendVerification(request.Phone, code)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid phone")
			return
		}

		err = tx.Commit(ctx)
		if err != nil {
			jsonError(
				w, http.StatusInternalServerError, "couldn't create account",
			)
			return
		}
	}

	jsonResp(w, http.StatusCreated, resp)
}

func getIPFromRequest(r *http.Request) (pgtype.Inet, error) {
	var address pgtype.Inet
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		err := address.Scan(forwarded)
		if err != nil {
			return address, errors.New("invalid X-FORWARDED-FOR")
		}
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if err := address.Scan(host); err != nil {
			return address, errors.New("invalid IP, wtf")
		}
	}
	return address, nil
}
func getPassword(ctx context.Context, db *database.Queries, email, phone string) (string, uuid.UUID, error) {
	if email == "" && phone == "" {
		return "", uuid.Nil, errors.New("expected phone or email")
	}
	if email != "" {
		var row database.GetPasswordByEmailRow
		row, err := db.GetPasswordByEmail(
			ctx,
			sql.NullString{String: email, Valid: true},
		)
		if err != nil {
			return "", uuid.Nil, errors.New("no user with email")
		}
		return row.Password, row.AccountID, nil
	} else if phone != "" {
		var row database.GetPasswordByPhoneRow
		row, err := db.GetPasswordByPhone(
			ctx,
			sql.NullString{String: phone, Valid: true},
		)
		if err != nil {
			return "", uuid.Nil, errors.New("no user with phone")
		}
		return row.Password, row.AccountID, nil
	}
	panic(
		"impossible case logically: Email and Password have been checked",
	)
}

// GenerateToken creates a new token for the user.
func (a *API) GenerateToken(w http.ResponseWriter, r *http.Request) {
	tokenRequest := r.Context().Value(CtxJSON).(*TokenGenerationRequest)

	address, err := getIPFromRequest(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(tokenRequest.Device) < minimumLengthForDevice {
		jsonError(w, http.StatusBadRequest, "device name too short")
		return
	}

	var resp TokenGenerationResponse
	var password string

	password, resp.AccountID, err = getPassword(
		r.Context(), a.db, tokenRequest.Email, tokenRequest.Phone,
	)

	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(password), []byte(tokenRequest.Password),
	)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid password")
		return
	}

	cstp := database.CreateSessionTokenParams{
		AccountID: resp.AccountID,
		Ip:        address,
		Device:    tokenRequest.Device,
	}
	token, err := a.db.CreateSessionToken(r.Context(), cstp)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't generate token")
		return
	}

	resp.Token = base64.RawStdEncoding.EncodeToString(token)
	jsonResp(w, http.StatusCreated, resp)
}

// SessionsForAccount lists the sessions for an account.
func (a *API) SessionsForAccount(w http.ResponseWriter, r *http.Request) {
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)

	rows, err := a.db.GetSessionsForAccount(r.Context(), authenticatedID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't get sessions")
		return
	}

	var resp SessionsForAccountResponse

	resp.Sessions = make([]sessionResponse, 0, len(rows))

	for _, r := range rows {
		session := sessionResponse{
			r.SessionID, r.ExpirationDate, r.Ip.IPNet.IP, r.Device,
		}
		resp.Sessions = append(resp.Sessions, session)
	}

	jsonResp(w, http.StatusOK, resp)
}

// RevokeSession revokes a session for the account.
func (a *API) RevokeSession(w http.ResponseWriter, r *http.Request) {
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	sessionID := r.Context().Value(CtxSessionID).(uuid.UUID)
	rsfap := database.RevokeSessionForAccountParams{
		SessionID: sessionID,
		AccountID: authenticatedID,
	}
	affectedRows, err := a.db.RevokeSessionForAccount(r.Context(), rsfap)
	if affectedRows != 1 {
		jsonError(w, http.StatusBadRequest, "invalid session")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't revoke session")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// AccountByEmailAsAdmin find and returns an account with admin access control.
func (a *API) AccountByEmailAsAdmin(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")

	account, err := a.db.FindAccountByEmail(
		r.Context(), sql.NullString{String: email, Valid: true},
	)
	if err != nil {
		jsonError(w, http.StatusNotFound, "invalid email")
		return
	}

	var resp AccountByEmailAsAdminResponse
	resp.AccountID = account.AccountID
	if account.Email.Valid {
		resp.Email = account.Email.String
	}
	if account.Phone.Valid {
		resp.Phone = account.Phone.String
	}
	resp.IsAdmin = account.IsAdmin
	resp.IsBusiness = account.IsBusiness

	jsonResp(w, http.StatusOK, resp)
}

// SetAdmin sets whether an user is an admin.
func (a *API) SetAdmin(w http.ResponseWriter, r *http.Request) {
	// TODO: use this for logs once you setup a logging system
	// authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	ctx := r.Context()
	accountID := ctx.Value(CtxAccountID).(uuid.UUID)

	json := ctx.Value(CtxJSON).(*AdminSettingRequest)
	safap := database.SetAdminForAccountParams{
		AccountID: accountID,
		IsAdmin:   json.Admin,
	}
	err := a.db.SetAdminForAccount(ctx, safap)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hmm")
	}

	w.WriteHeader(http.StatusOK)
}

// SetBusiness sets whether an account is a business account.
func (a *API) SetBusiness(w http.ResponseWriter, r *http.Request) {
	// TODO: use this for logs once you setup a logging system
	// authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	ctx := r.Context()
	accountID := ctx.Value(CtxAccountID).(uuid.UUID)

	json := ctx.Value(CtxJSON).(*BusinessSettingRequest)

	sbfap := database.SetBusinessForAccountParams{
		AccountID:  accountID,
		IsBusiness: json.Business,
	}
	err := a.db.SetBusinessForAccount(ctx, sbfap)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hmm")
	}

	w.WriteHeader(http.StatusOK)
}
