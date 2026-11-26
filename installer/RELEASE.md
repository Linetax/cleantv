# cleantv : une Fire TV propre, sans pubs

**cleantv** allège votre Fire TV ou votre Fire TV Stick et remplace l'accueil Amazon
par un accueil simple et rapide : vos apps, vos entrées HDMI, rien d'autre.

## Ce que fait l'installateur

- Désactive les services Amazon de pubs, de pistage et de télémétrie (rien n'est supprimé).
- Garde Alexa et le bouton vocal, sauf si vous choisissez de les couper.
- Installe l'accueil cleantv et lui confie le bouton Home.
- Installe SmartTube (YouTube sans pubs) et le place en premier sur l'accueil.
- Installe Cineflux (si vous dites oui), juste après SmartTube.
- Tout est réversible : relancez-le et choisissez « Tout restaurer ».

## Installation (Windows)

1. Téléchargez **cleantv-installer.exe** ci-dessous.
2. Double-cliquez dessus. Windows affiche un avertissement SmartScreen car le fichier
   n'est pas signé : cliquez sur « Informations complémentaires », puis
   « Exécuter quand même ».
3. Une fenêtre s'ouvre (elle utilise Microsoft Edge, déjà présent sur Windows).
   Suivez l'assistant, étape par étape, avec de gros boutons. Il vous explique :
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

Fire OS 6, 7 et 8 (Fire TV Stick, Fire TV Stick 4K et 4K Max, Fire TV Cube, téléviseurs
Fire TV Edition). Testé sur un Xiaomi TV F Pro 50 sous Fire OS 8.

## TV Samsung : cleantv-samsung.exe

Installe **TizenBrew**, qui contient **TizenTube** (YouTube sans pubs, saute les
passages sponsorisés). Pour les Samsung de 2017 à 2022 (Tizen 3 à 6). Les modèles
2023 et plus récents (Tizen 7+) demandent un compte Samsung : utilisez
[TizenBrew Installer](https://github.com/reisxd/TizenBrewInstaller/releases/latest).

1. Téléchargez **cleantv-samsung.exe** ci-dessous et lancez-le (même avertissement
   SmartScreen que plus haut).
2. Il explique comment activer le mode développeur (Apps, puis 12345) avec l'adresse
   du PC, se connecte à la TV et installe TizenBrew, avec Cineflux si vous le voulez.
3. À la fin, remettez l'adresse du mode développeur sur 127.0.0.1, redémarrez la TV et
   ouvrez TizenBrew, puis TizenTube ou Cineflux.

Samsung ne permet pas d'enlever ses applis ni de changer l'accueil : il n'y a pas
d'allègement comme sur Fire TV. L'assistant indique deux réglages manuels qui aident
(démarrage sans Smart Hub, DNS bloqueur de pubs).

## Bon à savoir

- Une mise à jour de Fire OS peut réactiver des services : relancez l'installateur.
- Une réinitialisation d'usine efface toutes les modifications.
- Basé sur [dumbtv](https://github.com/humblemuzzu/dumbtv) (licence MIT).
- TizenBrew et TizenTube : projets de [reisxd](https://github.com/reisxd/TizenBrew) (GPL-3.0), intégrés sans modification.
- À utiliser à vos risques. Projet non affilié à Amazon ni à Samsung.
