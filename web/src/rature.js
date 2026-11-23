// Rature d'une tâche cochée : un trait barre le titre, puis la ligne se replie et quitte la liste.
// Un bandeau « Annuler » reste quelques secondes après.
import { reactive, ref, onBeforeUnmount } from 'vue'

export const DUREE_TRAIT = 300
export const DUREE_SORTIE = 250
const DUREE_ANNULER = 5000

const reduit = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches

// basculer(t) inverse t.faite côté API et renvoie true si l'API l'a accepté.
export function useRature(basculer) {
  const raturees = reactive(new Set())
  const annulable = ref(null)
  const minuteurs = new Map()
  let minuteurAnnuler

  // Une tâche faite reste affichée le temps de son trait.
  const affichee = (t) => !t.faite || raturees.has(t.id)

  function arreter(id) {
    clearTimeout(minuteurs.get(id))
    minuteurs.delete(id)
    raturees.delete(id)
  }

  async function cocher(t) {
    // Reclic pendant le trait : il s'arrête et la tâche revient.
    if (raturees.has(t.id)) {
      arreter(t.id)
      return basculer(t)
    }
    if (t.faite) return basculer(t)
    raturees.add(t.id)
    minuteurs.set(t.id, setTimeout(() => {
      arreter(t.id)
      proposerAnnuler(t)
    }, reduit() ? 0 : DUREE_TRAIT))
    if (!(await basculer(t)) && minuteurs.has(t.id)) arreter(t.id)
  }

  function proposerAnnuler(t) {
    clearTimeout(minuteurAnnuler)
    annulable.value = t
    minuteurAnnuler = setTimeout(() => (annulable.value = null), DUREE_ANNULER)
  }

  function annuler() {
    const t = annulable.value
    clearTimeout(minuteurAnnuler)
    annulable.value = null
    if (t) basculer({ ...t, faite: true })
  }

  onBeforeUnmount(() => {
    minuteurs.forEach(clearTimeout)
    clearTimeout(minuteurAnnuler)
  })

  return { raturees, affichee, cocher, annulable, annuler }
}

// Sortie d'une ligne raturée dans un <TransitionGroup :css="false" @leave="sortie">.
// Les autres lignes (filtre changé, suppression) partent sans animation.
export function sortie(el, fin) {
  if (reduit() || !el.classList.contains('raturee')) return fin()
  el.style.height = `${el.offsetHeight}px`
  el.style.overflow = 'hidden'
  void el.offsetHeight
  el.style.transition = `height ${DUREE_SORTIE}ms ease-out, min-height ${DUREE_SORTIE}ms ease-out, opacity ${DUREE_SORTIE}ms ease-out`
  el.style.height = '0'
  el.style.minHeight = '0'
  el.style.opacity = '0'
  setTimeout(fin, DUREE_SORTIE)
}
