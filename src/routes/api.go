package routes

import (
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/controllers"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
)

func RegisterRoutes(
	mux *http.ServeMux,
	projectCtrl *controllers.ProjectController,
	uiDesignCtrl *controllers.UIDesignController,
	workspaceCtrl *controllers.WorkspaceController,
	asyncJobCtrl *controllers.AsyncJobController,
	shareCtrl *controllers.ShareController,
	authCtrl *controllers.AuthController,
	adminCtrl *controllers.AdminController,
) {
	// Auth Routes
	if authCtrl != nil {
		mux.HandleFunc("POST /api/auth/verify", authCtrl.Verify)
		mux.HandleFunc("GET /api/auth/me", authCtrl.Me)
		mux.HandleFunc("POST /api/auth/logout", authCtrl.Logout)
	}

	// Admin Credentials Management Routes
	if adminCtrl != nil {
		mux.HandleFunc("GET /api/admin/credentials", middlewares.RequireAdmin(adminCtrl.GetCredentials))
		mux.HandleFunc("POST /api/admin/credentials", middlewares.RequireAdmin(adminCtrl.CreateCredential))
		mux.HandleFunc("PUT /api/admin/credentials/{id}", middlewares.RequireAdmin(adminCtrl.UpdateCredential))
		mux.HandleFunc("DELETE /api/admin/credentials/{id}", middlewares.RequireAdmin(adminCtrl.DeleteCredential))
	}

	// Diagram Routes
	mux.HandleFunc("POST /api/projects", projectCtrl.Create)
	mux.HandleFunc("POST /api/projects/{id}/chat", projectCtrl.Chat)
	mux.HandleFunc("GET /api/projects", projectCtrl.GetAll)
	mux.HandleFunc("GET /api/projects/{id}", projectCtrl.GetByID)
	mux.HandleFunc("POST /api/projects/{id}/pin", projectCtrl.TogglePin)
	mux.HandleFunc("GET /api/projects/{id}/versions", projectCtrl.GetVersions)
	mux.HandleFunc("POST /api/projects/{id}/rollback/{versionId}", projectCtrl.Rollback)
	mux.HandleFunc("PUT /api/projects/{id}/graph", projectCtrl.UpdateGraph)
	mux.HandleFunc("GET /api/templates", projectCtrl.GetTemplates)

	// Share Project & Chat Routes (Public & Authenticated)
	if shareCtrl != nil {
		mux.HandleFunc("POST /api/projects/{id}/share", shareCtrl.CreateShare)
		mux.HandleFunc("GET /api/projects/{id}/share", shareCtrl.GetShareStatus)
		mux.HandleFunc("DELETE /api/projects/{id}/share", shareCtrl.RevokeShare)
		mux.HandleFunc("GET /api/shared/{token}", shareCtrl.GetSharedProject)
		mux.HandleFunc("POST /api/shared/{token}/fork", shareCtrl.ForkSharedProject)
	}

	// UI Design Modular Routes
	mux.HandleFunc("GET /api/ui-design/templates", uiDesignCtrl.GetTemplates)
	mux.HandleFunc("POST /api/ui-design/generate", uiDesignCtrl.Create)
	mux.HandleFunc("POST /api/ui-design/generate/stream", uiDesignCtrl.CreateStream)
	mux.HandleFunc("POST /api/ui-design/projects/{id}/chat", uiDesignCtrl.Chat)

	// CrewAI Asynchronous Orchestration Routes
	if asyncJobCtrl != nil {
		mux.HandleFunc("POST /api/ui-design/generate/async", asyncJobCtrl.CreateAsyncUIDesign)
		mux.HandleFunc("GET /api/jobs/{id}", asyncJobCtrl.GetJobStatus)
		mux.HandleFunc("POST /internal/v1/jobs/callback", asyncJobCtrl.HandleJobCallback)
	}

	// Workspace Enhancements Routes (Comments, Foundations, Settings, Exports)
	mux.HandleFunc("GET /api/projects/{id}/comments", workspaceCtrl.GetComments)
	mux.HandleFunc("POST /api/projects/{id}/comments", workspaceCtrl.CreateComment)
	mux.HandleFunc("PATCH /api/comments/{id}/status", workspaceCtrl.UpdateCommentStatus)
	mux.HandleFunc("DELETE /api/comments/{id}", workspaceCtrl.DeleteComment)

	mux.HandleFunc("GET /api/foundations", workspaceCtrl.GetFoundations)
	mux.HandleFunc("GET /api/settings", workspaceCtrl.GetSettings)
	mux.HandleFunc("PUT /api/settings", workspaceCtrl.UpdateSettings)
	mux.HandleFunc("GET /api/projects/{id}/exports", workspaceCtrl.GetExports)

	// Production & Container Health Check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"diagram-backend"}`))
	})

	// Root Service Info (Prevents 404 when opening backend domain directly)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"diagram-backend","status":"running","health":"/health"}`))
	})
}
