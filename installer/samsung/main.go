// cleantv-samsung: installs TizenBrew, and with it TizenTube (YouTube without ads), on a
// Samsung TV from Windows, macOS or Linux. It does what TizenBrew Installer does for TVs
// up to Tizen 6: push the signed widget over sdb and install it. Tizen 7 and later need a
// Samsung certificate made with a Samsung account, which this tool does not do.
// The widget is embedded at build time (see ../build.sh).
package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed assets/tizenbrew.wgt
var tizenbrew []byte

const (
	// From the widget's config.xml (<tizen:application package=...>).
	tizenbrewPkg = "xvvl3S1bvH"
	remoteWgt    = "/home/owner/share/tmp/sdk_tools/package.wgt"
)

var in = bufio.NewReader(os.Stdin)

func main() {
	consoleUTF8()
	fmt.Println("==============================================")
	fmt.Println("  cleantv Samsung : YouTube sans pubs")
	fmt.Println("==============================================")
	fmt.Println()
	fmt.Println("Ce programme installe TizenBrew sur votre TV Samsung. TizenBrew contient")
	fmt.Println("TizenTube : YouTube sans pubs, qui saute les passages sponsorisés.")
	fmt.Println("Rien n'est supprimé de la TV. Pour l'enlever : Apps > TizenBrew > Supprimer.")

	prepareTV()
	s := connect()
	defer s.Close()
	checkVersion(s)
	install(s)
	afterInstall()
	tips()
	finish()
}

// ---------- console ----------

func ask(q, def string) string {
	if def != "" {
		fmt.Printf("%s [%s] : ", q, def)
	} else {
		fmt.Printf("%s : ", q)
	}
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		die("Entrée fermée.")
	}
	if line = strings.TrimSpace(line); line == "" {
		return def
	}
	return line
}

func step(title string) { fmt.Printf("\n--- %s ---\n", title) }

func finish() {
	fmt.Println()
	ask("Appuyez sur Entrée pour fermer", "")
	os.Exit(0)
}

func die(msg string) {
	fmt.Println()
	fmt.Println("ERREUR : " + msg)
	ask("Appuyez sur Entrée pour fermer", "")
	os.Exit(1)
}

func appDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "cleantv")
}

// localIP is the address this computer uses on the home network. Dialing UDP sends nothing.
func localIP() string {
	c, err := net.Dial("udp", "192.168.0.1:9")
	if err != nil {
		return ""
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}

// ---------- steps ----------

func prepareTV() {
	step("1. Activer le mode développeur sur la TV")
	pc := localIP()
	if pc == "" {
		pc = "(introuvable : voir ipconfig)"
	}
	fmt.Println("Le PC et la TV doivent être sur le même réseau (même box).")
	fmt.Println("Sur la TV, avec la télécommande :")
	fmt.Println("  a. Bouton Accueil, puis ouvrez « Apps » (en plein écran, pas la barre).")
	fmt.Println("  b. Tapez 1 2 3 4 5. Sans touches chiffres : bouton « 123 » de la")
	fmt.Println("     télécommande, ou la télécommande de l'appli SmartThings du téléphone.")
	fmt.Println("     Rien ne s'affiche pendant la saisie, c'est normal.")
	fmt.Println("  c. Dans la fenêtre « Developer mode » : mettez On.")
	fmt.Println("     Host PC IP : tapez l'adresse de ce PC : " + pc)
	fmt.Println("  d. OK, puis redémarrez la TV : maintenez le bouton marche de la")
	fmt.Println("     télécommande jusqu'au logo Samsung.")
	ask("Appuyez sur Entrée quand la TV a redémarré", "")
}

func connect() *sdb {
	step("2. Se connecter à la TV")
	fmt.Println("L'adresse IP de la TV se trouve dans :")
	fmt.Println("  Paramètres > Général > Réseau > État du réseau > Paramètres IP")
	saved, _ := os.ReadFile(filepath.Join(appDir(), "last-samsung-ip"))
	for {
		ip, ok := parseIP(ask("Adresse IP de la TV (ex. 192.168.1.42)", strings.TrimSpace(string(saved))))
		if !ok {
			fmt.Println("Ce n'est pas une adresse IP valide. Elle ressemble à 192.168.1.42.")
			continue
		}
		fmt.Println("Connexion à " + ip + "...")
		s, err := dialSDB(ip)
		if err == nil {
			_ = os.MkdirAll(appDir(), 0o755)
			_ = os.WriteFile(filepath.Join(appDir(), "last-samsung-ip"), []byte(ip), 0o644)
			fmt.Println("Connecté.")
			return s
		}
		fmt.Println()
		fmt.Println("Pas de connexion (" + err.Error() + "). Vérifiez :")
		fmt.Println("  - que la TV est allumée et sur le même réseau ;")
		fmt.Println("  - que le mode développeur est sur On, avec l'IP de ce PC ;")
		fmt.Println("  - que la TV a bien été redémarrée depuis (bouton marche maintenu) ;")
		fmt.Println("  - l'adresse IP de la TV ;")
		fmt.Println("  - que le pare-feu Windows laisse passer ce programme.")
	}
}

func parseIP(s string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() == nil {
		return "", false
	}
	return ip.String(), true
}

// tizenMajor turns "6.0" into 6; 0 when unknown.
func tizenMajor(v string) int {
	n, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	return n
}

func checkVersion(s *sdb) {
	v, err := s.platformVersion()
	switch major := tizenMajor(v); {
	case err != nil || major == 0:
		fmt.Println("Version de Tizen inconnue, on essaie quand même.")
	case major < 3:
		die("Tizen " + v + " : trop ancien (TV d'avant 2017). TizenBrew demande Tizen 3 ou plus.")
	case major >= 7:
		fmt.Println("Tizen " + v + " (TV de 2023 ou plus récente).")
		die("Sur Tizen 7 et plus, il faut un certificat Samsung lié à un compte Samsung.\n" +
			"Ce programme ne le fait pas. Utilisez TizenBrew Installer :\n" +
			"  https://github.com/reisxd/TizenBrewInstaller/releases/latest")
	default:
		fmt.Println("Tizen " + v + " : compatible.")
	}
}

func install(s *sdb) {
	step("3. Installer TizenBrew")
	fmt.Printf("Envoi de TizenBrew à la TV (%d Ko)...\n", len(tizenbrew)/1024)
	s.deadline(5 * time.Minute)
	if err := s.push(remoteWgt, tizenbrew); err != nil {
		die("Envoi impossible : " + err.Error())
	}
	fmt.Println("Installation sur la TV (jusqu'à une minute)...")
	out, err := s.shell("0 vd_appinstall "+tizenbrewPkg+" "+remoteWgt, func(o string) bool {
		return strings.Contains(o, "spend time")
	})
	ok, msg := installResult(out)
	if err != nil && !ok {
		die("Installation interrompue : " + err.Error() + "\n" + out)
	}
	if !ok {
		fmt.Println(strings.TrimSpace(out))
		die(msg)
	}
	fmt.Println("TizenBrew est installé.")
}

// installResult reads what wascmd printed. Error 118 is Tizen's generic install failure:
// most often a full TV or a half-installed earlier copy.
func installResult(out string) (bool, string) {
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "install failed") || strings.Contains(low, "failed["):
		return false, "La TV a refusé l'installation.\n" +
			"  - Débranchez la TV du secteur 15 secondes, rebranchez, relancez ce programme.\n" +
			"  - Libérez de la place : Paramètres > Assistance > Entretien de l'appareil >\n" +
			"    Gérer le stockage.\n" +
			"  - Si TizenBrew est déjà dans Apps, supprimez-le puis relancez."
	case strings.Contains(low, "install completed") || strings.Contains(low, "spend time"):
		return true, ""
	default:
		return false, "Réponse inattendue de la TV : regardez dans Apps si TizenBrew est présent."
	}
}

func afterInstall() {
	step("4. Dernier réglage sur la TV")
	fmt.Println("TizenBrew a besoin du mode développeur, mais plus de ce PC :")
	fmt.Println("  a. Apps, puis 1 2 3 4 5 comme tout à l'heure.")
	fmt.Println("  b. Laissez Developer mode sur On, remplacez Host PC IP par 127.0.0.1, OK.")
	fmt.Println("  c. Redémarrez la TV (bouton marche maintenu jusqu'au logo).")
	fmt.Println("  d. Ouvrez TizenBrew dans Apps, puis choisissez TizenTube.")
	fmt.Println("Astuce : dans Apps, sur TizenBrew, maintenez OK > « Ajouter à l'accueil »")
	fmt.Println("et placez-le en premier.")
}

func tips() {
	step("Pour aller plus loin (facultatif, à faire à la main)")
	fmt.Println("Samsung ne permet pas d'enlever ses applis ni son accueil (pas d'accès comme")
	fmt.Println("sur Fire TV). Deux réglages réduisent quand même pubs et pistage :")
	fmt.Println("  - Démarrer sur la dernière source au lieu de l'accueil Smart Hub :")
	fmt.Println("    Paramètres > Général > Fonctions Smart > Exécution auto Smart Hub : Off.")
	fmt.Println("  - Bloquer les pubs de l'accueil : Paramètres > Général > Réseau > État du")
	fmt.Println("    réseau > Paramètres IP > DNS : saisie manuelle 94.140.14.14 (AdGuard DNS).")
	fmt.Println("Les noms des menus varient un peu selon l'année de la TV.")
}
