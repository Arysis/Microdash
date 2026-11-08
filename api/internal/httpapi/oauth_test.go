package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRetourOAuthAutorise(t *testing.T) {
	ok := []string{"https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback", "https://CLAUDE.ai/x?y=1"}
	ko := []string{"http://claude.ai/api/mcp/auth_callback", "https://claude.ai.evil.com/cb", "https://evil.com/claude.ai",
		"https://user@claude.ai/cb", "https://claude.ai:8443/cb", "https://claude.ai/cb#frag", "javascript:alert(1)", "", "https://sub.claude.ai/cb"}
	for _, u := range ok {
		if !retourOAuthAutorise(u) {
			t.Errorf("%q devrait être accepté", u)
		}
	}
	for _, u := range ko {
		if retourOAuthAutorise(u) {
			t.Errorf("%q devrait être refusé", u)
		}
	}
}

func TestVerifierPKCE(t *testing.T) {
	verifier := strings.Repeat("a", 43)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	if !verifierPKCE(verifier, challenge) {
		t.Fatal("verifier correct refusé")
	}
	if verifierPKCE(strings.Repeat("b", 43), challenge) {
		t.Fatal("mauvais verifier accepté")
	}
	if verifierPKCE("court", challenge) {
		t.Fatal("verifier trop court accepté")
	}
}

func TestMetadonnees(t *testing.T) {
	s := &Server{SiteURL: "https://microdash.exemple.fr", now: time.Now}
	h := s.Routes()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res["resource"] != "https://microdash.exemple.fr/mcp" {
		t.Fatalf("ressource : %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res["authorization_endpoint"] != "https://microdash.exemple.fr/autoriser-claude" {
		t.Fatalf("autorisation : %d %s", w.Code, w.Body)
	}
}

func TestMCPSansJeton(t *testing.T) {
	s := &Server{SiteURL: "https://microdash.exemple.fr", now: time.Now}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer pas-un-jeton")
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Header().Get("WWW-Authenticate"), "/.well-known/oauth-protected-resource/mcp") {
		t.Fatalf("statut %d, en-tête %q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
}

func TestOutilsMCP(t *testing.T) {
	s := &Server{now: time.Now}
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.serveurMCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var noms []string
	for _, o := range res.Tools {
		noms = append(noms, o.Name)
		if o.Annotations == nil || o.Annotations.Title == "" || o.Description == "" {
			t.Errorf("%s : titre ou description manquant", o.Name)
		}
		destructif := o.Annotations != nil && o.Annotations.DestructiveHint != nil && *o.Annotations.DestructiveHint
		if destructif != strings.HasPrefix(o.Name, "supprimer_") {
			t.Errorf("%s : destructif = %v", o.Name, destructif)
		}
	}
	slices.Sort(noms)
	attendus := []string{"agenda", "ajouter_rendez_vous", "ajouter_tache", "ce_soir", "lister_taches", "marquer_echeance",
		"modifier_tache", "resume_finances", "supprimer_rendez_vous", "supprimer_tache", "terminer_tache"}
	if !slices.Equal(noms, attendus) {
		t.Fatalf("outils : %v", noms)
	}
	// Sans jeton (transport en mémoire), un outil refuse de s'exécuter.
	r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "ce_soir", Arguments: map[string]any{}})
	if err != nil || !r.IsError {
		t.Fatalf("appel sans compte : %v %+v", err, r)
	}
}
