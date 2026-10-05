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
    </ul>
    <p>Nous ne demandons ni ton nom, ni ton numéro de téléphone, ni tes coordonnées bancaires.</p>

    <h2>Pourquoi</h2>
    <p>Uniquement pour faire fonctionner ton compte et calculer tes estimations. C'est le service que tu demandes en créant ton compte : c'est la base légale du traitement (article 6.1.b du RGPD).</p>
    <p>Tes données ne sont ni vendues, ni utilisées pour de la publicité, ni partagées avec l'URSSAF ou les impôts.</p>

    <h2>Combien de temps</h2>
    <ul>
      <li>Ton compte, ton profil et tes saisies : jusqu'à ce que tu supprimes ton compte.</li>
      <li>Ta session de connexion : 30 jours au plus, ou jusqu'à ce que tu te déconnectes.</li>
      <li>Les journaux techniques du serveur (adresse IP, page demandée) : <span class="a-completer">[{{ ac }} : durée]</span>.</li>
    </ul>

    <h2>Qui y a accès</h2>
    <ul>
      <li>L'éditeur du site, pour faire fonctionner le service.</li>
      <li>L'hébergeur du serveur : <span class="a-completer">[{{ ac }} : nom et pays]</span>.</li>
    </ul>
    <p>Microdash n'envoie pas encore d'e-mail. Quand les rappels d'échéances arriveront, ils passeront par le service Resend. Cette page sera mise à jour avant.</p>

    <h2>Cookies et stockage sur ton appareil</h2>
    <p>Microdash utilise un seul cookie, nommé « session ». Il te garde connecté et il est indispensable au service. Il n'y a aucun cookie de mesure d'audience ni de publicité : c'est pour cela qu'aucun bandeau ne te demande ton accord.</p>
    <p>Pour fonctionner sans réseau, l'app garde sur ton appareil une copie des pages et de tes dernières données. Cette copie est effacée quand tu te déconnectes ou supprimes ton compte.</p>
    <p>Les polices sont hébergées sur notre serveur : ta visite n'est pas transmise à Google ou à un autre fournisseur.</p>

    <h2>Tes droits</h2>
    <p>Tu peux consulter, corriger ou effacer tes données, en limiter l'usage, t'y opposer ou les récupérer. Tu supprimes tout toi-même depuis l'écran Compte. Pour le reste, écris à <span class="a-completer">[{{ ac }} : e-mail de contact]</span>.</p>
    <p>Si tu estimes que tes droits ne sont pas respectés, tu peux saisir la CNIL sur <a href="https://www.cnil.fr/fr/plaintes" rel="noopener">cnil.fr</a>.</p>
  </article>
</template>
