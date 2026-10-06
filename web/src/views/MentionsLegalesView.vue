<script setup>
// Les champs [À COMPLÉTER] sont à remplir par l'éditeur du site avant la mise en ligne.
const ac = 'À COMPLÉTER'
</script>

<template>
  <article class="texte-legal largeur-lecture">
    <h1>Mentions légales</h1>
    <p class="aide">Dernière mise à jour : <span class="a-completer">[{{ ac }} : date]</span></p>

    <h2>Éditeur du site</h2>
    <dl>
      <dt>Nom et prénom, ou raison sociale</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>Statut juridique</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>SIRET</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>Adresse</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>E-mail de contact</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>Directeur ou directrice de la publication</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
    </dl>

    <h2>Hébergeur</h2>
    <dl>
      <dt>Nom ou raison sociale</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>Adresse</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
      <dt>Téléphone</dt>
      <dd><span class="a-completer">[{{ ac }}]</span></dd>
    </dl>

    <h2>Propriété intellectuelle</h2>
    <p>Les textes, le logo et l'interface de Microdash appartiennent à l'éditeur. Tu ne peux pas les reproduire sans son accord écrit.</p>
    <p>Les polices Bricolage Grotesque et IBM Plex Sans sont publiées sous licence SIL Open Font License. Les icônes Lucide sont publiées sous licence ISC.</p>

    <h2>Estimations</h2>
    <p>Les montants affichés par Microdash sont des estimations calculées à partir de tes saisies et des taux publiés. Ils ne remplacent pas ta déclaration ni les montants calculés par l'URSSAF ou les impôts.</p>

    <h2>Données personnelles</h2>
    <p>Tout est expliqué sur la page <RouterLink to="/confidentialite">Confidentialité</RouterLink>.</p>
  </article>
</template>
