<script setup>
// Les champs [À COMPLÉTER] sont à remplir par l'éditeur du site avant la mise en ligne.
const ac = 'À COMPLÉTER'
</script>

<template>
  <article class="texte-legal largeur-lecture">
    <h1>Confidentialité</h1>
    <p class="aide">Dernière mise à jour : <span class="a-completer">[{{ ac }} : date]</span></p>
    <p>Cette page dit quelles données Microdash garde sur toi, pourquoi, combien de temps, et comment les effacer.</p>

    <h2>Qui est responsable de tes données</h2>
    <p>L'éditeur du site : <span class="a-completer">[{{ ac }} : nom ou raison sociale]</span>. Pour toute question : <span class="a-completer">[{{ ac }} : e-mail de contact]</span>.</p>

    <h2>Ce que nous gardons</h2>
    <ul>
      <li><strong>Ton adresse e-mail</strong>, pour te connecter.</li>
      <li><strong>Ton mot de passe</strong>, jamais en clair : seule une empreinte chiffrée (argon2id) est enregistrée.</li>
      <li><strong>Ton profil d'activité</strong> : catégorie, nature de l'activité, date de début, ACRE, versement libératoire, périodicité de déclaration.</li>
      <li><strong>Tes saisies</strong> : recettes et dépenses, avec leur date, leur montant, leur catégorie, leur libellé et, si tu le notes, le nom du client ou du fournisseur.</li>
      <li><strong>Ton agenda</strong> : les échéances que tu as cochées comme faites, et la date où tu les as cochées.</li>
      <li><strong>Tes rappels par e-mail</strong> : ceux que tu as activés ou coupés, et la liste des rappels déjà envoyés, pour ne jamais t'envoyer deux fois le même.</li>
      <li><strong>Ta trésorerie</strong> : tes dépenses récurrentes et, si tu le notes, le solde de ton compte et sa date.</li>
      <li><strong>Ta connexion Stripe</strong>, si tu l'actives : ta clé restreinte, chiffrée (AES-256-GCM) et jamais réaffichée, l'identifiant et le nom de ton compte Stripe, et les chiffres de la dernière lecture (revenu mensuel récurrent, nombre d'abonnements, encaissements et frais par mois). Aucune donnée sur tes clients n'est gardée.</li>
    </ul>
    <p>Nous ne demandons ni ton nom, ni ton numéro de téléphone, ni tes coordonnées bancaires.</p>

    <h2>Pourquoi</h2>
    <p>Uniquement pour faire fonctionner ton compte et calculer tes estimations. C'est le service que tu demandes en créant ton compte : c'est la base légale du traitement (article 6.1.b du RGPD).</p>
    <p>Tes données ne sont ni vendues, ni utilisées pour de la publicité, ni partagées avec l'URSSAF ou les impôts.</p>

    <h2>Combien de temps</h2>
    <ul>
      <li>Ton compte, ton profil, tes saisies, ton agenda, ta trésorerie et tes réglages de rappels : jusqu'à ce que tu supprimes ton compte.</li>
      <li>Ta connexion Stripe : jusqu'à ce que tu la déconnectes ou que tu supprimes ton compte.</li>
      <li>Ta session de connexion : 30 jours au plus, ou jusqu'à ce que tu te déconnectes.</li>
      <li>Les journaux techniques du serveur (adresse IP, page demandée) : <span class="a-completer">[{{ ac }} : durée]</span>.</li>
    </ul>

    <h2>Qui y a accès</h2>
    <ul>
      <li>L'éditeur du site, pour faire fonctionner le service.</li>
      <li>L'hébergeur du serveur : <span class="a-completer">[{{ ac }} : nom et pays]</span>.</li>
      <li>Resend, le service qui envoie les rappels par e-mail. Il reçoit ton adresse e-mail et le contenu du rappel : l'échéance, sa date et les montants estimés. Resend est une société américaine : <span class="a-completer">[{{ ac }} : garanties du transfert hors de l'Union européenne, voir l'accord de traitement des données de Resend]</span>.</li>
    </ul>
    <p>Les rappels sont activés à la création du compte. Tu les coupes un par un dans l'écran Compte, ou tous d'un coup avec le lien en bas de chaque e-mail.</p>

    <h2>Cookies et stockage sur ton appareil</h2>
    <p>Microdash utilise un seul cookie, nommé « session ». Il te garde connecté et il est indispensable au service. Il n'y a aucun cookie de mesure d'audience ni de publicité : c'est pour cela qu'aucun bandeau ne te demande ton accord.</p>
    <p>Pour fonctionner sans réseau, l'app garde sur ton appareil une copie des pages et de tes dernières données. Cette copie est effacée quand tu te déconnectes ou supprimes ton compte.</p>
    <p>Les polices sont hébergées sur notre serveur : ta visite n'est pas transmise à Google ou à un autre fournisseur.</p>

    <h2>Tes droits</h2>
    <p>Tu peux consulter, corriger ou effacer tes données, en limiter l'usage, t'y opposer ou les récupérer. Tu supprimes tout toi-même depuis l'écran Compte. Pour le reste, écris à <span class="a-completer">[{{ ac }} : e-mail de contact]</span>.</p>
    <p>Si tu estimes que tes droits ne sont pas respectés, tu peux saisir la CNIL sur <a href="https://www.cnil.fr/fr/plaintes" rel="noopener">cnil.fr</a>.</p>
  </article>
</template>
