package store

import (
	"testing"
	"time"

	"github.com/arysis/microdash/api/internal/calc"
)

func TestRegleSaisie(t *testing.T) {
	d := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	cats := []string{"services_bic", "vente_bic"}
	rec := OperationQonto{Type: calc.Recette, Date: d, Centimes: 1000, Nom: "Client"}
	dep := OperationQonto{Type: calc.Depense, Date: d, Centimes: 500, Nom: "URSSAF", EcheanceProposee: "urssaf-2026-09"}

	if s, ok := (Regle{Action: "valider", Categorie: "vente_bic"}).Saisie(rec, cats); !ok || s.Categorie != "vente_bic" || s.Libelle != "Client" {
		t.Errorf("recette : %+v %v", s, ok)
	}
	// Activité retirée du profil depuis : la principale.
	if s, ok := (Regle{Action: "valider", Categorie: "liberal_ssi"}).Saisie(rec, cats); !ok || s.Categorie != "services_bic" {
		t.Errorf("activité disparue : %+v %v", s, ok)
	}
	if _, ok := (Regle{Action: "valider", Poste: "Matériel"}).Saisie(rec, cats); ok {
		t.Error("une règle de dépense ne vaut pas pour un crédit")
	}
	if s, ok := (Regle{Action: "valider", Poste: calc.PosteURSSAF}).Saisie(dep, cats); !ok || s.Echeance != "urssaf-2026-09" || s.Poste != calc.PosteURSSAF {
		t.Errorf("URSSAF : %+v %v", s, ok)
	}
	if s, ok := (Regle{Action: "valider", Poste: "Matériel"}).Saisie(dep, cats); !ok || s.Echeance != "" {
		t.Errorf("dépense : %+v %v", s, ok)
	}
}
