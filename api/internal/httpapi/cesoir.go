package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/arysis/microdash/api/internal/alertes"
	"github.com/arysis/microdash/api/internal/modules"
)

// --- Modules du copilote ---

type moduleJSON struct {
	Code  string `json:"code"`
	Nom   string `json:"nom"`
	Actif bool   `json:"actif"`
}

func (s *Server) listModules(w http.ResponseWriter, r *http.Request) {
	inactifs, err := s.Store.ModulesInactifs(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := []moduleJSON{}
	for _, m := range s.Modules.Modules() {
		res = append(res, moduleJSON{Code: m.Code(), Nom: m.Nom(), Actif: !inactifs[m.Code()]})
	}
	ecrireJSON(w, http.StatusOK, res)
}

func (s *Server) putModule(w http.ResponseWriter, r *http.Request) {
	m, ok := s.Modules.Module(chi.URLParam(r, "code"))
	if !ok {
		erreur(w, http.StatusNotFound, "module inconnu")
		return
	}
	var in struct {
		Actif bool `json:"actif"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.ActiverModule(r.Context(), userID(r.Context()), m.Code(), in.Actif); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, moduleJSON{Code: m.Code(), Nom: m.Nom(), Actif: in.Actif})
}

// --- Ce soir ---

type sourceJSON struct {
	Code   string `json:"code"`
	Nom    string `json:"nom"`
	Nombre int    `json:"nombre"`
}

// ceSoir rassemble les éléments à traiter de tous les modules actifs. Un module en erreur
// n'empêche pas les autres de s'afficher : il est nommé dans « indisponibles ».
func (s *Server) ceSoir(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r.Context())
	aujourdhui := alertes.Aujourdhui(s.now())
	inactifs, err := s.Store.ModulesInactifs(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	etats, err := s.Store.EtatsATraiter(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	sources := []sourceJSON{}
	indisponibles := []string{}
	elements := []modules.Element{}
	for _, mod := range s.Modules.Modules() {
		m, ok := mod.(modules.Source)
		if !ok || inactifs[m.Code()] {
			continue
		}
		els, err := m.ATraiter(ctx, uid, aujourdhui)
		if err != nil {
			slog.Error("ce soir : module", "module", m.Code(), "compte", uid, "err", err)
			indisponibles = append(indisponibles, m.Nom())
			continue
		}
		els = modules.Filtrer(els, etats, aujourdhui)
		sources = append(sources, sourceJSON{Code: m.Code(), Nom: m.Nom(), Nombre: len(els)})
		elements = append(elements, els...)
	}
	modules.Trier(elements)
	ecrireJSON(w, http.StatusOK, map[string]any{
		"aujourdhui": aujourdhui.Format(formatDate), "sources": sources,
		"elements": elements, "indisponibles": indisponibles,
	})
}

type decisionJSON struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Quand  string `json:"quand,omitempty"` // plus tard : demain, week-end, semaine
}

func (s *Server) lireDecision(w http.ResponseWriter, r *http.Request) (decisionJSON, modules.Module, bool) {
	var in decisionJSON
	if !lireJSON(w, r, &in) {
		return in, nil, false
	}
	m, ok := s.Modules.Module(in.Source)
	if !ok || in.Ref == "" || len(in.Ref) > 200 {
		erreur(w, http.StatusBadRequest, "élément inconnu")
		return in, nil, false
	}
	return in, m, true
}

// faitCeSoir marque un élément « Fait » : il quitte la liste, et le module agit s'il le doit
// (une échéance est cochée dans l'agenda).
func (s *Server) faitCeSoir(w http.ResponseWriter, r *http.Request) {
	in, m, ok := s.lireDecision(w, r)
	if !ok {
		return
	}
	ctx, uid := r.Context(), userID(r.Context())
	if f, ok := m.(modules.Finisseur); ok {
		if err := f.Finir(ctx, uid, in.Ref); err != nil {
			slog.Error("ce soir : fait", "module", m.Code(), "compte", uid, "err", err)
			erreur(w, http.StatusBadGateway, m.Nom()+" : "+err.Error())
			return
		}
	}
	if err := s.Store.MarquerATraiter(ctx, uid, m.Code(), in.Ref, nil); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) plusTardCeSoir(w http.ResponseWriter, r *http.Request) {
	in, m, ok := s.lireDecision(w, r)
	if !ok {
		return
	}
	jour, err := modules.Reporter(in.Quand, alertes.Aujourdhui(s.now()))
	if err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.MarquerATraiter(r.Context(), userID(r.Context()), m.Code(), in.Ref, &jour); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]string{"jusqua": jour.Format(formatDate)})
}
