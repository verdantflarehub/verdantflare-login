package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

const handoffBodyLimit = 16 << 10

var handoffUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type handoffSessionFact struct {
	SubjectRef        string    `json:"subject_ref"`
	AuthenticationRef string    `json:"authentication_ref"`
	LoginExpiresAt    time.Time `json:"login_expires_at"`
	CheckedAt         time.Time `json:"checked_at"`
}

func (s *Server) handoffAuthorized(r *http.Request) bool {
	if s.config.HandoffToken == "" {
		return false
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if provided == "" {
		return false
	}
	want, got := sha256.Sum256([]byte(s.config.HandoffToken)), sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func writeHandoffError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"code": code})
}

func writeHandoffFact(w http.ResponseWriter, result auth.SessionResult) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, handoffSessionFact{
		SubjectRef: result.UserID, AuthenticationRef: result.AuthenticationRef,
		LoginExpiresAt: result.ExpiresAt, CheckedAt: time.Now().UTC(),
	})
}

func handoffSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrNotFound) || errors.Is(err, auth.ErrExpired) || errors.Is(err, auth.ErrAccountDisabled) {
		writeHandoffError(w, http.StatusForbidden, "HANDOFF_REVOKED")
		return
	}
	writeHandoffError(w, http.StatusServiceUnavailable, "HANDOFF_UNAVAILABLE")
}

// resolveHandoffSession is internal-only: the public Login Web proxy must not
// expose /api/internal. The bearer grants Control a session fact, not a token.
func (s *Server) resolveHandoffSession(w http.ResponseWriter, r *http.Request) {
	if !s.handoffAuthorized(r) {
		writeHandoffError(w, http.StatusUnauthorized, "HANDOFF_UNAUTHENTICATED")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, handoffBodyLimit))
	if err != nil || len(body) != 0 {
		writeHandoffError(w, http.StatusBadRequest, "HANDOFF_INVALID_REQUEST")
		return
	}
	cookie, err := r.Cookie(s.config.CookieName)
	if err != nil || cookie.Value == "" {
		writeHandoffError(w, http.StatusUnauthorized, "HANDOFF_UNAUTHENTICATED")
		return
	}
	result, err := s.service.Session(r.Context(), cookie.Value)
	if err != nil {
		handoffSessionError(w, err)
		return
	}
	writeHandoffFact(w, result)
}

func (s *Server) checkHandoffSession(w http.ResponseWriter, r *http.Request) {
	if !s.handoffAuthorized(r) {
		writeHandoffError(w, http.StatusUnauthorized, "HANDOFF_UNAUTHENTICATED")
		return
	}
	input, err := decodeHandoffCheck(w, r)
	if err != nil {
		writeHandoffError(w, http.StatusBadRequest, "HANDOFF_INVALID_REQUEST")
		return
	}
	result, err := s.service.SessionByAuthenticationRef(r.Context(), input.AuthenticationRef, input.SubjectRef)
	if err != nil {
		handoffSessionError(w, err)
		return
	}
	writeHandoffFact(w, result)
}

type handoffCheckInput struct {
	AuthenticationRef string
	SubjectRef        string
}

// A field-by-field token walk rejects duplicate keys as well as unknown keys.
// encoding/json's DisallowUnknownFields alone does not reject duplicates.
func decodeHandoffCheck(w http.ResponseWriter, r *http.Request) (handoffCheckInput, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, handoffBodyLimit))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return handoffCheckInput{}, errors.New("object required")
	}
	input := handoffCheckInput{}
	seen := map[string]bool{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return handoffCheckInput{}, err
		}
		key, ok := keyToken.(string)
		if !ok || seen[key] || (key != "authentication_ref" && key != "subject_ref") {
			return handoffCheckInput{}, errors.New("unknown or duplicate field")
		}
		seen[key] = true
		var value string
		if err := decoder.Decode(&value); err != nil || !handoffUUIDPattern.MatchString(value) {
			return handoffCheckInput{}, errors.New("invalid value")
		}
		if key == "authentication_ref" {
			input.AuthenticationRef = value
		} else {
			input.SubjectRef = value
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || !seen["authentication_ref"] || !seen["subject_ref"] {
		return handoffCheckInput{}, errors.New("incomplete object")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return handoffCheckInput{}, errors.New("trailing content")
	}
	return input, nil
}
