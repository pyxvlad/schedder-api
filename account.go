package schedder

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
	"golang.org/x/crypto/bcrypt"
)

type PostAccountRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type PostAccountResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone,omitempty"`
}

type GenerateTokenResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Token     string    `json:"token"`
}

const BCRYPT_ROUNDS = 10

func (a *API) PostAccount(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var account_request PostAccountRequest
	err := decoder.Decode(&account_request)
	if err != nil {
		json_error(w, http.StatusBadRequest, "invalid json")
		return
	}

	var resp PostAccountResponse
	password_bytes, err := bcrypt.GenerateFromPassword([]byte(account_request.Password), BCRYPT_ROUNDS)

	if err != nil {
		json_error(w, http.StatusInternalServerError, err.Error())
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
		resp = PostAccountResponse{row.AccountID, row.Email.String, row.Phone.String}
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

		resp = PostAccountResponse{row.AccountID, row.Email.String, row.Phone.String}
	} else {
		json_error(w, http.StatusBadRequest, "expected phone or email")
		return
	}

	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.Encode(resp)
	return
}

func (a *API) GenerateToken(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var account_request PostAccountRequest
	err := decoder.Decode(&account_request)
	if err != nil {
		json_error(w, http.StatusBadRequest, "invalid json")
		return
	}

	var resp GenerateTokenResponse

	password := ""
	if account_request.Email != "" {
		row, err := a.db.GetPasswordByEmail(r.Context(), sql.NullString{String: account_request.Email, Valid: true})
		password = row.Password
		resp.AccountID = row.AccountID
		if err != nil {
			json_error(w, http.StatusBadRequest, "no user with email")
			return
		}
	} else if account_request.Phone != "" {
		row, err := a.db.GetPasswordByPhone(r.Context(), sql.NullString{String: account_request.Phone, Valid: true})
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

	err = bcrypt.CompareHashAndPassword([]byte(password), []byte(account_request.Password))
	if err != nil {
		json_error(w, http.StatusBadRequest, "invalid password")
		return
	}

	token, err := a.db.CreateSessionToken(r.Context(), resp.AccountID)
	resp.Token = base64.RawStdEncoding.EncodeToString(token)
	if err != nil {
		json_error(w, http.StatusInternalServerError, "couldn't generate token"+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.Encode(resp)
	return
}
