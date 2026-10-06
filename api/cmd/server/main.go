// Commande server lance l'API Microdash.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // fuseau Europe/Paris, absent de l'image alpine

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/alertes"
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

	sig, err := signature()
	if err != nil {
		return err
	}
	if err := lancerAlertes(ctx, st, set, sig); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           httpapi.New(st, set, env("COOKIE_SECURE", "true") == "true", sig).Routes(),
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

// signature lit le secret des liens de désinscription. Sans ALERTES_SECRET, un secret
// aléatoire sert jusqu'au redémarrage (aucun e-mail ne part alors, voir lancerAlertes).
func signature() (alertes.Signature, error) {
	if v := os.Getenv("ALERTES_SECRET"); v != "" {
		if len(v) < 32 {
			return nil, errors.New("ALERTES_SECRET doit faire au moins 32 caractères")
		}
		return alertes.Signature(v), nil
	}
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return alertes.Signature(b), err
}

// lancerAlertes démarre la tâche des rappels du matin si Resend est configuré.
func lancerAlertes(ctx context.Context, st *store.Store, set *bareme.Set, sig alertes.Signature) error {
	cle := os.Getenv("RESEND_API_KEY")
	if cle == "" {
		slog.Info("rappels par e-mail désactivés : RESEND_API_KEY vide")
		return nil
	}
	manque := []string{}
	for _, v := range []string{"ALERTES_SECRET", "ALERTES_EXPEDITEUR", "SITE_URL"} {
		if os.Getenv(v) == "" {
			manque = append(manque, v)
		}
	}
	if len(manque) > 0 {
		return fmt.Errorf("RESEND_API_KEY est défini, il manque aussi : %s", strings.Join(manque, ", "))
	}
	var limiter []string
	if v := os.Getenv("ALERTES_LIMITER_A"); v != "" {
		limiter = strings.Split(v, ",")
	}
	svc := &alertes.Service{
		Source: st, Baremes: set, Signature: sig, LimiterA: limiter,
		Envoyeur: &alertes.Resend{Cle: cle, De: os.Getenv("ALERTES_EXPEDITEUR")},
		AppURL:   strings.TrimSuffix(os.Getenv("SITE_URL"), "/"),
	}
	slog.Info("rappels par e-mail activés", "chaque_jour", "8 h, heure de Paris", "limites_a", len(limiter))
	go svc.Planifier(ctx)
	return nil
}
