package schedder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api/database"
)

type AddTenantPhotoResponse struct {
	Response
	PhotoID uuid.UUID `json:"photo_id"`
}

type ListTenantPhotosResponse struct {
	Response
	Photos []uuid.UUID `json:"photo_ids"`
}

func (a *API) AddTenantPhoto(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)

	part := make([]byte, 512)
	n, err := r.Body.Read(part)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	if n != 512 {
		jsonError(w, http.StatusBadRequest, "invalid image")
		return
	}

	content := http.DetectContentType(part)

	reader := io.MultiReader(bytes.NewReader(part), r.Body)

	if content != "image/jpeg" && content != "image/png" {
		jsonError(w, http.StatusBadRequest, "invalid image")
		return
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	checksum := sha256.Sum256(data)

	atpp := database.AddTenantPhotoParams{
		Sha256sum: checksum[:],
		TenantID:  tenantID,
	}

	photoID, err := a.db.AddTenantPhoto(ctx, atpp)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	count, err := a.db.CountPhotosWithHash(ctx, checksum[:])
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	if count == 1 {
		encoded := hex.EncodeToString(checksum[:])
		os.WriteFile(a.photosPath+encoded, data, 0777)
	}

	jsonResp(w, http.StatusCreated, AddTenantPhotoResponse{PhotoID: photoID})
}

func (a *API) ListTenantPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)

	photos, err := a.db.ListTenantPhotos(ctx, tenantID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	var response ListTenantPhotosResponse
	response.Photos = photos

	jsonResp(w, http.StatusOK, response)
}

func (a *API) DownloadTenantPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	photoIDParameter := chi.URLParam(r, "photoID")
	photoID, err := uuid.Parse(photoIDParameter)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid photoID")
		return
	}

	gtphp := database.GetTenantPhotoHashParams{
		TenantID: tenantID, PhotoID: photoID,
	}

	hash, err := a.db.GetTenantPhotoHash(ctx, gtphp)
	if err != nil {
		jsonError(w, http.StatusNotFound, "invalid photoID")
		return
	}

	file, err := os.Open(a.photosPath + hex.EncodeToString(hash))
	if err != nil {
		// Here we return InternalServerError because if the photo is missing
		// from the filesystem clearly there are some other issues.
		jsonError(w, http.StatusInternalServerError, "invalid photo hash")
		return
	}

	stat, err := file.Stat()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	written, err := io.Copy(w, file)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	if written != stat.Size() {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
}

func (a *API) DeleteTenantPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctx.Value(CtxTenantID).(uuid.UUID)
	photoID := ctx.Value(CtxPhotoID).(uuid.UUID)
	dtpp := database.DeleteTenantPhotoParams{
		PhotoID: photoID,
		TenantID: tenantID,
	}
	hash, err := a.db.DeleteTenantPhoto(ctx, dtpp)
	if err != nil {
		jsonError(w, http.StatusNotFound, "no photo")
		return
	}

	count, err := a.db.CountPhotosWithHash(ctx, hash)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	if count == 0 {
		encoded := hex.EncodeToString(hash)
		err = os.Remove(a.photosPath + encoded)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
func (a *API) SetProfilePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	part := make([]byte, 512)
	n, err := r.Body.Read(part)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	if n != 512 {
		jsonError(w, http.StatusBadRequest, "invalid image")
		return
	}

	content := http.DetectContentType(part)

	reader := io.MultiReader(bytes.NewReader(part), r.Body)

	if content != "image/jpeg" && content != "image/png" {
		jsonError(w, http.StatusBadRequest, "invalid image")
		return
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	checksum := sha256.Sum256(data)
	args := database.SetProfilePhotoParams{
		AccountID: authenticatedID,
		Sha256sum: checksum[:],
	}

	err = a.db.SetProfilePhoto(ctx, args)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	count, err := a.db.CountPhotosWithHash(ctx, checksum[:])
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
	if count == 1 {
		encoded := hex.EncodeToString(checksum[:])
		os.WriteFile(a.photosPath+encoded, data, 0777)
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) DownloadProfilePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)

	hash, err := a.db.GetProfilePhotoHash(ctx, authenticatedID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "no photo")
		return
	}

	encoded := hex.EncodeToString(hash)
	file, err := os.Open(a.photosPath + encoded)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	stat, err := file.Stat()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	written, err := io.Copy(w, file)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	if written != stat.Size() {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}
}

func (a *API) DeleteProfilePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authenticatedID := ctx.Value(CtxAuthenticatedID).(uuid.UUID)
	hash, err := a.db.DeleteProfilePhoto(ctx, authenticatedID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "no photo")
		return
	}

	count, err := a.db.CountPhotosWithHash(ctx, hash)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "not implemented")
		return
	}

	if count == 0 {
		encoded := hex.EncodeToString(hash)
		err = os.Remove(a.photosPath + encoded)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "not implemented")
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
