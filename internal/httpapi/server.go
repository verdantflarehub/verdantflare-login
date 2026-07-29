package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

type Config struct {
	CookieName   string
	CookieDomain string
	CookieSecure bool
	SessionTTL   time.Duration
}

type Server struct {
	service *auth.Service
	config  Config
	logger  *slog.Logger
	limiter *rateLimiter
}

func New(service *auth.Service, config Config, logger *slog.Logger) http.Handler {
	server := &Server{
		service: service,
		config:  config,
		logger:  logger,
		limiter: newRateLimiter(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("POST /api/auth/verification-code", server.sendVerificationCode)
	mux.HandleFunc("POST /api/auth/verify-email/resend", server.sendVerificationCode)
	mux.HandleFunc("POST /api/auth/sign-up", server.signUp)
	mux.HandleFunc("POST /api/auth/sign-in", server.signIn)
	mux.HandleFunc("POST /api/auth/forgot-password", server.forgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", server.resetPassword)
	mux.HandleFunc("GET /api/auth/session", server.session)
	mux.HandleFunc("POST /api/auth/logout", server.logout)
	mux.HandleFunc("GET /api/auth/sso/authorize", server.ssoNotConfigured)
	mux.HandleFunc("GET /api/auth/callback", server.ssoNotConfigured)

	crossOrigin := http.NewCrossOriginProtection()
	return server.recoverPanic(server.securityHeaders(server.requestLog(crossOrigin.Handler(mux))))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) sendVerificationCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	key := "verification:" + clientIP(r) + ":" + strings.ToLower(strings.TrimSpace(request.Email))
	if !s.limiter.allow(key, 5, 10*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
		return
	}
	result, err := s.service.SendVerificationCode(r.Context(), request.Email)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	response := map[string]any{"expiresIn": result.ExpiresInSeconds}
	if result.DebugCode != "" {
		response["debugCode"] = result.DebugCode
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (s *Server) signUp(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
		Accepted bool   `json:"accepted"`
		ReturnTo string `json:"returnTo"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if !s.limiter.allow("signup:"+clientIP(r), 10, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
		return
	}
	user, err := s.service.SignUp(r.Context(), auth.SignUpInput{
		Email: request.Email, Password: request.Password, Code: request.Code, Accepted: request.Accepted,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"email": user.Email, "emailVerified": true})
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		ReturnTo string `json:"returnTo"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if !s.limiter.allow("signin:"+clientIP(r), 10, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后重试")
		return
	}
	result, err := s.service.SignIn(r.Context(), auth.SignInInput{
		Email: request.Email, Password: request.Password, ReturnTo: request.ReturnTo,
		IPAddress: clientIP(r), UserAgent: truncate(r.UserAgent(), 512),
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.config.CookieName, Value: result.SessionToken, Path: "/", Domain: s.config.CookieDomain,
		MaxAge: int(s.config.SessionTTL.Seconds()), Expires: result.ExpiresAt,
		Secure: s.config.CookieSecure, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"redirectTo": result.RedirectTo})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if !s.limiter.allow("forgot:"+clientIP(r), 5, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
		return
	}
	if err := s.service.ForgotPassword(r.Context(), request.Email); err != nil {
		s.logger.Error("forgot password failed", "error", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"email": strings.TrimSpace(request.Email)})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if !s.limiter.allow("reset:"+clientIP(r), 10, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
		return
	}
	if err := s.service.ResetPassword(r.Context(), request.Token, request.Password); err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.config.CookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "登录会话无效或已过期")
		return
	}
	result, err := s.service.Session(r.Context(), cookie.Value)
	if err != nil {
		s.clearSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "登录会话无效或已过期")
		return
	}
	w.Header().Set("X-VF-Login-Subject", result.UserID)
	writeJSON(w, http.StatusOK, map[string]any{
		"userId": result.UserID, "email": result.Email, "expiresAt": result.ExpiresAt,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.config.CookieName); err == nil {
		if err := s.service.Logout(r.Context(), cookie.Value); err != nil {
			s.logger.Error("logout failed", "error", err)
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ssoNotConfigured(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "企业 SSO 提供商尚未配置")
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: s.config.CookieName, Value: "", Path: "/", Domain: s.config.CookieDomain,
		MaxAge: -1, Expires: time.Unix(1, 0), Secure: s.config.CookieSecure,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "邮箱或密码不正确")
	case errors.Is(err, auth.ErrAccountDisabled):
		writeError(w, http.StatusForbidden, "账号已被停用")
	case errors.Is(err, auth.ErrConflict):
		writeError(w, http.StatusConflict, "该邮箱已注册")
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "请检查输入内容")
	case errors.Is(err, auth.ErrInvalidChallenge):
		writeError(w, http.StatusBadRequest, "验证码不正确")
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeError(w, http.StatusTooManyRequests, "验证码尝试次数过多，请重新获取")
	case errors.Is(err, auth.ErrExpired), errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusBadRequest, "链接或验证码无效或已过期")
	default:
		s.logger.Error("authentication request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "服务暂时不可用，请稍后重试")
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(started).String())
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("http handler panic", "error", fmt.Sprint(recovered), "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "服务暂时不可用，请稍后重试")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) == nil {
		return errors.New("multiple JSON values")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
