# DESIGN.md

Toute modification de l'interface (app et vitrine) respecte ce fichier. Un choix qui n'y figure pas y est ajouté avant d'être appliqué.

## Direction

**Un outil de gestion clair et rassurant : la précision de Linear, le ton proche d'Indy.**

Références retenues : [linear.app](https://linear.app) pour la mise en page précise, les alignements stricts et la sobriété ; [indy.fr](https://www.indy.fr) pour le ton proche et rassurant face aux démarches. Fond clair, une seule couleur vive, les chiffres au premier plan.

## Couleurs

Toutes les couleurs sont des variables CSS définies dans `web/src/style.css` (`:root`). Aucune couleur n'est écrite en dur ailleurs. Les rares copies hors CSS (manifeste, `theme-color`, icônes) sont lues depuis `web/theme.js`.

| Rôle | Nom | Code | Variable |
| --- | --- | --- | --- |
| Principale | Safran | `#C2410C` | `--safran` |
| Principale, survol et appui | Safran foncé | `#9A3412` | `--safran-fonce` |
| Principale, fond léger | Safran clair | `#FFF1E8` | `--safran-clair` |
| Accent (recettes, montants positifs, jauges sous le seuil) | Sarcelle | `#0F5E59` | `--sarcelle` |
| Accent, fond léger | Sarcelle clair | `#E6F2F0` | `--sarcelle-clair` |
| Texte | Encre | `#1C1917` | `--encre` |
| Texte secondaire | Pierre | `#57534E` | `--pierre` |
| Bordures et séparateurs | Lin | `#E7E5E4` | `--lin` |
| Fond de page | Papier | `#FAF8F5` | `--papier` |
| Surfaces (cartes, champs) | Blanc | `#FFFFFF` | `--blanc` |
| Erreur et seuil dépassé, uniquement | Brique | `#B91C1C` | `--brique` |
| Fond des écrans principaux | Lin chaud | `#F3F0EB` | `--fond-relief` |

Pas de dégradé. Pas de mode sombre pour l'instant : une seule identité claire.

Contrastes vérifiés (WCAG) : encre sur papier 16,5:1, pierre sur papier 7,2:1, blanc sur safran 5,2:1, sarcelle sur blanc 7,6:1, blanc sur sarcelle 7,6:1, brique sur blanc 6,5:1.

## Typographie

Deux polices, auto-hébergées avec Fontsource (aucun appel à Google Fonts) :

- **Bricolage Grotesque** pour les titres et le logo, en graisses 600 et 700.
- **IBM Plex Sans** pour le texte, les champs et les chiffres, en graisses 400, 500 et 600. Les montants utilisent les chiffres tabulaires (`font-variant-numeric: tabular-nums`).

| Usage | Taille | Interlignage | Police, graisse |
| --- | --- | --- | --- |
| Titre de la vitrine (h1) | 44 px (32 px sous 600 px de large) | 1,1 | Bricolage 700 |
| Titre de page (h1 de l'app) | 28 px | 1,2 | Bricolage 700 |
| Titre de section (h2) | 20 px | 1,3 | Bricolage 600 |
| Chiffre principal (revenu net) | 40 px | 1,1 | Plex 700 |
| Texte courant | 16 px | 1,5 | Plex 400 |
| Libellés, boutons | 15 px | 1,4 | Plex 500 ou 600 |
| Légendes, aides | 13 px | 1,45 | Plex 400 |

## Formes et espacements

- **Un seul rayon d'arrondi : 6 px** (`--rayon`), pour les boutons, cartes, champs, badges, jauges et images. Pas de pilule.
- Espacements sur une grille de 4 px : 4, 8, 12, 16, 24, 32, 48, 64 (`--e1` à `--e8`).
- Bordures de 1 px en lin. Pas d'ombre portée, sauf la fenêtre de confirmation et les écrans principaux.
- **Écrans principaux en relief** (tableau de bord, saisies) : un bandeau sarcelle en haut porte le titre et les filtres, en texte blanc. Les cartes blanches le chevauchent de 48 px, sans bordure, avec une ombre douce (`--ombre-carte`), sur le fond `--fond-relief`. Les autres écrans (compte, profil, connexion, vitrine, pages légales) restent à plat, avec bordure lin.
- Boutons : **principal** (fond safran, texte blanc) et **secondaire** (fond blanc, bordure lin, texte encre). Les actions destructives prennent le style secondaire avec un texte brique. Hauteur minimale de 44 px.
- Largeur de lecture maximale : 640 px dans l'app, 1080 px sur la vitrine.

## Icônes

Un seul jeu : **Lucide** (`lucide-vue-next`), trait de 1,75, taille de 20 px (24 px dans la barre d'onglets). Aucun emoji dans l'interface.

## Mouvement

Seules animations autorisées :

- les transitions de couleur, de fond, de bordure et d'opacité au survol, au focus et au clic : 150 ms, `ease-out` ;
- l'apparition en fondu de la fenêtre de confirmation : 150 ms.

Pas d'animation au défilement, pas de parallaxe, pas de rebond, pas de curseur personnalisé. Avec le réglage système « réduire les animations », toutes les transitions sont coupées.

## Ton des textes

- **Tutoiement** partout, dans l'app comme sur la vitrine.
- Phrases de 20 mots au plus. Une idée par phrase.
- On dit ce que fait l'outil, pour qui, et ce que la personne y gagne, avec des mots concrets : cotisations, URSSAF, chiffre d'affaires, revenu net, échéance.
- On ne promet rien que l'app ne fait pas encore. Ce qui est en préparation est annoncé comme tel.
- Les montants calculés sont toujours présentés comme des estimations.
- Ponctuation : pas de tiret long (—) ni de tiret court (–) comme ponctuation. On utilise le point, la virgule ou les deux-points.
- Jamais de liste de trois adjectifs.
- Mots et formules interdits : transformer, booster, libérer, potentiel, révolutionner, révolutionnaire, tout-en-un, solution, sans effort, en quelques clics, simplifiez-vous la vie, intelligent, puissant, ultime, innovant, incroyable, magique, game changer, « grâce à l'IA ».
