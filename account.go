package schedder

import (
	"database/sql"
	"encoding/base64"
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

// PostAccountRequest represents the parameters that the account creation
// endpoint expects.
type PostAccountRequest struct {
	// Email for the newly created account, i.e. user@example.com
	Email string `json:"email"`
	// Phone number as a string, i.e. "+40743123123"
	Phone string `json:"phone"`
	// The password that the user wants to use
	Password string `json:"password"`
}

type PostAccountResponse struct {
	Response
	AccountID uuid.UUID `json:"account_id"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone,omitempty"`
}

type GenerateTokenResponse struct {
	Response
	AccountID uuid.UUID `json:"account_id"`
	Token     string    `json:"token"`
}

type GenerateTokenRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Device   string `json:"device"`
}

type sessionResponse struct {
	ID             uuid.UUID `json:"session_id"`
	ExpirationDate time.Time `json:"expiration_date"`
	IP             net.IP    `json:"ip"`
	Device         string    `json:"device"`
}

type GetSessionsResponse struct {
	Response
	Sessions []sessionResponse `json:"sessions"`
}

type GetAccountByEmailAsAdminResponse struct {
	Response
	AccountID uuid.UUID `json:"account_id"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	IsBusiness bool `json:"is_business"`
	IsAdmin bool `json:"is_admin"`
}

type SetAdminRequest struct {
	Admin bool `json:"admin"`
}

type SetBusinessRequest struct {
	Business bool `json:"business"`
}

const BcryptRounds = 10

func (a *API) PostAccount(w http.ResponseWriter, r *http.Request) {
	accountRequest := r.Context().Value(CtxJSON).(*PostAccountRequest)
	rawPassword := []byte(accountRequest.Password)

	if (len(rawPassword) < 8) || (len(rawPassword) > 64) {
		jsonError(w, http.StatusBadRequest, "password too short")
		return
	}

	var resp PostAccountResponse
	passwordBytes, err := bcrypt.GenerateFromPassword([]byte(accountRequest.Password), BcryptRounds)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	password := string(passwordBytes)

	if accountRequest.Email != "" {
		_, err := mail.ParseAddress(accountRequest.Email)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid email")
			return
		}

		cawep := database.CreateAccountWithEmailParams{
			Email:    sql.NullString{String: accountRequest.Email, Valid: true},
			Password: password,
		}
		row, err := a.db.CreateAccountWithEmail(r.Context(), cawep)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "email already used")
			return
		}
		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String
	} else if accountRequest.Phone != "" {
		phone := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) || r == '+' {
				return r
			}
			return -1
		}, accountRequest.Phone)

		if !strings.HasPrefix(phone, "+") && (strings.HasPrefix(phone, "07") || strings.HasPrefix(phone, "02")) {
			phone = "+4" + phone
		}

		if len(phone) != 12 {
			jsonError(w, http.StatusBadRequest, "phone too short/long")
			return
		}

		cawpp := database.CreateAccountWithPhoneParams{
			Phone:    sql.NullString{String: phone, Valid: true},
			Password: password,
		}
		row, err := a.db.CreateAccountWithPhone(r.Context(), cawpp)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "phone already used")
			return
		}

		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String
	} else {
		jsonError(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	jsonResp(w, http.StatusCreated, resp)
}

func (a *API) GenerateToken(w http.ResponseWriter, r *http.Request) {
	tokenRequest := r.Context().Value(CtxJSON).(*GenerateTokenRequest)

	var ip pgtype.Inet
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		err := ip.Scan(forwarded)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid X-FORWARDED-FOR")
			return
		}
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if err := ip.Scan(host); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid IP, wtf")
			panic("bro, wtf happened here: " + err.Error())
		}
	}

	if len(tokenRequest.Device) < 8 {
		jsonError(w, http.StatusBadRequest, "device name too short")
		return
	}

	var resp GenerateTokenResponse
	password := ""
	if tokenRequest.Email != "" {
		row, err := a.db.GetPasswordByEmail(r.Context(), sql.NullString{String: tokenRequest.Email, Valid: true})
		password = row.Password
		resp.AccountID = row.AccountID
		if err != nil {
			jsonError(w, http.StatusBadRequest, "no user with email")
			return
		}
	} else if tokenRequest.Phone != "" {
		row, err := a.db.GetPasswordByPhone(r.Context(), sql.NullString{String: tokenRequest.Phone, Valid: true})
		password = row.Password
		resp.AccountID = row.AccountID
		if err != nil {
			jsonError(w, http.StatusBadRequest, "no user with phone")
			return
		}
	} else {
		jsonError(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(password), []byte(tokenRequest.Password))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid password")
		return
	}

	//token, err := a.db.CreateSessionToken(r.Context(), resp.AccountID)
	token, err := a.db.CreateSessionToken(r.Context(), database.CreateSessionTokenParams{AccountID: resp.AccountID, Ip: ip, Device: tokenRequest.Device})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't generate token"+err.Error())
		return
	}

	resp.Token = base64.RawStdEncoding.EncodeToString(token)
	jsonResp(w, http.StatusCreated, resp)
}

func (a *API) GetSessionsForAccount(w http.ResponseWriter, r *http.Request) {
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)

	rows, err := a.db.GetSessionsForAccount(r.Context(), authenticatedID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't get sessions")
		return
	}

	var resp GetSessionsResponse

	resp.Sessions = make([]sessionResponse, 0, len(rows))

	for _, r := range rows {
		resp.Sessions = append(resp.Sessions, sessionResponse{r.SessionID, r.ExpirationDate, r.Ip.IPNet.IP, r.Device})
	}

	jsonResp(w, http.StatusOK, resp)
}

func (a *API) RevokeSession(w http.ResponseWriter, r *http.Request) {
	authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	sessionID := r.Context().Value(CtxSessionID).(uuid.UUID)

	affectedRows, err := a.db.RevokeSessionForAccount(r.Context(), database.RevokeSessionForAccountParams{SessionID: sessionID, AccountID: authenticatedID})
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

func (a *API) GetAccountByEmailAsAdmin(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")

	account, err := a.db.FindAccountByEmail(r.Context(), sql.NullString{String: email, Valid: true})
	if err != nil {
		jsonError(w, http.StatusNotFound, "invalid email")
		return
	}

	var resp GetAccountByEmailAsAdminResponse
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

func (a *API) SetAdmin(w http.ResponseWriter, r *http.Request) {
	// TODO: use this for logs once you setup a logging system
	// authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	accountID := r.Context().Value(CtxAccountID).(uuid.UUID)

	json := r.Context().Value(CtxJSON).(*SetAdminRequest)
	
	err := a.db.SetAdminForAccount(r.Context(), database.SetAdminForAccountParams{AccountID: accountID, IsAdmin: json.Admin})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hmm")
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) SetBusiness(w http.ResponseWriter, r *http.Request) {
	// TODO: use this for logs once you setup a logging system
	// authenticatedID := r.Context().Value(CtxAuthenticatedID).(uuid.UUID)
	accountID := r.Context().Value(CtxAccountID).(uuid.UUID)

	json := r.Context().Value(CtxJSON).(*SetBusinessRequest)
	
	err := a.db.SetBusinessForAccount(r.Context(), database.SetBusinessForAccountParams{AccountID: accountID, IsBusiness: json.Business})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hmm")
	}

	w.WriteHeader(http.StatusOK)
}
