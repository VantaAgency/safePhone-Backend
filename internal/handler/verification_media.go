package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/storage"
)

// signedMediaTTL bounds how long a signed verification-media link stays valid.
const signedMediaTTL = 30 * time.Minute

func mediaSignature(secret []byte, objectPath string, exp int64) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(objectPath + ":" + strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignStoredMediaURL turns a stored verification-media URL into a same-origin
// signed PATH (relative) that a browser <img>/<video> can load directly — no
// bearer token, so video streams progressively. Returns the input unchanged if
// it isn't a recognizable verification-media URL.
func SignStoredMediaURL(secret []byte, storedURL string) string {
	const prefix = "/api/v1/devices/verification-media/"
	u, err := url.Parse(storedURL)
	if err != nil || !strings.HasPrefix(u.Path, prefix) {
		return storedURL
	}
	objectPath := strings.TrimPrefix(u.Path, prefix) // userID/filename
	exp := time.Now().Add(signedMediaTTL).Unix()
	sig := mediaSignature(secret, objectPath, exp)
	return u.Path + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=" + sig
}

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
	store      storage.VerificationMediaStore
	publicURL  string
	signSecret []byte
}

func NewVerificationMediaHandler(store storage.VerificationMediaStore, publicURL string, signSecret []byte) *VerificationMediaHandler {
	return &VerificationMediaHandler{
		store:      store,
		publicURL:  strings.TrimRight(publicURL, "/"),
		signSecret: signSecret,
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

// ServeSigned streams verification media for a valid short-lived signed URL.
// Public (no bearer) so a browser <img>/<video> can load it directly — the
// signature, which only the backend can produce, is the authorization.
func (h *VerificationMediaHandler) ServeSigned(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	filename := chi.URLParam(r, "filename")
	if userID == "" || filename == "" ||
		strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		WriteError(w, r, domain.NotFound("verification media not found"))
		return
	}

	exp, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || exp < time.Now().Unix() {
		WriteError(w, r, domain.Forbidden("media link expired"))
		return
	}
	objectPath := userID + "/" + filename
	expected := mediaSignature(h.signSecret, objectPath, exp)
	if !hmac.Equal([]byte(expected), []byte(r.URL.Query().Get("sig"))) {
		WriteError(w, r, domain.Forbidden("invalid media link"))
		return
	}

	mc, err := h.store.OpenRange(r.Context(), objectPath, r.Header.Get("Range"))
	if err != nil {
		slog.Warn("verification media: open failed", "error", err, "user_id", userID, "filename", filename)
		WriteError(w, r, domain.NotFound("verification media not found"))
		return
	}
	defer mc.Body.Close()

	ct := mc.ContentType
	if ct == "" {
		ct = contentTypeFromExt(filename)
	}
	w.Header().Set("Content-Type", ct)
	// Advertise + honor Range so a <video> streams/seeks instead of pulling
	// the whole file before it can play.
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=300")
	if mc.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(mc.ContentLength, 10))
	}
	if mc.Partial {
		w.Header().Set("Content-Range", mc.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}
	if _, err := io.Copy(w, mc.Body); err != nil {
		return
	}
}

func contentTypeFromExt(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
