package server

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func setupRoutes(router *gin.Engine, store *Store, cfg Config) {
	admin := router.Group("/__admin")
	if cfg.EnableAuth {
		admin.Use(gin.BasicAuth(gin.Accounts{
			cfg.AdminUser: cfg.AdminPass,
		}))
	}

	api := admin.Group("/api")
	api.GET("/endpoints", func(c *gin.Context) { listEndpointsHandler(c, store) })
	api.POST("/endpoints", func(c *gin.Context) { createEndpointHandler(c, store) })
	api.GET("/endpoints/:id", func(c *gin.Context) { getEndpointHandler(c, store) })
	api.PUT("/endpoints/:id", func(c *gin.Context) { updateEndpointHandler(c, store) })
	api.DELETE("/endpoints/:id", func(c *gin.Context) { deleteEndpointHandler(c, store) })
	api.GET("/endpoints/:id/rules", func(c *gin.Context) { listRulesHandler(c, store) })
	api.POST("/endpoints/:id/rules", func(c *gin.Context) { createRuleHandler(c, store) })
	api.PUT("/rules/:id", func(c *gin.Context) { updateRuleHandler(c, store) })
	api.DELETE("/rules/:id", func(c *gin.Context) { deleteRuleHandler(c, store) })
	api.POST("/preview", func(c *gin.Context) { previewHandler(c, store) })
	api.GET("/export", func(c *gin.Context) { exportHandler(c, store) })
	api.POST("/import", func(c *gin.Context) { importHandler(c, store) })

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/__admin") {
			if cfg.EnableAuth {
				user, pass, ok := c.Request.BasicAuth()
				if !ok || user != cfg.AdminUser || pass != cfg.AdminPass {
					c.Header("WWW-Authenticate", "Basic realm=mock-server")
					c.Status(http.StatusUnauthorized)
					return
				}
			}
			serveAdmin(c)
			return
		}
		mockHandler(c, store)
	})
}

func NewRouter(store *Store, cfg Config) *gin.Engine {
	router := gin.Default()
	setupRoutes(router, store, cfg)
	return router
}

func serveAdmin(c *gin.Context) {
	root := filepath.Join("web", "dist")
	path := filepath.Join(root, filepath.Clean(c.Param("any")))
	if strings.HasSuffix(c.Param("any"), "/") || c.Param("any") == "" {
		path = filepath.Join(root, "index.html")
	}
	c.File(path)
}

func listEndpointsHandler(c *gin.Context, store *Store) {
	c.JSON(http.StatusOK, store.ListEndpoints())
}

func createEndpointHandler(c *gin.Context, store *Store) {
	var payload Endpoint
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	if err := validateEndpoint(payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	created, err := store.CreateEndpoint(payload)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

func getEndpointHandler(c *gin.Context, store *Store) {
	ep, ok := store.GetEndpoint(c.Param("id"))
	if !ok {
		errorJSON(c, http.StatusNotFound, "NOT_FOUND", "endpoint not found")
		return
	}
	c.JSON(http.StatusOK, ep)
}

func updateEndpointHandler(c *gin.Context, store *Store) {
	var payload Endpoint
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	if err := validateEndpoint(payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	updated, err := store.UpdateEndpoint(c.Param("id"), payload)
	if err != nil {
		status := http.StatusInternalServerError
		code := "DB_ERROR"
		if err.Error() == "not found" {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		}
		errorJSON(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

func deleteEndpointHandler(c *gin.Context, store *Store) {
	if err := store.DeleteEndpoint(c.Param("id")); err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func listRulesHandler(c *gin.Context, store *Store) {
	c.JSON(http.StatusOK, store.ListRules(c.Param("id")))
}

func createRuleHandler(c *gin.Context, store *Store) {
	var payload Rule
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	payload.EndpointID = c.Param("id")
	if err := validateRule(payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	created, err := store.CreateRule(payload)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

func updateRuleHandler(c *gin.Context, store *Store) {
	var payload Rule
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	if err := validateRule(payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	updated, err := store.UpdateRule(c.Param("id"), payload)
	if err != nil {
		status := http.StatusInternalServerError
		code := "DB_ERROR"
		if err.Error() == "not found" {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		}
		errorJSON(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

func deleteRuleHandler(c *gin.Context, store *Store) {
	if err := store.DeleteRule(c.Param("id")); err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func previewHandler(c *gin.Context, store *Store) {
	var payload PreviewRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	query := map[string][]string{}
	for k, v := range payload.Query {
		query[k] = []string{v}
	}
	headers := map[string][]string{}
	for k, v := range payload.Headers {
		headers[strings.ToLower(k)] = []string{v}
	}
	bodyResult := gjson.Result{}
	hasJSON := false
	if strings.TrimSpace(payload.Body) != "" && gjson.Valid(payload.Body) {
		bodyResult = gjson.Parse(payload.Body)
		hasJSON = true
	}
	info := RequestInfo{
		Method:   payload.Method,
		Path:     payload.Path,
		Query:    query,
		Headers:  headers,
		Cookies:  map[string]string{},
		BodyRaw:  payload.Body,
		BodyJSON: bodyResult,
		HasJSON:  hasJSON,
	}
	matched, ep, rule, explain, resp := store.Match(info)
	res := PreviewResponse{
		Matched: matched,
		Explain: explain,
	}
	if matched && ep != nil && rule != nil {
		res.EndpointID = ep.ID
		res.RuleID = rule.ID
		res.Response = resp
	}
	c.JSON(http.StatusOK, res)
}

func exportHandler(c *gin.Context, store *Store) {
	bundle, err := store.ExportAll()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func importHandler(c *gin.Context, store *Store) {
	var payload ExportBundle
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payload", err.Error())
		return
	}
	if err := store.ImportAll(payload); err != nil {
		errorJSON(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func mockHandler(c *gin.Context, store *Store) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	bodyRaw := string(bodyBytes)
	bodyResult := gjson.Result{}
	hasJSON := false
	if strings.TrimSpace(bodyRaw) != "" && gjson.Valid(bodyRaw) {
		bodyResult = gjson.Parse(bodyRaw)
		hasJSON = true
	}
	headers := map[string][]string{}
	for k, v := range c.Request.Header {
		headers[strings.ToLower(k)] = v
	}
	cookies := map[string]string{}
	for _, ck := range c.Request.Cookies() {
		cookies[ck.Name] = ck.Value
	}
	info := RequestInfo{
		Method:   c.Request.Method,
		Path:     c.Request.URL.Path,
		Query:    c.Request.URL.Query(),
		Headers:  headers,
		Cookies:  cookies,
		BodyRaw:  bodyRaw,
		BodyJSON: bodyResult,
		HasJSON:  hasJSON,
	}
	matched, _, _, _, resp := store.Match(info)
	if !matched {
		c.Status(http.StatusNotFound)
		return
	}
	if resp.DelayMs > 0 {
		time.Sleep(time.Duration(resp.DelayMs) * time.Millisecond)
	}
	for k, v := range resp.Headers {
		c.Header(k, v)
	}
	if resp.ContentType != "" {
		c.Header("Content-Type", resp.ContentType)
	} else if resp.BodyType == "json" {
		c.Header("Content-Type", "application/json")
	} else if resp.BodyType == "text" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	c.Status(status)
	if resp.Body != "" {
		_, _ = c.Writer.WriteString(resp.Body)
	}
}

func errorJSON(c *gin.Context, status int, code string, message string, details ...string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
