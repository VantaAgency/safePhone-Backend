package handler

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

type UpdateCommercialStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=active inactive"`
}

type UpdateCommercialCommissionRequest struct {
	CommissionPercentage float64 `json:"commission_percentage" validate:"required"`
}

// CommercialHandler handles commercial acquisition routes.
type CommercialHandler struct {
	svc      *service.CommercialService
	validate *validator.Validate
}

func NewCommercialHandler(svc *service.CommercialService) *CommercialHandler {
	return &CommercialHandler{svc: svc, validate: validator.New()}
}

func (h *CommercialHandler) ReferralDetails(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	code := chi.URLParam(r, "code")
	profile, appErr := h.svc.GetReferralDetails(r.Context(), ac.OrgID, code)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, profile)
}

func (h *CommercialHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	overview, appErr := h.svc.GetOverview(r.Context(), ac)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, overview)
}

func (h *CommercialHandler) ListPartners(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	items, appErr := h.svc.ListPartners(r.Context(), ac, parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, items)
}

func (h *CommercialHandler) ListCommissions(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	items, appErr := h.svc.ListCommissions(r.Context(), ac, parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, items)
}

func (h *CommercialHandler) ListActivityReports(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	var partnerID *uuid.UUID
	if raw := r.URL.Query().Get("partner_id"); raw != "" {
		parsed, appErr := parseUUIDParam(raw, "invalid partner_id")
		if appErr != nil {
			WriteError(w, r, appErr)
			return
		}
		partnerID = &parsed
	}
	items, appErr := h.svc.ListActivityReports(r.Context(), ac, partnerID, parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, items)
}

func (h *CommercialHandler) CreateActivityReport(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		WriteError(w, r, domain.BadRequest("invalid multipart form"))
		return
	}

	var partnerID *uuid.UUID
	if raw := r.FormValue("partner_id"); raw != "" {
		parsed, appErr := parseUUIDParam(raw, "invalid partner_id")
		if appErr != nil {
			WriteError(w, r, appErr)
			return
		}
		partnerID = &parsed
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		WriteError(w, r, domain.BadRequest("photo is required"))
		return
	}
	defer file.Close()

	report, appErr := h.svc.CreateActivityReport(r.Context(), ac, service.CreateActivityReportInput{
		PartnerID:    partnerID,
		ProspectName: optionalFormValue(r, "prospect_name"),
		ActivityType: r.FormValue("activity_type"),
		Comment:      r.FormValue("comment"),
		City:         optionalFormValue(r, "city"),
		Location:     optionalFormValue(r, "location"),
		Photo: service.ActivityPhotoInput{
			Reader:      file,
			FileName:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
		},
	})
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusCreated, report)
}

func (h *CommercialHandler) ActivityReportPhoto(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	reportID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid activity report id")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	report, serviceErr := h.svc.GetActivityReportPhoto(r.Context(), ac, reportID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}
	file, err := os.Open(report.PhotoStoragePath)
	if err != nil {
		WriteError(w, r, domain.NotFound("activity report photo"))
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", report.PhotoContentType)
	http.ServeContent(w, r, report.ID.String(), report.CreatedAt, file)
}

func (h *CommercialHandler) AdminListCommercials(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	items, appErr := h.svc.AdminListCommercials(r.Context(), ac, parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, items)
}

func (h *CommercialHandler) AdminGetCommercial(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	commercialID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid commercial id")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	item, serviceErr := h.svc.AdminGetCommercial(r.Context(), ac, commercialID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, item)
}

func (h *CommercialHandler) AdminListActivityReports(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	var commercialID *uuid.UUID
	var partnerID *uuid.UUID
	if raw := r.URL.Query().Get("commercial_id"); raw != "" {
		parsed, appErr := parseUUIDParam(raw, "invalid commercial_id")
		if appErr != nil {
			WriteError(w, r, appErr)
			return
		}
		commercialID = &parsed
	}
	if raw := r.URL.Query().Get("partner_id"); raw != "" {
		parsed, appErr := parseUUIDParam(raw, "invalid partner_id")
		if appErr != nil {
			WriteError(w, r, appErr)
			return
		}
		partnerID = &parsed
	}
	items, appErr := h.svc.AdminListActivityReports(r.Context(), ac, commercialID, partnerID, parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, items)
}

func (h *CommercialHandler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	commercialID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid commercial id")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	var req UpdateCommercialStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}
	profile, serviceErr := h.svc.AdminUpdateStatus(r.Context(), ac, commercialID, req.Status)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, profile)
}

func (h *CommercialHandler) AdminUpdateCommission(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	commercialID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid commercial id")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	var req UpdateCommercialCommissionRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	profile, serviceErr := h.svc.AdminUpdateCommissionPercentage(r.Context(), ac, commercialID, req.CommissionPercentage)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, profile)
}

func optionalFormValue(r *http.Request, key string) *string {
	value := r.FormValue(key)
	if value == "" {
		return nil
	}
	return &value
}
