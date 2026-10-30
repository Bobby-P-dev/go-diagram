package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/config"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	emailFlag := flag.String("email", "", "account email")
	name := flag.String("name", "Developer", "display name")
	output := flag.String("password-file", "", "new private output file outside the repository")
	existing := flag.String("existing-user-id", "", "optional existing identity ID to retain project ownership")
	flag.Parse()
	email, err := identity.NormalizeEmail(*emailFlag)
	if err != nil {
		return err
	}
	if *output == "" || len(strings.TrimSpace(*name)) == 0 || len(*name) > 150 {
		return fmt.Errorf("name and password-file are required")
	}
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		file.Close()
		if !success {
			os.Remove(*output)
		}
	}()
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(secret)
	hash, err := identity.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	disabledKey := identity.HashCredentialKey(base64.RawURLEncoding.EncodeToString(secret))
	if _, err = fmt.Fprintf(file, "Email: %s\nPassword: %s\n", email, password); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	env := config.LoadEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Connect(ctx, database.Config{Host: env.DBHost, Port: env.DBPort, User: env.DBUser, Password: env.DBPassword, DBName: env.DBName, SSLMode: env.DBSSLMode})
	if err != nil {
		return err
	}
	defer db.Close()
	userID, err := repository.ProvisionPasswordAccount(ctx, db, repository.ProvisionAccount{Email: email, Name: strings.TrimSpace(*name), PasswordHash: hash, DisabledKeyHash: disabledKey, ExistingUserID: *existing})
	if err != nil {
		return err
	}
	success = true
	fmt.Printf("Account created: %s (identity %s). Credentials saved privately to %s\n", email, userID, *output)
	return nil
}
