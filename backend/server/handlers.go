package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func setupAdminRoutes(router *gin.Engine, store *Store, cfg Config) {
	adminRoot, err := resolveAdminUIRoot()
	if err != nil {
		// 允许服务启动（仅管理台静态资源不可用），错误在访问时提示即可。
		adminRoot = ""
	}

	if cfg.EnableAuth {
		router.Use(gin.BasicAuth(gin.Accounts{
			cfg.AdminUser: cfg.AdminPass,
		}))
	}

	api := router.Group("/api")
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

	// Vite build 默认使用绝对路径 /assets/*，因此在管理端口上暴露 assets 静态目录。
	if adminRoot != "" {
		router.Static("/assets", filepath.Join(adminRoot, "assets"))
	}

	// 管理台默认挂载在根路径：/
	router.GET("/", func(c *gin.Context) { serveAdmin(c, adminRoot) })

	// SPA history fallback：管理台任意路径都走 index.html（排除 /api 与 /assets）
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/assets") {
			c.Status(http.StatusNotFound)
			return
		}
		serveAdmin(c, adminRoot)
	})
}

func setupMockRoutes(router *gin.Engine, store *Store) {
	router.NoRoute(func(c *gin.Context) {
		mockHandler(c, store)
	})
}

func NewAdminRouter(store *Store, cfg Config) *gin.Engine {
	router := gin.Default()
	setupAdminRoutes(router, store, cfg)
	return router
}

func NewMockRouter(store *Store) *gin.Engine {
	router := gin.Default()
	setupMockRoutes(router, store)
	return router
}

func serveAdmin(c *gin.Context, root string) {
	if root == "" {
		c.String(http.StatusNotFound, "admin ui not built: run `make web-build` (or build via docker)")
		return
	}

	reqPath := c.Request.URL.Path
	if reqPath == "" || reqPath == "/" {
		serveAdminIndex(c, root)
		return
	}

	// 防御性：避免把管理 API 当静态资源处理。
	if strings.HasPrefix(reqPath, "/api") || strings.HasPrefix(reqPath, "/assets") {
		c.Status(http.StatusNotFound)
		return
	}

	// reqPath 形如 "/assets/xxx.js"；清理后去掉前导 "/"，避免 Join 变成绝对路径。
	rel := strings.TrimPrefix(filepath.Clean(reqPath), string(filepath.Separator))
	path := filepath.Join(root, rel)

	// 防止 path traversal：确保最终路径仍在 root 下。
	absRoot, err1 := filepath.Abs(root)
	absPath, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil || !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		c.Status(http.StatusNotFound)
		return
	}

	st, err := os.Stat(absPath)
	if err == nil && !st.IsDir() {
		c.File(absPath)
		return
	}

	// SPA history fallback：未知路径返回 index.html
	serveAdminIndex(c, root)
}

func serveAdminIndex(c *gin.Context, root string) {
	indexPath := filepath.Join(root, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		c.String(http.StatusNotFound, "admin ui not built: run `make web-build` (or build via docker)")
		return
	}
	c.File(indexPath)
}

func resolveAdminUIRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("ADMIN_UI_DIR")); v != "" {
		if ok, err := hasIndexHTML(v); ok {
			return v, nil
		} else if err != nil {
			return "", err
		}
		return "", os.ErrNotExist
	}

	// 支持从仓库根目录运行（root/web/dist）与从 backend 目录运行（backend/../web/dist）。
	candidates := []string{
		filepath.Join("web", "dist"),
		filepath.Join("..", "web", "dist"),
	}
	for _, cand := range candidates {
		if ok, err := hasIndexHTML(cand); ok {
			return cand, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", os.ErrNotExist
}

func hasIndexHTML(root string) (bool, error) {
	st, err := os.Stat(filepath.Join(root, "index.html"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !st.IsDir(), nil
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
