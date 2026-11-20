// Package webpush envoie des notifications Web Push (RFC 8030) chiffrées (RFC 8291) et signées
// VAPID (RFC 8292), avec la seule bibliothèque standard. Les services de push des navigateurs
// (Google, Mozilla, Apple, Microsoft) sont gratuits : aucune clé d'API n'est nécessaire.
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Abonnement est ce que le navigateur donne quand la personne accepte les notifications.
type Abonnement struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"` // clé publique du navigateur, base64url
	Auth     string `json:"auth"`   // secret d'authentification, base64url
}

// ErrExpire signale un abonnement que le service de push ne connaît plus (404 ou 410) :
// il faut l'oublier.
var ErrExpire = errors.New("abonnement expiré")

var b64 = base64.RawURLEncoding

// decoder accepte le base64url avec ou sans « = », et le base64 classique.
func decoder(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return b64.DecodeString(s)
}

// --- Clés VAPID du serveur ---

// Cles est la paire de clés VAPID du serveur : le navigateur n'accepte que les messages signés par elle.
type Cles struct{ prive *ecdsa.PrivateKey }

// NouvellesCles crée une paire de clés P-256.
func NouvellesCles() (*Cles, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Cles{prive: k}, nil
}

// Exporter renvoie la clé privée au format PKCS #8, pour la garder en base.
func (c *Cles) Exporter() ([]byte, error) { return x509.MarshalPKCS8PrivateKey(c.prive) }

// Importer relit une clé gardée avec Exporter.
func Importer(der []byte) (*Cles, error) {
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	e, ok := k.(*ecdsa.PrivateKey)
	if !ok || e.Curve != elliptic.P256() {
		return nil, errors.New("clé VAPID : P-256 attendue")
	}
	return &Cles{prive: e}, nil
}

// Publique renvoie la clé publique en base64url (point non compressé de 65 octets) :
// c'est l'applicationServerKey que le navigateur demande.
func (c *Cles) Publique() string {
	p, err := c.prive.PublicKey.ECDH()
	if err != nil {
		return ""
	}
	return b64.EncodeToString(p.Bytes())
}

// jwt signe le jeton VAPID pour le service de push (aud) : valable 12 heures.
func (c *Cles) jwt(aud, sujet string, maintenant time.Time) (string, error) {
	entete := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	corps, err := json.Marshal(map[string]any{"aud": aud, "exp": maintenant.Add(12 * time.Hour).Unix(), "sub": sujet})
	if err != nil {
		return "", err
	}
	signe := entete + "." + b64.EncodeToString(corps)
	h := sha256.Sum256([]byte(signe))
	r, s, err := ecdsa.Sign(rand.Reader, c.prive, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signe + "." + b64.EncodeToString(sig), nil
}

// --- Chiffrement du message (RFC 8291, aes128gcm) ---

const tailleEnregistrement = 4096

// Chiffrer chiffre le message pour le navigateur de l'abonnement.
func Chiffrer(ab Abonnement, message []byte) ([]byte, error) {
	ephemere, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	sel := make([]byte, 16)
	if _, err := rand.Read(sel); err != nil {
		return nil, err
	}
	return chiffrer(ab, message, ephemere, sel)
}

func chiffrer(ab Abonnement, message []byte, ephemere *ecdh.PrivateKey, sel []byte) ([]byte, error) {
	brut, err := decoder(ab.P256dh)
	if err != nil {
		return nil, fmt.Errorf("clé p256dh : %w", err)
	}
	navigateur, err := ecdh.P256().NewPublicKey(brut)
	if err != nil {
		return nil, fmt.Errorf("clé p256dh : %w", err)
	}
	auth, err := decoder(ab.Auth)
	if err != nil || len(auth) != 16 {
		return nil, errors.New("secret auth invalide (16 octets attendus)")
	}
	if len(message) > tailleEnregistrement-16-1-86 {
		return nil, errors.New("message trop long pour une notification")
	}
	partage, err := ephemere.ECDH(navigateur)
	if err != nil {
		return nil, err
	}
	serveur := ephemere.PublicKey().Bytes()

	info := append([]byte("WebPush: info\x00"), brut...)
	info = append(info, serveur...)
	ikm, err := hkdf.Key(sha256.New, partage, auth, string(info), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, sel, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, sel, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	bloc, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(bloc)
	if err != nil {
		return nil, err
	}
	// Un seul enregistrement : le message suivi du délimiteur de fin 0x02.
	clair := append(append([]byte{}, message...), 0x02)

	var b bytes.Buffer
	b.Write(sel)
	_ = binary.Write(&b, binary.BigEndian, uint32(tailleEnregistrement))
	b.WriteByte(byte(len(serveur)))
	b.Write(serveur)
	b.Write(gcm.Seal(nil, nonce, clair, nil))
	return b.Bytes(), nil
}

// --- Envoi ---

// Envoyeur pousse les notifications vers les services de push des navigateurs.
type Envoyeur struct {
	Cles   *Cles
	Sujet  string // contact pour les services de push : « mailto:… » ou l'adresse du site
	Client *http.Client
	now    func() time.Time
}

// Options d'un envoi.
type Options struct {
	TTL     time.Duration // durée pendant laquelle le service garde le message si l'appareil est éteint
	Urgence string        // very-low, low, normal, high ; vide : normal
	Sujet   string        // remplace un message en attente du même sujet (32 caractères base64url au plus)
}

// HoteAutorise dit si l'adresse d'un abonnement est bien celle d'un service de push connu :
// le serveur n'envoie rien ailleurs (pas de requête vers une adresse choisie par un tiers).
func HoteAutorise(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, d := range []string{"googleapis.com", "mozilla.com", "mozaws.net", "notify.windows.com", "push.apple.com"} {
		if h == d || strings.HasSuffix(h, "."+d) {
			return true
		}
	}
	return false
}

// Envoyer chiffre et envoie un message. ErrExpire : l'abonnement est à oublier.
func (e *Envoyeur) Envoyer(ctx context.Context, ab Abonnement, message []byte, o Options) error {
	if !HoteAutorise(ab.Endpoint) {
		return errors.New("service de push inconnu")
	}
	u, _ := url.Parse(ab.Endpoint)
	now := time.Now
	if e.now != nil {
		now = e.now
	}
	jeton, err := e.Cles.jwt(u.Scheme+"://"+u.Host, e.Sujet, now())
	if err != nil {
		return err
	}
	corps, err := Chiffrer(ab, message)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ab.Endpoint, bytes.NewReader(corps))
	if err != nil {
		return err
	}
	ttl := o.TTL
	if ttl <= 0 {
		ttl = 4 * time.Hour
	}
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "vapid t="+jeton+", k="+e.Cles.Publique())
	if o.Urgence != "" {
		req.Header.Set("Urgency", o.Urgence)
	}
	if o.Sujet != "" {
		req.Header.Set("Topic", o.Sujet)
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(res.Body, 300))
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrExpire
	case res.StatusCode >= 300:
		return fmt.Errorf("service de push : %s %s", res.Status, strings.TrimSpace(string(detail)))
	}
	return nil
}
