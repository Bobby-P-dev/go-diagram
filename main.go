package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/app"
	identityrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	platformconfig "github.com/Bobby-P-dev/go-diagram.git/internal/platform/config"
	platformdatabase "github.com/Bobby-P-dev/go-diagram.git/internal/platform/database"
	platformredis "github.com/Bobby-P-dev/go-diagram.git/internal/platform/redis"
)

func main() {
	env := platformconfig.LoadEnv()
	if err := env.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	db, err := platformdatabase.Connect(context.Background(), platformdatabase.Config{
		Host:     env.DBHost,
		Port:     env.DBPort,
		User:     env.DBUser,
		Password: env.DBPassword,
		DBName:   env.DBName,
		SSLMode:  env.DBSSLMode,
	})
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer platformdatabase.Close(db)

	if err := platformdatabase.RunMigrations(context.Background(), db); err != nil {
		log.Printf("Warning during automatic database migration: %v", err)
	}

	redisClient, err := platformredis.Connect(context.Background(), env.RedisURL)
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v. Background queue will operate in direct fallback mode.", err)
	} else {
		defer platformredis.Close(redisClient)
	}

	credentialService := identityservice.NewCredentialService(identityrepository.NewPostgresRepository(db))
	if err := credentialService.MigrateCredentialKeys(context.Background()); err != nil {
		log.Fatalf("Credential key migration failed: %v", err)
	}
	if err := credentialService.EnsureMasterAdmin(context.Background(), env.MasterAdminKey); err != nil {
		log.Fatalf("Master admin bootstrap failed: %v", err)
	}

	handler := app.NewHandler(env, db, redisClient, credentialService)

	server := &http.Server{
		Addr:         ":" + env.AppPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", env.AppPort)
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
