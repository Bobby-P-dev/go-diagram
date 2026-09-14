package routes

import (
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/controllers"
)

func RegisterRoutes(mux *http.ServeMux, projectCtrl *controllers.ProjectController) {
	mux.HandleFunc("POST /api/projects", projectCtrl.Create)
	mux.HandleFunc("POST /api/projects/{id}/chat", projectCtrl.Chat)
	mux.HandleFunc("GET /api/projects", projectCtrl.GetAll)
	mux.HandleFunc("GET /api/projects/{id}", projectCtrl.GetByID)
}
