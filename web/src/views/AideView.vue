<script setup>
// Guide utilisateur : le texte vient de l'API (api/guide/GUIDE.md), le même que lit Claude.
import { computed, onMounted, ref } from 'vue'

const texte = ref('')
const erreur = ref('')

onMounted(async () => {
  try {
    const r = await fetch('/api/guide', { credentials: 'same-origin' })
    if (!r.ok) throw new Error(`Erreur ${r.status}`)
    texte.value = await r.text()
  } catch (e) {
    erreur.value = e.message === 'Failed to fetch' ? 'Pas de connexion. Réessaie quand le réseau revient.' : e.message
  }
})

// Markdown réduit : # titre, ## sections, listes « - », paragraphes, **gras** et `code`.
// Le texte est échappé avant : aucun HTML du fichier n'est interprété.
const echapper = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
const enLigne = (s) => echapper(s).replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>').replace(/`(.+?)`/g, '<code>$1</code>')

const guide = computed(() => {
  const res = { titre: '', intro: [], sections: [] }
  let section = null
  for (const ligne of texte.value.split('\n')) {
    const l = ligne.trim()
    if (!l) continue
    if (l.startsWith('## ')) {
      section = { titre: l.slice(3), id: l.slice(3).toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''), blocs: [] }
      res.sections.push(section)
    } else if (l.startsWith('# ')) {
      res.titre = l.slice(2)
    } else {
      const blocs = section ? section.blocs : res.intro
      const point = l.startsWith('- ')
      const dernier = blocs[blocs.length - 1]
      if (point && dernier?.liste) dernier.liste.push(enLigne(l.slice(2)))
      else if (point) blocs.push({ liste: [enLigne(l.slice(2))] })
      else blocs.push({ html: enLigne(l) })
    }
  }
  return res
})
</script>

<template>
  <h1>Aide</h1>
  <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
  <template v-else-if="texte">
    <section class="carte pile">
      <p v-for="(b, i) in guide.intro" :key="i" v-html="b.html" />
      <nav aria-label="Sommaire" class="sommaire">
        <a v-for="s in guide.sections" :key="s.id" :href="`#${s.id}`">{{ s.titre }}</a>
      </nav>
    </section>
    <section v-for="s in guide.sections" :id="s.id" :key="s.id" class="carte" :aria-labelledby="`titre-${s.id}`">
      <h2 :id="`titre-${s.id}`">{{ s.titre }}</h2>
      <template v-for="(b, i) in s.blocs" :key="i">
        <ul v-if="b.liste"><li v-for="(p, j) in b.liste" :key="j" v-html="p" /></ul>
        <p v-else class="aide" v-html="b.html" />
      </template>
    </section>
  </template>
  <p v-else class="aide">Chargement…</p>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
h2 { margin-bottom: var(--e2); }
.carte { scroll-margin-top: var(--e4); }
.sommaire { display: flex; flex-wrap: wrap; gap: var(--e2) var(--e4); }
.sommaire a { color: var(--safran); font-weight: 500; }
ul { margin: var(--e2) 0 0; padding-left: var(--e5); display: grid; gap: var(--e2); }
:deep(code) { font-size: 14px; background: var(--papier); border: 1px solid var(--lin); border-radius: var(--rayon); padding: 0 4px; overflow-wrap: anywhere; }
</style>
