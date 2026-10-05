// Package baremes embarque les fichiers de barème annuels dans le binaire.
package baremes

import "embed"

//go:embed *.yaml
var FS embed.FS
