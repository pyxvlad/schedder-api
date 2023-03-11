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

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"gitlab.com/vlad.anghel/schedder-api/database"
	"golang.org/x/crypto/bcrypt"
)

type PostAccountRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
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

const BCRYPT_ROUNDS = 10

func (a *API) PostAccount(w http.ResponseWriter, r *http.Request) {
	account_request := r.Context().Value("json").(*PostAccountRequest)
	raw_password := []byte(account_request.Password)

	if (len(raw_password) < 8) || (len(raw_password) > 64) {
		json_error(w, http.StatusBadRequest, "password too short")
		return
	}

	var resp PostAccountResponse
	password_bytes, err := bcrypt.GenerateFromPassword([]byte(account_request.Password), BCRYPT_ROUNDS)
	if err != nil {
		json_error(w, http.StatusInternalServerError, err.Error())
		return
	}

	password := string(password_bytes)

	if account_request.Email != "" {
		_, err := mail.ParseAddress(account_request.Email)
		if err != nil {
			json_error(w, http.StatusBadRequest, err.Error())
			return
		}

		cawep := database.CreateAccountWithEmailParams{
			Email:    sql.NullString{String: account_request.Email, Valid: true},
			Password: password,
		}
		row, err := a.db.CreateAccountWithEmail(r.Context(), cawep)
		if err != nil {
			json_error(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String
	} else if account_request.Phone != "" {
		phone := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) || r == '+' {
				return r
			} else {
				return -1
			}
		}, account_request.Phone)

		if !strings.HasPrefix(phone, "+") && (strings.HasPrefix(phone, "07") || strings.HasPrefix(phone, "02")) {
			phone = "+4" + phone
		}

		if len(phone) != 12 {
			json_error(w, http.StatusBadRequest, "phone too short/long")
			return
		}

		cawpp := database.CreateAccountWithPhoneParams{
			Phone:    sql.NullString{String: phone, Valid: true},
			Password: password,
		}
		row, err := a.db.CreateAccountWithPhone(r.Context(), cawpp)
		if err != nil {
			json_error(w, http.StatusInternalServerError, err.Error())
		}

		resp.AccountID = row.AccountID
		resp.Email = row.Email.String
		resp.Phone = row.Phone.String
	} else {
		json_error(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	json_resp(w, http.StatusCreated, resp)
	return
}

func (a *API) GenerateToken(w http.ResponseWriter, r *http.Request) {
	token_request := r.Context().Value("json").(*GenerateTokenRequest)

	var ip pgtype.Inet
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		err := ip.Scan(forwarded)
		if err != nil {
			json_error(w, http.StatusBadRequest, "invalid X-FORWARDED-FOR")
			return
		}
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if err := ip.Scan(host); err != nil {
			json_error(w, http.StatusBadRequest, "invalid IP, wtf")
			panic("bro, wtf happened here: " + err.Error())
		}
	}

	if len(token_request.Device) < 8 {
		json_error(w, http.StatusBadRequest, "device name too short")
		return
	}

	var resp GenerateTokenResponse
	password := ""
	if token_request.Email != "" {
		row, err := a.db.GetPasswordByEmail(r.Context(), sql.NullString{String: token_request.Email, Valid: true})
		password = row.Password
		resp.AccountID = row.AccountID
		if err != nil {
			json_error(w, http.StatusBadRequest, "no user with email")
			return
		}
	} else if token_request.Phone != "" {
		row, err := a.db.GetPasswordByPhone(r.Context(), sql.NullString{String: token_request.Phone, Valid: true})
		password = row.Password
		resp.AccountID = row.AccountID
		if err != nil {
			json_error(w, http.StatusBadRequest, "no user with phone")
			return
		}
	} else {
		json_error(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(password), []byte(token_request.Password))
	if err != nil {
		json_error(w, http.StatusBadRequest, "invalid password")
		return
	}

	//token, err := a.db.CreateSessionToken(r.Context(), resp.AccountID)
	token, err := a.db.CreateSessionToken(r.Context(), database.CreateSessionTokenParams{AccountID: resp.AccountID, Ip: ip, Device: token_request.Device})
	if err != nil {
		json_error(w, http.StatusInternalServerError, "couldn't generate token"+err.Error())
		return
	}

	resp.Token = base64.RawStdEncoding.EncodeToString(token)
	json_resp(w, http.StatusCreated, resp)
	return
}

func (a *API) GetSessionsForAccount(w http.ResponseWriter, r *http.Request) {
	account_id := r.Context().Value("account_id").(uuid.UUID)

	rows, err := a.db.GetSessionsForAccount(r.Context(), account_id)
	if err != nil {
		json_error(w, http.StatusInternalServerError, "couldn't get sessions")
		return
	}

	var resp GetSessionsResponse

	resp.Sessions = make([]sessionResponse, 0)

	for _, r := range rows {
		resp.Sessions = append(resp.Sessions, sessionResponse{r.SessionID, r.ExpirationDate, r.Ip.IPNet.IP, r.Device})
	}

	json_resp(w, http.StatusOK, resp)
}

func (a *API) RevokeSession(w http.ResponseWriter, r *http.Request) {
	account_id := r.Context().Value("account_id").(uuid.UUID)
	session_id := r.Context().Value("session_id").(uuid.UUID)

	affected_rows, err := a.db.RevokeSessionForAccount(r.Context(), database.RevokeSessionForAccountParams{SessionID: session_id, AccountID: account_id})
	if affected_rows != 1 {
		json_error(w, http.StatusBadRequest, "invalid session")
		return
	}
	if err != nil {
		json_error(w, http.StatusInternalServerError, "couldn't revoke session")
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}
