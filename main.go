package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/controllers"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/routes"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
)

func main() {
	config.LoadEnv()
	config.ConnectDatabase()
	defer config.CloseDatabase()

	config.ConnectRedis()
	defer config.CloseRedis()

	projectModel := models.NewProjectModel(config.DB)
	messageModel := models.NewMessageModel(config.DB)
	versionModel := models.NewVersionModel(config.DB)
	logModel := models.NewLogModel(config.DB)
	templateModel := models.NewTemplateModel(config.DB)
	uiTemplateModel := models.NewUITemplateModel(config.DB)

	commentModel := models.NewCommentModel(config.DB)
	foundationModel := models.NewFoundationModel(config.DB)
	settingsModel := models.NewSettingsModel(config.DB)
	exportModel := models.NewExportModel(config.DB)

	aiService := services.NewAIService()
	projectService := services.NewProjectService(
		aiService,
		projectModel,
		messageModel,
		versionModel,
		logModel,
		templateModel,
	)
	projectController := controllers.NewProjectController(projectService)

	uiDesignService := services.NewUIDesignService(
		projectModel,
		messageModel,
		versionModel,
		uiTemplateModel,
		aiService,
	)
	uiDesignController := controllers.NewUIDesignController(uiDesignService)

	workspaceController := controllers.NewWorkspaceController(
		commentModel,
		foundationModel,
		settingsModel,
		exportModel,
	)

	jobDispatcherService := services.NewJobDispatcherService(
		projectModel,
		messageModel,
		versionModel,
		logModel,
	)
	asyncJobController := controllers.NewAsyncJobController(jobDispatcherService, projectService)

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, projectController, uiDesignController, workspaceController, asyncJobController)

	var handler http.Handler = mux
	handler = middlewares.LoggerMiddleware(handler)
	handler = middlewares.CORSMiddleware(handler)

	server := &http.Server{
		Addr:         ":" + config.Env.AppPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", config.Env.AppPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
