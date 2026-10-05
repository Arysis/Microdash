// Commande server lance l'API Microdash.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/httpapi"
	"github.com/arysis/microdash/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("arrêt", "err", err)
		os.Exit(1)
	}
}

func env(cle, defaut string) string {
	if v := os.Getenv(cle); v != "" {
		return v
	}
	return defaut
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Les barèmes embarqués servent par défaut ; BAREMES_DIR permet de les remplacer sans recompiler.
	var source fs.FS = baremes.FS
	if dir := os.Getenv("BAREMES_DIR"); dir != "" {
		source = os.DirFS(dir)
	}
	set, err := bareme.Load(source)
	if err != nil {
		return err
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL manquante")
	}
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           httpapi.New(st, set, env("COOKIE_SECURE", "true") == "true").Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		arret, annuler := context.WithTimeout(context.Background(), 10*time.Second)
		defer annuler()
		_ = srv.Shutdown(arret)
	}()
	slog.Info("API démarrée", "adresse", srv.Addr, "baremes", set.Annees())
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
