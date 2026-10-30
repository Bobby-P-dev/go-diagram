package app

import (
	"context"
	"database/sql"
	"net/http"
	"os"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/contextgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/llmgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/projectgateway"
	artifactcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/controller"
	artifactrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/repository"
	artifactservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/service"
	diagramcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/controller"
	diagramintegration "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/integration"
	diagramrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/repository"
	diagramservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/service"
	generationcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/controller"
	generationrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/repository"
	generationservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identitycontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/controller"
	identitymiddleware "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/middleware"
	identityrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	productcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/controller"
	productrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/repository"
	productservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/service"
	sharingcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/controller"
	sharerepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/repository"
	shareservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/service"
	uidesigncontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/controller"
	uidesignintegration "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/integration"
	uidesignrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/repository"
	uidesignservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	workspacecontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/controller"
	workspacerepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/repository"
	workspaceservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
	platformconfig "github.com/Bobby-P-dev/go-diagram.git/internal/platform/config"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/llm"
	platformmiddleware "github.com/Bobby-P-dev/go-diagram.git/internal/platform/middleware"
	"github.com/redis/go-redis/v9"
)

func NewHandler(env *platformconfig.EnvConfig, db *sql.DB, redisClient *redis.Client, credentialService identityservice.CredentialService) http.Handler {
	return NewHandlerWithContext(nil, env, db, redisClient, credentialService)
}

func NewHandlerWithContext(workerContext context.Context, env *platformconfig.EnvConfig, db *sql.DB, redisClient *redis.Client, credentialService identityservice.CredentialService) http.Handler {
	projectModel := diagramrepository.NewProjectRepository(db, identity.MasterAdminID)
	messageModel := diagramrepository.NewMessageRepository(db)
	versionModel := diagramrepository.NewVersionRepository(db)
	usageRecorder := aiusage.NewPostgresRepository(db)
	templateModel := diagramrepository.NewTemplateRepository(db)

	commentRepository := workspacerepository.NewCommentRepository(db)
	foundationRepository := workspacerepository.NewFoundationRepository(db)
	settingsRepository := workspacerepository.NewSettingsRepository(db)
	exportRepository := workspacerepository.NewExportRepository(db)
	shareRepo := sharerepository.NewPostgresRepository(db)
	projectGateway := projectgateway.NewSQLProjectGateway(db)
	shareService := shareservice.NewShareService(shareRepo, projectGateway, identity.MasterAdminID)

	llmClient := llm.NewClient(llm.Config{
		AIProvider:       env.AIProvider,
		OpenAIAPIKey:     env.OpenAIAPIKey,
		OpenAIModel:      env.OpenAIModel,
		OpenAIMBaseURL:   env.OpenAIMBaseURL,
		AnthropicAPIKey:  env.AnthropicAPIKey,
		AnthropicModel:   env.AnthropicModel,
		AnthropicBaseURL: env.AnthropicBaseURL,
		AIMaxTokens:      env.AIMaxTokens,
	}, nil)
	projectService := diagramservice.NewDiagramService(
		diagramintegration.NewAIClient(llmClient),
		projectModel,
		messageModel,
		versionModel,
		usageRecorder,
		templateModel,
	)
	projectController := diagramcontroller.NewProjectController(projectService)
	shareReader := sharingcontroller.ProjectAccessReaderFunc(func(ctx context.Context, projectID, userID string) (string, error) {
		p, _, err := projectModel.GetByIDScopedContext(ctx, projectID, userID, "")
		if err != nil {
			return "", err
		}
		return p.Title, nil
	})
	shareLoader := sharingcontroller.ProjectDetailLoaderFunc(func(ctx context.Context, id string) (interface{}, error) {
		return projectService.GetProjectByID(id)
	})
	shareController := sharingcontroller.NewShareController(shareService, shareReader, shareLoader)
	browserService := identityservice.NewBrowserService(identityrepository.NewBrowserRepository(db), credentialService)
	browserController := identitycontroller.NewBrowserController(browserService, env.AuthCookieSecure)
	authController := identitycontroller.NewAuthController(credentialService, browserController)
	adminController := identitycontroller.NewAdminController(credentialService)

	uiDesignProjects := projectgateway.NewUIDesignProjectGateway(projectModel, messageModel, versionModel)
	uiDesignService := uidesignservice.NewUIDesignService(
		uiDesignProjects,
		uidesignintegration.NewUIDesignAIClient(llmClient),
		func(ctx context.Context) string {
			if user := authcontext.UserFromContext(ctx); user != nil {
				return user.ID
			}
			return ""
		},
	)
	uiDesignController := uidesigncontroller.NewUIDesignController(uiDesignService)

	workspaceService := workspaceservice.NewWorkspaceService(
		projectgateway.NewWorkspaceProjectAccessReader(projectModel),
		commentRepository,
		foundationRepository,
		settingsRepository,
		exportRepository,
	)
	workspaceController := workspacecontroller.NewWorkspaceController(workspaceService)

	var jobStatusStore uidesignservice.JobStatusStore
	var jobQueue uidesignservice.JobQueue
	if redisClient != nil {
		store := uidesignrepository.NewRedisJobStore(redisClient)
		jobStatusStore, jobQueue = store, store
	}
	jobDispatcherService := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: uiDesignProjects,
		JobModel:       uidesignrepository.NewPostgresJobBindingRepository(db),
		StatusStore:    jobStatusStore,
		Queue:          jobQueue,
		LogRecorder:    usageRecorder,
		Sidecar:        uidesignintegration.NewHTTPSidecarDispatcher(env.OrchestrationServiceURL, env.InternalServiceKey, nil),
		CallbackResolver: func() string {
			return uidesignintegration.ResolveCallbackURL(os.Getenv("BACKEND_INTERNAL_URL"), os.Getenv("GO_BACKEND_URL"), env.AppPort)
		},
	})
	asyncJobController := uidesigncontroller.NewAsyncJobControllerWithOptions(
		jobDispatcherService,
		nil,
		func(request *http.Request) bool {
			return platformmiddleware.ValidInternalKey(env.InternalServiceKey, request)
		},
	)

	mux := http.NewServeMux()
	productController := productcontroller.NewProductController(productservice.NewService(productrepository.NewPostgresRepository(db)))
	RegisterProductRoutes(mux, productController)
	artifactController := artifactcontroller.NewArtifactController(artifactservice.NewService(artifactrepository.NewPostgresRepository(db)))
	RegisterArtifactRoutes(mux, artifactController)
	resolver := generationservice.NewContextResolver(contextgateway.NewPostgresReader(db))
	contextController := generationcontroller.NewContextController(resolver)
	RegisterContextRoutes(mux, contextController)
	generationClient := llmgateway.New(llmClient)
	coordinator := generationservice.NewRunCoordinator(
		generationrepository.NewPostgresRunRepository(db), resolver,
		generationservice.NewDocumentEngine(generationClient),
		generationservice.NewDiagramEngine(generationClient),
	)
	RegisterGenerationRoutes(mux, generationcontroller.NewRunController(coordinator))
	if workerContext != nil {
		go coordinator.Run(workerContext)
	}
	RegisterRoutes(
		mux,
		projectController,
		uiDesignController,
		workspaceController,
		asyncJobController,
		shareController,
		authController,
		adminController,
		env.InternalServiceKey,
	)

	var handler http.Handler = mux
	RegisterBrowserAuthRoutes(mux, browserController)
	handler = identitymiddleware.BrowserMiddleware(browserService, credentialService, env.CORSAllowedOrigins, env.LegacyCredentialLogin)(handler)
	handler = platformmiddleware.LoggerMiddleware(handler)
	handler = platformmiddleware.CORSMiddleware(handler, env.CORSAllowedOrigins)

	return handler
}
