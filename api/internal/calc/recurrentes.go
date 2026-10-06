package calc

import "time"

// Annuelle est la fréquence d'une dépense qui revient chaque année ; Mensuelle sert aussi ici.
const Annuelle = "annuelle"

// Recurrente est une dépense qui revient chaque mois ou chaque année, au jour de sa première date.
type Recurrente struct {
	Centimes  int64
	Frequence string
	Debut     time.Time
	Fin       *time.Time // dernière date possible, incluse ; nil si sans fin
}

// jourDansMois place le jour j dans le mois, ramené au dernier jour si le mois est plus court (31 → 30 avril).
func jourDansMois(a int, m time.Month, j int) time.Time {
	dernier := time.Date(a, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if j > dernier {
		j = dernier
	}
	return time.Date(a, m, j, 0, 0, 0, 0, time.UTC)
}

// Occurrences renvoie les dates de la dépense comprises dans [du, au].
func (r Recurrente) Occurrences(du, au time.Time) []time.Time {
	var res []time.Time
	pas := 1
	if r.Frequence == Annuelle {
		pas = 12
	}
	for i := 0; ; i += pas {
		premier := time.Date(r.Debut.Year(), r.Debut.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		d := jourDansMois(premier.Year(), premier.Month(), r.Debut.Day())
		if d.After(au) || (r.Fin != nil && d.After(*r.Fin)) {
			return res
		}
		if !d.Before(du) {
			res = append(res, d)
		}
	}
}
