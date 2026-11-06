package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListeMdpOublie(t *testing.T) {
	if l := ListeMdpOublie(""); len(l) != 0 {
		t.Fatalf("vide : %v", l)
	}
	l := ListeMdpOublie(" Moi@Exemple.fr , ,autre@x.fr")
	if len(l) != 2 || l[0] != "moi@exemple.fr" || l[1] != "autre@x.fr" {
		t.Fatalf("liste : %v", l)
	}
	s := &Server{MdpOublie: l}
	if !s.mdpOublieAutorise("moi@exemple.fr") || s.mdpOublieAutorise("intrus@x.fr") {
		t.Fatal("autorisation par adresse")
	}
	s.MdpOublie = []string{"*"}
	if !s.mdpOublieAutorise("intrus@x.fr") {
		t.Fatal("« * » doit tout autoriser")
	}
}

func TestMdpOublieRefuse(t *testing.T) {
	cas := []struct {
		nom    string
		liste  []string
		corps  string
		statut int
	}{
		{"désactivé", nil, `{"email":"moi@exemple.fr","mot_de_passe":"0123456789"}`, http.StatusNotFound},
		{"mot de passe court", []string{"*"}, `{"email":"moi@exemple.fr","mot_de_passe":"court"}`, http.StatusBadRequest},
		{"adresse non listée", []string{"moi@exemple.fr"}, `{"email":"intrus@x.fr","mot_de_passe":"0123456789"}`, http.StatusForbidden},
	}
	for _, c := range cas {
		s := &Server{MdpOublie: c.liste}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/auth/mot-de-passe-oublie", strings.NewReader(c.corps))
		r.Header.Set("Content-Type", "application/json")
		s.mdpOublie(w, r)
		if w.Code != c.statut {
			t.Errorf("%s : statut %d, attendu %d", c.nom, w.Code, c.statut)
		}
	}
}
