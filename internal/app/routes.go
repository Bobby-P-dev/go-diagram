package app

import (
	artifactcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/controller"
	generationcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/controller"
	"net/http"

	diagramcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/controller"
	identitycontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/controller"
	identitymiddleware "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/middleware"
	productcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/controller"
	sharingcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/controller"
	uidesigncontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/controller"
	workspacecontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/controller"
	platformmiddleware "github.com/Bobby-P-dev/go-diagram.git/internal/platform/middleware"
)

func RegisterContextRoutes(mux *http.ServeMux, controller *generationcontroller.ContextController) {
	mux.HandleFunc("POST /api/v2/projects/{projectId}/threads/{threadId}/context/resolve", controller.Resolve)
}

func RegisterGenerationRoutes(mux *http.ServeMux, controller *generationcontroller.RunController) {
	mux.HandleFunc("POST /api/v2/projects/{projectId}/threads/{threadId}/generations", controller.Start)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/generations/{runId}", controller.Get)
	mux.HandleFunc("POST /api/v2/projects/{projectId}/generations/{runId}/cancel", controller.Cancel)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/generations/{runId}/events", controller.Events)
}

func RegisterArtifactRoutes(mux *http.ServeMux, controller *artifactcontroller.ArtifactController) {
	mux.HandleFunc("POST /api/v2/projects/{projectId}/artifacts", controller.CreateArtifact)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/artifacts", controller.ListArtifacts)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/artifacts/{artifactId}", controller.GetArtifact)
	mux.HandleFunc("POST /api/v2/projects/{projectId}/artifacts/{artifactId}/revisions", controller.AppendRevision)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/artifacts/{artifactId}/revisions", controller.ListRevisions)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/artifacts/{artifactId}/revisions/{revisionId}", controller.GetRevision)
	mux.HandleFunc("POST /api/v2/projects/{projectId}/artifacts/{artifactId}/restores", controller.RestoreRevision)
}

func RegisterProductRoutes(mux *http.ServeMux, controller *productcontroller.ProductController) {
	mux.HandleFunc("POST /api/v2/projects", controller.CreateProject)
	mux.HandleFunc("GET /api/v2/projects", controller.ListProjects)
	mux.HandleFunc("POST /api/v2/projects/{projectId}/threads", controller.CreateThread)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/threads", controller.ListThreads)
	mux.HandleFunc("POST /api/v2/projects/{projectId}/threads/{threadId}/messages", controller.PostMessage)
	mux.HandleFunc("GET /api/v2/projects/{projectId}/threads/{threadId}/messages", controller.ListMessages)
}

func RegisterBrowserAuthRoutes(mux *http.ServeMux, controller *identitycontroller.BrowserController) {
	mux.HandleFunc("POST /api/auth/login", controller.Login)
	mux.HandleFunc("GET /api/auth/session", controller.Session)
	mux.HandleFunc("DELETE /api/auth/session", controller.Logout)
}

func RegisterRoutes(
	mux *http.ServeMux,
	projectCtrl *diagramcontroller.ProjectController,
	uiDesignCtrl *uidesigncontroller.UIDesignController,
	workspaceCtrl *workspacecontroller.WorkspaceController,
	asyncJobCtrl *uidesigncontroller.AsyncJobController,
	shareCtrl *sharingcontroller.ShareController,
	authCtrl *identitycontroller.AuthController,
	adminCtrl *identitycontroller.AdminController,
	internalServiceKey string,
) {
	// Auth Routes
	if authCtrl != nil {
		mux.HandleFunc("POST /api/auth/verify", authCtrl.Verify)
		mux.HandleFunc("GET /api/auth/me", authCtrl.Me)
		mux.HandleFunc("POST /api/auth/logout", authCtrl.Logout)
	}

	// Admin Credentials Management Routes
	if adminCtrl != nil {
		mux.HandleFunc("GET /api/admin/credentials", identitymiddleware.RequireAdmin(adminCtrl.GetCredentials))
		mux.HandleFunc("POST /api/admin/credentials", identitymiddleware.RequireAdmin(adminCtrl.CreateCredential))
		mux.HandleFunc("PUT /api/admin/credentials/{id}", identitymiddleware.RequireAdmin(adminCtrl.UpdateCredential))
		mux.HandleFunc("DELETE /api/admin/credentials/{id}", identitymiddleware.RequireAdmin(adminCtrl.DeleteCredential))
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
		mux.HandleFunc("POST /internal/v1/jobs/callback", platformmiddleware.RequireInternalKey(internalServiceKey, asyncJobCtrl.HandleJobCallback))
	}

	// Workspace Enhancements Routes (Comments, Foundations, Settings, Exports)
	mux.HandleFunc("GET /api/projects/{id}/comments", workspaceCtrl.GetComments)
	mux.HandleFunc("POST /api/projects/{id}/comments", workspaceCtrl.CreateComment)
	mux.HandleFunc("PATCH /api/comments/{id}/status", workspaceCtrl.UpdateCommentStatus)
	mux.HandleFunc("DELETE /api/comments/{id}", workspaceCtrl.DeleteComment)

	mux.HandleFunc("GET /api/foundations", workspaceCtrl.GetFoundations)
	mux.HandleFunc("GET /api/settings", workspaceCtrl.GetSettings)
	mux.HandleFunc("PUT /api/settings", identitymiddleware.RequireAdmin(workspaceCtrl.UpdateSettings))
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
