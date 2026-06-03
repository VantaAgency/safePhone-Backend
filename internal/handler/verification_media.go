package handler

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/storage"
)

// Plans v2 verification media upload. The frontend posts a multipart
// form with a single "file" field; we validate type+size, persist to
// S3 (or the local fallback), and return the public-facing URL the
// frontend embeds in the create-payment / register-device payload.
//
// The URL is served back through GET /api/v1/devices/verification-media/{token}
// which proxies the bucket read after checking the requester is either
// the uploader OR an admin (so the Verifications tab can render them).

const (
	maxPhotoBytes int64 = 12 << 20  // 12 MiB
	maxVideoBytes int64 = 75 << 20  // 75 MiB
	maxFormBytes  int64 = 100 << 20 // hard ceiling on multipart parse
)

var allowedPhotoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/heic": ".heic",
}

var allowedVideoTypes = map[string]string{
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
}

type VerificationMediaHandler struct {
	store     storage.VerificationMediaStore
	publicURL string
}

func NewVerificationMediaHandler(store storage.VerificationMediaStore, publicURL string) *VerificationMediaHandler {
	return &VerificationMediaHandler{
		store:     store,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

type verificationMediaUploadResponse struct {
	URL string `json:"url"`
}

func (h *VerificationMediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		WriteError(w, r, domain.BadRequest("upload too large or malformed"))
		return
	}

	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	if kind != "photo" && kind != "video" {
		WriteError(w, r, domain.ValidationFailed("invalid kind", map[string]string{
			"kind": "must be 'photo' or 'video'",
		}))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, r, domain.BadRequest("file is required"))
		return
	}
	defer file.Close()

	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	var (
		allowed map[string]string
		maxSize int64
	)
	if kind == "photo" {
		allowed, maxSize = allowedPhotoTypes, maxPhotoBytes
	} else {
		allowed, maxSize = allowedVideoTypes, maxVideoBytes
	}
	ext, ok := allowed[contentType]
	if !ok {
		// Fall back to the filename's extension if the browser
		// didn't send a usable Content-Type (some mobile uploads).
		fallbackExt := strings.ToLower(filepath.Ext(header.Filename))
		matched := false
		for ct, mappedExt := range allowed {
			if fallbackExt == mappedExt {
				contentType = ct
				ext = mappedExt
				matched = true
				break
			}
		}
		if !matched {
			WriteError(w, r, domain.ValidationFailed("unsupported file type", map[string]string{
				"file": "unsupported content type for " + kind,
			}))
			return
		}
	}
	if header.Size > maxSize {
		WriteError(w, r, domain.ValidationFailed("file too large", map[string]string{
			"file": "exceeds the size limit",
		}))
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		WriteError(w, r, domain.BadRequest("failed to read upload"))
		return
	}
	if int64(len(data)) > maxSize {
		WriteError(w, r, domain.ValidationFailed("file too large", map[string]string{
			"file": "exceeds the size limit",
		}))
		return
	}

	token, err := h.store.Store(r.Context(), ac.UserID.String(), ext, contentType, data)
	if err != nil {
		slog.Error("verification media: store failed", "error", err, "user_id", ac.UserID)
		WriteError(w, r, domain.Internal("failed to store upload"))
		return
	}

	url := h.publicURL + "/api/v1/devices/verification-media/" + token
	WriteSuccess(w, r, http.StatusCreated, verificationMediaUploadResponse{URL: url})
}

func (h *VerificationMediaHandler) Serve(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	userID := chi.URLParam(r, "userID")
	filename := chi.URLParam(r, "filename")
	if userID == "" || filename == "" {
		WriteError(w, r, domain.NotFound("verification media not found"))
		return
	}
	// Reject path traversal.
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		WriteError(w, r, domain.NotFound("verification media not found"))
		return
	}

	// Authorization: uploader OR admin.
	if userID != ac.UserID.String() && !ac.HasRole(auth.RoleAdmin) {
		WriteError(w, r, domain.Forbidden("not allowed"))
		return
	}

	rc, ct, err := h.store.Open(r.Context(), userID+"/"+filename)
	if err != nil {
		slog.Warn("verification media: open failed", "error", err, "user_id", userID, "filename", filename)
		WriteError(w, r, domain.NotFound("verification media not found"))
		return
	}
	defer rc.Close()

	if ct == "" {
		// Best-effort fallback from the extension when storage didn't preserve it.
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".jpg", ".jpeg":
			ct = "image/jpeg"
		case ".png":
			ct = "image/png"
		case ".webp":
			ct = "image/webp"
		case ".heic":
			ct = "image/heic"
		case ".mp4":
			ct = "video/mp4"
		case ".mov":
			ct = "video/quicktime"
		case ".webm":
			ct = "video/webm"
		default:
			ct = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=300")
	if _, err := io.Copy(w, rc); err != nil {
		return
	}
}
