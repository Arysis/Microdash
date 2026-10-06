package alertes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message est un e-mail en texte simple.
type Message struct {
	A              string
	Sujet          string
	Texte          string
	Desinscription string // lien de désinscription en un clic
}

type Envoyeur interface {
	Envoyer(ctx context.Context, m Message) error
}

// Resend envoie par l'API HTTP de Resend (https://resend.com/docs/api-reference/emails/send-email).
type Resend struct {
	Cle    string
	De     string // expéditeur, ex. « Microdash <rappels@mondomaine.fr> »
	URL    string // vide : https://api.resend.com
	Client *http.Client
}

func (r *Resend) Envoyer(ctx context.Context, m Message) error {
	url := r.URL
	if url == "" {
		url = "https://api.resend.com"
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	corps, err := json.Marshal(map[string]any{
		"from":    r.De,
		"to":      []string{m.A},
		"subject": m.Sujet,
		"text":    m.Texte,
		"headers": map[string]string{
			"List-Unsubscribe":      "<" + m.Desinscription + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/emails", bytes.NewReader(corps))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Cle)
	req.Header.Set("Content-Type", "application/json")
	rep, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rep.Body.Close()
	if rep.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(rep.Body, 2048))
		return fmt.Errorf("resend : %s : %s", rep.Status, bytes.TrimSpace(detail))
	}
	return nil
}
