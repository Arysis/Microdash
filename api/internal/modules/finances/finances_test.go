package finances

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

func jour(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

type fausses struct {
	profil   *store.Profil
	txs      []store.Transaction
	faites   map[string]time.Time
	aValider int
	marquees []string
}

func (f *fausses) Profil(context.Context, int64) (*store.Profil, error) {
	if f.profil == nil {
		return nil, store.ErrNotFound
	}
	return f.profil, nil
}
func (f *fausses) Transactions(context.Context, int64, time.Time, time.Time) ([]store.Transaction, error) {
	return f.txs, nil
}
func (f *fausses) EcheancesFaites(context.Context, int64) (map[string]time.Time, error) {
	return f.faites, nil
}
func (f *fausses) MarquerEcheance(_ context.Context, _ int64, code string, _ bool) error {
	f.marquees = append(f.marquees, code)
	return nil
}
func (f *fausses) CompterAValider(context.Context, int64) (int, error) { return f.aValider, nil }

func module(t *testing.T, f *fausses) Module {
	set, err := bareme.Load(baremes.FS)
	if err != nil {
		t.Fatal(err)
	}
	return Module{Donnees: f, Baremes: set}
}

func parRef(els []modules.Element) map[string]modules.Element {
	res := map[string]modules.Element{}
	for _, e := range els {
		res[e.Ref] = e
	}
	return res
}

func TestSansProfil(t *testing.T) {
	els, err := module(t, &fausses{}).ATraiter(context.Background(), 1, jour("2026-10-26"))
	if err != nil || len(els) != 0 {
		t.Fatalf("ATraiter sans profil = %+v, %v", els, err)
	}
}

func TestATraiter(t *testing.T) {
	f := &fausses{
		profil:   &store.Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: "trimestrielle"},
		txs:      []store.Transaction{{Type: "recette", Date: jour("2026-08-10"), Centimes: 7000000, Categorie: "services_bic"}},
		faites:   map[string]time.Time{},
		aValider: 3,
	}
	m := module(t, f)
	els, err := m.ATraiter(context.Background(), 1, jour("2026-10-26"))
	if err != nil {
		t.Fatal(err)
	}
	refs := parRef(els)

	t3, ok := refs["urssaf-2026-t3"]
	if !ok {
		t.Fatalf("déclaration du 3e trimestre absente : %+v", els)
	}
	if t3.Importance != 1 || !t3.Finir || t3.Lien != "/agenda" || !strings.HasPrefix(t3.Detail, "Chiffre d'affaires à déclarer") {
		t.Errorf("déclaration T3 = %+v", t3)
	}
	if _, ok := refs["urssaf-2026-t4"]; ok {
		t.Error("la déclaration de janvier ne doit pas encore apparaître")
	}

	q, ok := refs[refQonto]
	if !ok || q.Titre != "3 opérations Qonto à valider" || q.Finir {
		t.Errorf("opérations Qonto = %+v", q)
	}

	plafond := false
	for ref, e := range refs {
		if strings.HasPrefix(ref, prefixePlaf) {
			plafond = true
			if e.Importance == 0 || !strings.Contains(ref, "-2026-") {
				t.Errorf("plafond = %+v", e)
			}
		}
	}
	if !plafond {
		t.Error("70 000 € de services BIC : au moins un seuil devrait être signalé")
	}

	// Une échéance cochée dans l'agenda quitte « Ce soir ».
	f.faites["urssaf-2026-t3"] = jour("2026-10-20")
	els, _ = m.ATraiter(context.Background(), 1, jour("2026-10-26"))
	if _, ok := parRef(els)["urssaf-2026-t3"]; ok {
		t.Error("échéance faite encore affichée")
	}
}

func TestFinir(t *testing.T) {
	f := &fausses{}
	m := module(t, f)
	for _, ref := range []string{"urssaf-2026-t3", refQonto, "plafond-2026-tva_services-depasse"} {
		if err := m.Finir(context.Background(), 1, ref); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.marquees) != 1 || f.marquees[0] != "urssaf-2026-t3" {
		t.Fatalf("échéances cochées = %v", f.marquees)
	}
}
