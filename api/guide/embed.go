// Package guide embarque le guide utilisateur (GUIDE.md) : servi à la page Aide de l'app
// (/api/guide) et à Claude par l'outil MCP lire_guide.
package guide

import _ "embed"

//go:embed GUIDE.md
var Texte string
