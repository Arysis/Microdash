package notifs

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
	"github.com/arysis/microdash/api/internal/webpush"
)

// tout : les deux notifications activées, à 18 h et 15 min avant.
var tout = store.PreferencesNotifications{Soiree: true, HeureSoiree: "18:00", RendezVous: true, Avance: 15}

func a(jour, heure string) time.Time {
	t, _ := aHeure(jour, heure)
	return t
}

var agenda = []modules.Evenement{
	{Source: "taches", Ref: "1-2026-11-20", Titre: "Rig du canard", Date: "2026-11-20", Type: "tache"},
	{Source: "taches", Ref: "2-2026-11-20", Titre: "Feu de camp", Date: "2026-11-20", Type: "tache", Fait: true},
	{Source: "taches", Ref: "3-2026-11-20", Titre: "Caméra", Date: "2026-11-20", Type: "tache"},
	{Source: "agenda", Ref: "7", Titre: "Dentiste", Date: "2026-11-20", Heure: "10:00", Type: "evenement"},
	{Source: "agenda", Ref: "8", Titre: "Appel Paul", Date: "2026-11-20", Heure: "20:30", Notes: "Devis Educari\nLe reste", Type: "evenement"},
	{Source: "agenda", Ref: "9", Titre: "Anniversaire", Date: "2026-11-20", Type: "evenement"},
	{Source: "finances", Ref: "urssaf", Titre: "Déclaration URSSAF", Date: "2026-11-20", Type: "echeance"},
	{Source: "agenda", Ref: "10", Titre: "Train", Date: "2026-11-21", Heure: "00:15", Type: "evenement"},
}

func TestSoiree(t *testing.T) {
	p := tout
	if ns := Choisir(agenda, p, a("2026-11-20", "17:59")); len(ns) != 0 {
		t.Fatalf("avant l'heure : %+v", ns)
	}
	ns := Choisir(agenda, p, a("2026-11-20", "18:00"))
	if len(ns) != 1 || ns[0].Cle != "soiree:2026-11-20" || ns[0].Titre != "Ta soirée : 2 tâches" || ns[0].Lien != "/ce-soir" {
		t.Fatalf("soirée : %+v", ns)
	}
	// Tâches faites et rendez-vous du matin exclus, 5 lignes ramenées à 4.
	lignes := strings.Split(ns[0].Corps, "\n")
	attendu := []string{"Rig du canard", "Caméra", "20:30 Appel Paul", "et 2 autres"}
	if strings.Join(lignes, "|") != strings.Join(attendu, "|") {
		t.Errorf("corps : %q", lignes)
	}
	if ns[0].TTL != fenetreSoiree {
		t.Errorf("TTL %v", ns[0].TTL)
	}
	// Rattrapage jusqu'à 3 h après, puis abandon.
	if ns := Choisir(agenda, p, a("2026-11-20", "20:59")); len(ns) != 1 {
		t.Errorf("rattrapage : %+v", ns)
	}
	if ns := Choisir(agenda, p, a("2026-11-20", "21:00")); len(ns) != 0 {
		t.Errorf("trop tard : %+v", ns)
	}
	// Rien à faire : pas de notification.
	if ns := Choisir(agenda[1:2], p, a("2026-11-20", "18:30")); len(ns) != 0 {
		t.Errorf("rien : %+v", ns)
	}
	p.Soiree = false
	if ns := Choisir(agenda, p, a("2026-11-20", "18:00")); len(ns) != 0 {
		t.Errorf("désactivé : %+v", ns)
	}
}

func TestRendezVous(t *testing.T) {
	p := tout
	p.Soiree = false
	if ns := Choisir(agenda, p, a("2026-11-20", "20:14")); len(ns) != 0 {
		t.Fatalf("trop tôt : %+v", ns)
	}
	ns := Choisir(agenda, p, a("2026-11-20", "20:15"))
	if len(ns) != 1 || ns[0].Titre != "Appel Paul" || ns[0].Corps != "À 20:30 (dans 15 min)\nDevis Educari" || ns[0].Urgence != "high" {
		t.Fatalf("rappel : %+v", ns)
	}
	if ns[0].Cle != "rdv:agenda-8:2026-11-20T20:30" || ns[0].TTL != 45*time.Minute {
		t.Errorf("clé ou TTL : %+v", ns[0])
	}
	if ns := Choisir(agenda, p, a("2026-11-20", "20:35")); len(ns) != 1 || ns[0].Corps != "À 20:30, c'est maintenant\nDevis Educari" {
		t.Errorf("en retard : %+v", ns)
	}
	if ns := Choisir(agenda, p, a("2026-11-20", "20:40")); len(ns) != 0 {
		t.Errorf("passé : %+v", ns)
	}
	// Juste après minuit : rappelé la veille au soir.
	if ns := Choisir(agenda, p, a("2026-11-21", "00:00")); len(ns) != 1 || ns[0].Titre != "Train" {
		t.Errorf("minuit : %+v", ns)
	}
	p.Avance = 60
	if ns := Choisir(agenda, p, a("2026-11-20", "09:00")); len(ns) != 1 || ns[0].Corps != "À 10:00 (dans 1 h)" {
		t.Errorf("1 h avant : %+v", ns)
	}
	p.RendezVous = false
	if ns := Choisir(agenda, p, a("2026-11-20", "20:15")); len(ns) != 0 {
		t.Errorf("désactivé : %+v", ns)
	}
}

// --- Service, avec une fausse base et un faux service de push ---

type fausseBase struct {
	abs      []store.AbonnementPush
	envoyees map[string]bool
	oublies  []int64
	erreurs  map[int64]string
}

func (f *fausseBase) Abonnements(_ context.Context, uid int64) ([]store.AbonnementPush, error) {
	var res []store.AbonnementPush
	for _, a := range f.abs {
		if uid == 0 || a.UserID == uid {
			res = append(res, a)
		}
	}
	return res, nil
}
func (f *fausseBase) PreferencesNotifications(_ context.Context, uid int64) (store.PreferencesNotifications, error) {
	if uid == 6 {
		return store.PreferencesNotificationsDefaut, nil // rien choisi : tout coupé
	}
	return tout, nil
}
func (f *fausseBase) ReserverNotification(_ context.Context, _ int64, cle string) (bool, error) {
	if f.envoyees[cle] {
		return false, nil
	}
	f.envoyees[cle] = true
	return true, nil
}
func (f *fausseBase) AnnulerNotification(_ context.Context, _ int64, cle string) error {
	delete(f.envoyees, cle)
	return nil
}
func (f *fausseBase) OublierAbonnement(_ context.Context, id int64) error {
	f.oublies = append(f.oublies, id)
	return nil
}
func (f *fausseBase) NoterErreurAbonnement(_ context.Context, id int64, msg string) error {
	f.erreurs[id] = msg
	for i := range f.abs {
		if f.abs[i].ID == id {
			f.abs[i].Erreur = msg
		}
	}
	return nil
}
func (f *fausseBase) PurgerNotifications(context.Context) error { return nil }

type fauxPush struct {
	recus   map[string]int
	reponse map[string]error
}

func (p *fauxPush) Envoyer(_ context.Context, ab webpush.Abonnement, _ []byte, _ webpush.Options) error {
	if err := p.reponse[ab.Endpoint]; err != nil {
		return err
	}
	p.recus[ab.Endpoint]++
	return nil
}

func TestService(t *testing.T) {
	base := &fausseBase{envoyees: map[string]bool{}, erreurs: map[int64]string{}, abs: []store.AbonnementPush{
		{ID: 1, UserID: 5, Endpoint: "pc"}, {ID: 2, UserID: 5, Endpoint: "telephone"}, {ID: 3, UserID: 5, Endpoint: "vieux"},
		{ID: 4, UserID: 6, Endpoint: "defaut"},
	}}
	push := &fauxPush{recus: map[string]int{}, reponse: map[string]error{"vieux": webpush.ErrExpire}}
	svc := &Service{Donnees: base, Push: push, Agenda: func(_ context.Context, uid int64, du, au time.Time) ([]modules.Evenement, error) {
		if uid == 6 {
			t.Error("compte sans notification choisie : agenda lu quand même")
		}
		if uid != 5 || du.Format("2006-01-02") != "2026-11-20" || au.Format("2006-01-02") != "2026-11-21" {
			t.Errorf("agenda demandé : %d %v %v", uid, du, au)
		}
		return agenda, nil
	}}
	for _, h := range []string{"18:00", "18:01"} {
		if err := svc.Tourner(context.Background(), a("2026-11-20", h)); err != nil {
			t.Fatal(err)
		}
	}
	// Une seule fois par appareil, l'appareil expiré oublié.
	if push.recus["defaut"] != 0 {
		t.Error("notification envoyée sans l'avoir choisie")
	}
	if push.recus["pc"] != 1 || push.recus["telephone"] != 1 || len(base.oublies) != 1 || base.oublies[0] != 3 {
		t.Errorf("reçus %v, oubliés %v", push.recus, base.oublies)
	}

	// Aucun appareil atteint : la réservation est levée pour réessayer.
	push.reponse = map[string]error{"pc": errTest, "telephone": errTest}
	base.abs = base.abs[:2] // l'appareil expiré est oublié, le compte 6 ne compte plus
	if err := svc.Tourner(context.Background(), a("2026-11-20", "20:15")); err != nil {
		t.Fatal(err)
	}
	if base.envoyees["rdv:agenda-8:2026-11-20T20:30"] || base.erreurs[1] == "" {
		t.Errorf("échec : réservations %v, erreurs %v", base.envoyees, base.erreurs)
	}
	push.reponse = nil
	_ = svc.Tourner(context.Background(), a("2026-11-20", "20:16"))
	if !base.envoyees["rdv:agenda-8:2026-11-20T20:30"] || push.recus["pc"] != 2 || base.erreurs[1] != "" {
		t.Errorf("nouvel essai : %v %v %v", base.envoyees, push.recus, base.erreurs)
	}
}

type erreurTest struct{}

func (erreurTest) Error() string { return "panne" }

var errTest = erreurTest{}
