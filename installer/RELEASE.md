# cleantv : une Fire TV propre, sans pubs

**cleantv** allège votre Fire TV ou votre Fire TV Stick et remplace l'accueil Amazon
par un accueil simple et rapide : vos apps, vos entrées HDMI, rien d'autre.

## Ce que fait l'installateur

- Désactive les services Amazon de pubs, de pistage et de télémétrie (rien n'est supprimé).
- Garde Alexa et le bouton vocal, sauf si vous choisissez de les couper.
- Installe l'accueil cleantv et lui confie le bouton Home.
- Propose d'installer SmartTube (YouTube sans pubs).
- Tout est réversible : relancez-le et choisissez « Tout restaurer ».

## Installation (Windows)

1. Téléchargez **cleantv-installer.exe** ci-dessous.
2. Double-cliquez dessus. Windows affiche un avertissement SmartScreen car le fichier
   n'est pas signé : cliquez sur « Informations complémentaires », puis
   « Exécuter quand même ».
3. Suivez l'assistant. Il vous explique :
   - comment activer le **débogage ADB** sur la TV
     (Paramètres > Ma Fire TV > À propos : appuyez 7 fois sur le nom de l'appareil,
     puis Options pour les développeurs > Débogage ADB) ;
   - où trouver l'**adresse IP** de la TV (Paramètres > Ma Fire TV > À propos > Réseau),
     une adresse du type 192.168.x.x ;
   - d'accepter le message « Autoriser le débogage » sur la TV
     (cochez « Toujours autoriser »).

Le PC et la TV doivent être sur le même réseau. Les outils ADB de Google sont
téléchargés automatiquement si besoin.

macOS et Linux : utilisez `cleantv-installer-macos` ou `cleantv-installer-linux`
(rendez-le exécutable avec `chmod +x`).

## Compatibilité

Fire OS 6, 7 et 8 (Fire TV Stick, Fire TV Cube, téléviseurs Fire TV Edition).
Testé sur un Xiaomi TV F Pro 50 sous Fire OS 8.

## Bon à savoir

- Une mise à jour de Fire OS peut réactiver des services : relancez l'installateur.
- Une réinitialisation d'usine efface toutes les modifications.
- Basé sur [dumbtv](https://github.com/humblemuzzu/dumbtv) (licence MIT).
- À utiliser à vos risques. Projet non affilié à Amazon.
