# Cineflux pour TizenBrew

Module TizenBrew de type « site » : il ouvre https://cineflux.linetax.party/ sur une TV Samsung
et branche le bouton Retour de la télécommande sur `window.cinefluxBack()`, comme l'app Android.

TizenBrew charge les modules par jsDelivr (`npm/<paquet>` ou `gh/<compte>/<dépôt>`),
donc ces deux fichiers doivent être dans un dépôt GitHub public ou sur npm.
Sur la TV : TizenBrew › Gestionnaire de modules › Ajouter un Module GitHub › `<compte>/<dépôt>`.
