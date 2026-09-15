package routes

import (
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/controllers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	projectCtrl *controllers.ProjectController,
	uiDesignCtrl *controllers.UIDesignController,
	workspaceCtrl *controllers.WorkspaceController,
) {
	// Diagram Routes
	mux.HandleFunc("POST /api/projects", projectCtrl.Create)
	mux.HandleFunc("POST /api/projects/{id}/chat", projectCtrl.Chat)
	mux.HandleFunc("GET /api/projects", projectCtrl.GetAll)
	mux.HandleFunc("GET /api/projects/{id}", projectCtrl.GetByID)
	mux.HandleFunc("POST /api/projects/{id}/pin", projectCtrl.TogglePin)
	mux.HandleFunc("GET /api/projects/{id}/versions", projectCtrl.GetVersions)
	mux.HandleFunc("POST /api/projects/{id}/rollback/{versionId}", projectCtrl.Rollback)
	mux.HandleFunc("GET /api/templates", projectCtrl.GetTemplates)

	// UI Design Modular Routes
	mux.HandleFunc("GET /api/ui-design/templates", uiDesignCtrl.GetTemplates)
	mux.HandleFunc("POST /api/ui-design/generate", uiDesignCtrl.Create)
	mux.HandleFunc("POST /api/ui-design/projects/{id}/chat", uiDesignCtrl.Chat)

	// Workspace Enhancements Routes (Comments, Foundations, Settings, Exports)
	mux.HandleFunc("GET /api/projects/{id}/comments", workspaceCtrl.GetComments)
	mux.HandleFunc("POST /api/projects/{id}/comments", workspaceCtrl.CreateComment)
	mux.HandleFunc("PATCH /api/comments/{id}/status", workspaceCtrl.UpdateCommentStatus)
	mux.HandleFunc("DELETE /api/comments/{id}", workspaceCtrl.DeleteComment)

	mux.HandleFunc("GET /api/foundations", workspaceCtrl.GetFoundations)
	mux.HandleFunc("GET /api/settings", workspaceCtrl.GetSettings)
	mux.HandleFunc("PUT /api/settings", workspaceCtrl.UpdateSettings)
	mux.HandleFunc("GET /api/projects/{id}/exports", workspaceCtrl.GetExports)
}
