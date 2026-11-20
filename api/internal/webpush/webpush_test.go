package webpush

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Exemple de l'annexe A de la RFC 8291.
const (
	rfcClair    = "V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24"
	rfcASPrive  = "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"
	rfcUAPublic = "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	rfcUAPrive  = "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"
	rfcSel      = "DGv6ra1nlYgDCS1FRnbzlw"
	rfcAuth     = "BTBZMqHH6r4Tts7J_aSIgg"
	rfcResultat = "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
)

func dec(t *testing.T, s string) []byte {
	t.Helper()
	b, err := decoder(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChiffrerRFC8291(t *testing.T) {
	clair := dec(t, rfcClair)
	as, err := ecdh.P256().NewPrivateKey(dec(t, rfcASPrive))
	if err != nil {
		t.Fatal(err)
	}
	ab := Abonnement{Endpoint: "https://push.example.net/x", P256dh: rfcUAPublic, Auth: rfcAuth}
	got, err := chiffrer(ab, clair, as, dec(t, rfcSel))
	if err != nil {
		t.Fatal(err)
	}
	if b64.EncodeToString(got) != rfcResultat {
		t.Fatalf("chiffré différent de la RFC :\n%s\n%s", b64.EncodeToString(got), rfcResultat)
	}
}

// dechiffrer fait le travail du navigateur, pour vérifier un aller-retour.
func dechiffrer(t *testing.T, ua *ecdh.PrivateKey, auth, corps []byte) []byte {
	t.Helper()
	sel, rs, idlen := corps[:16], binary.BigEndian.Uint32(corps[16:20]), int(corps[20])
	if rs != 4096 {
		t.Fatalf("taille d'enregistrement %d", rs)
	}
	serveur := corps[21 : 21+idlen]
	pub, err := ecdh.P256().NewPublicKey(serveur)
	if err != nil {
		t.Fatal(err)
	}
	partage, _ := ua.ECDH(pub)
	info := append(append([]byte("WebPush: info\x00"), ua.PublicKey().Bytes()...), serveur...)
	ikm, _ := hkdf.Key(sha256.New, partage, auth, string(info), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, sel, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, sel, "Content-Encoding: nonce\x00", 12)
	bloc, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(bloc)
	clair, err := gcm.Open(nil, nonce, corps[21+idlen:], nil)
	if err != nil {
		t.Fatal(err)
	}
	if clair[len(clair)-1] != 0x02 {
		t.Fatal("délimiteur de fin absent")
	}
	return clair[:len(clair)-1]
}

func TestEnvoyer(t *testing.T) {
	cles, err := NouvellesCles()
	if err != nil {
		t.Fatal(err)
	}
	// La clé survit à un passage par la base.
	der, err := cles.Exporter()
	if err != nil {
		t.Fatal(err)
	}
	cles, err = Importer(der)
	if err != nil {
		t.Fatal(err)
	}
	ua, _ := ecdh.P256().NewPrivateKey(dec(t, rfcUAPrive))
	auth := dec(t, rfcAuth)
	ab := Abonnement{Endpoint: "https://fcm.googleapis.com/fcm/send/abc", P256dh: b64.EncodeToString(ua.PublicKey().Bytes()), Auth: rfcAuth}
	maintenant := time.Date(2026, 11, 20, 18, 0, 0, 0, time.UTC)

	var recu *http.Request
	var corps []byte
	statut := http.StatusCreated
	env := &Envoyeur{Cles: cles, Sujet: "https://microdash.example", now: func() time.Time { return maintenant },
		Client: &http.Client{Transport: allerRetour(func(r *http.Request) (*http.Response, error) {
			recu = r
			corps, _ = io.ReadAll(r.Body)
			return &http.Response{StatusCode: statut, Status: http.StatusText(statut), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}}

	message := []byte(`{"titre":"Ta soirée","corps":"2 tâches"}`)
	if err := env.Envoyer(context.Background(), ab, message, Options{TTL: time.Hour, Urgence: "high", Sujet: "soiree"}); err != nil {
		t.Fatal(err)
	}
	if got := string(dechiffrer(t, ua, auth, corps)); got != string(message) {
		t.Errorf("message relu : %q", got)
	}
	for cle, val := range map[string]string{"TTL": "3600", "Content-Encoding": "aes128gcm", "Urgency": "high", "Topic": "soiree"} {
		if recu.Header.Get(cle) != val {
			t.Errorf("%s = %q, attendu %q", cle, recu.Header.Get(cle), val)
		}
	}

	// En-tête VAPID : jeton signé par la clé du serveur, pour le bon service, et sa clé publique.
	a := recu.Header.Get("Authorization")
	if !strings.HasPrefix(a, "vapid t=") || !strings.HasSuffix(a, ", k="+cles.Publique()) {
		t.Fatalf("Authorization : %q", a)
	}
	jeton := strings.TrimSuffix(strings.TrimPrefix(a, "vapid t="), ", k="+cles.Publique())
	parts := strings.Split(jeton, ".")
	if len(parts) != 3 {
		t.Fatalf("jeton : %q", jeton)
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	_ = json.Unmarshal(dec(t, parts[1]), &claims)
	if claims.Aud != "https://fcm.googleapis.com" || claims.Sub != "https://microdash.example" || claims.Exp != maintenant.Add(12*time.Hour).Unix() {
		t.Errorf("revendications : %+v", claims)
	}
	sig := dec(t, parts[2])
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(&cles.prive.PublicKey, h[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Error("signature VAPID invalide")
	}

	statut = http.StatusGone
	if err := env.Envoyer(context.Background(), ab, message, Options{}); !errors.Is(err, ErrExpire) {
		t.Errorf("410 : %v, attendu ErrExpire", err)
	}
	statut = http.StatusTooManyRequests
	if err := env.Envoyer(context.Background(), ab, message, Options{}); err == nil || errors.Is(err, ErrExpire) {
		t.Errorf("429 : %v", err)
	}
}

type allerRetour func(*http.Request) (*http.Response, error)

func (f allerRetour) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHoteAutorise(t *testing.T) {
	for adresse, ok := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/x":              true,
		"https://updates.push.services.mozilla.com/wpush/v2": true,
		"https://web.push.apple.com/QGx":                     true,
		"https://wns2-par02p.notify.windows.com/w/?token=x":  true,
		"http://fcm.googleapis.com/x":                        false,
		"https://fcm.googleapis.com:8443/x":                  false,
		"https://evilgoogleapis.com/x":                       false,
		"https://localhost/x":                                false,
		"https://169.254.169.254/latest":                     false,
		"https://user@fcm.googleapis.com/x":                  false,
	} {
		if HoteAutorise(adresse) != ok {
			t.Errorf("%s : %v attendu", adresse, ok)
		}
	}
}
