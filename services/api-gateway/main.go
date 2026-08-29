package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/logx"
	"github.com/golang-jwt/jwt/v5"
)

const (
	serviceName = "api-gateway"
	listenAddr  = ":8080"
)

type gateway struct {
	secret []byte
	routes []route
}
type route struct {
	prefix string
	proxy  *httputil.ReverseProxy
}

func main() {
	closeLog, logErr := logx.Configure(serviceName)
	if logErr != nil {
		log.Printf("event=log_config_error error=%q", logErr)
	} else {
		defer closeLog()
	}

	g := &gateway{secret: []byte(httpx.Secret("JWT_SECRET", "local-dev-secret-change-this-value", 32))}
	g.routes = []route{
		{"/api/auth", proxy(httpx.Env("AUTH_SERVICE_URL", "http://auth-service:8080"))},
		{"/api/users", proxy(httpx.Env("AUTH_SERVICE_URL", "http://auth-service:8080"))},
		{"/api/audits", proxy(httpx.Env("AUDIT_SERVICE_URL", "http://audit-service:8080"))},
		{"/api/issues", proxy(httpx.Env("AUDIT_SERVICE_URL", "http://audit-service:8080"))},
		{"/api/inventory", proxy(httpx.Env("INVENTORY_SERVICE_URL", "http://inventory-service:8080"))},
		{"/api/suppliers", proxy(httpx.Env("INVENTORY_SERVICE_URL", "http://inventory-service:8080"))},
		{"/api/sales", proxy(httpx.Env("SALES_SERVICE_URL", "http://sales-service:8080"))},
		{"/api/dashboard", proxy(httpx.Env("DASHBOARD_SERVICE_URL", "http://dashboard-service:8080"))},
		{"/api/chat", proxy(httpx.Env("CHAT_SERVICE_URL", "http://chat-service:8080"))},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/livez", httpx.LivenessHandler(serviceName))
	mux.HandleFunc("/readyz", httpx.ReadinessHandler(serviceName, 0))
	mux.Handle("/", g)
	log.Println(serviceName + " listening on " + listenAddr)
	server := httpx.NewServer(listenAddr, httpx.RequestLogger(serviceName, mux))
	if err := httpx.RunServer(server, serviceName, 0); err != nil {
		log.Fatal(err)
	}
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
}

func proxy(raw string) *httputil.ReverseProxy {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	p := httputil.NewSingleHostReverseProxy(u)
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("event=proxy_error request_id=%q upstream=%q error=%q", httpx.RequestID(r.Context()), u.Host, err)
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": "upstream service unavailable"})
	}
	return p
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", httpx.Env("CORS_ORIGIN", "http://localhost:3000"))
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.URL.Path != "/api/auth/login" {
		claims, err := g.claims(r)
		if err != nil {
			httpx.JSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		role, _ := claims["role"].(string)
		if !allowed(role, r.Method, r.URL.Path) {
			httpx.JSON(w, 403, map[string]string{"error": "forbidden for role " + role})
			return
		}
		name, _ := claims["name"].(string)
		if strings.TrimSpace(name) == "" {
			name, _ = claims["email"].(string)
		}
		// Downstream identity is derived from the verified JWT, never from browser-provided headers.
		r.Header.Set("X-User-ID", fmt.Sprint(claims["sub"]))
		r.Header.Set("X-User-Name", name)
		r.Header.Set("X-User-Role", role)
	}

	for _, rt := range g.routes {
		if pathMatchesPrefix(r.URL.Path, rt.prefix) {
			rt.proxy.ServeHTTP(w, r)
			return
		}
	}
	httpx.JSON(w, 404, map[string]string{"error": "route not found"})
}

func (g *gateway) claims(r *http.Request) (jwt.MapClaims, error) {
	raw, err := httpx.BearerToken(r)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return g.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func allowed(role, method, path string) bool {
	if role != "admin" && role != "auditor" && role != "staff" {
		return false
	}
	if pathMatchesPrefix(path, "/api/chat") && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}
	if role == "admin" {
		return true
	}
	if method == http.MethodGet {
		return true
	}
	if role == "auditor" && (pathMatchesPrefix(path, "/api/audits") || pathMatchesPrefix(path, "/api/issues")) {
		return true
	}
	if role == "staff" && pathMatchesPrefix(path, "/api/sales") {
		return true
	}
	return false
}

func pathMatchesPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
