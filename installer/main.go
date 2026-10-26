// cleantv-installer: one file that sets up cleantv on a Fire TV or Fire TV Stick from
// Windows, macOS or Linux. It ports what scripts/debloat.sh, launcher/install.sh and
// scripts/restore.sh do, minus the DNS filter, so nobody needs bash or the Android SDK.
// The package lists and the APK are embedded at build time (see build.sh).
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed assets
var assets embed.FS

const (
	pkgHome   = "tv.dumbtv.home"
	guard     = pkgHome + "/" + pkgHome + ".Guard"
	oldGuard  = "com.spocky.projengmenu/com.spocky.projengmenu.services.ProjectivyAccessibilityService"
	guardBind = "Service[label=dumbtv home button,"
	guardRaw  = "Service[label=tv.dumbtv.home.Guard,"

	// Pinned build: releases are immutable, the hash identifies it. Universal runs on 32- and 64-bit Fire OS.
	smartTubePkg = "org.smarttube.stable"
	smartTubeURL = "https://github.com/yuliskov/SmartTube/releases/download/32.68s/SmartTube_stable_32.68_universal.apk"
	smartTubeSHA = "45a36e62110bcb866527ba8e9ab876f4122e1143647bbd0a97362d13803f6b94"
)

var (
	in      = bufio.NewReader(os.Stdin)
	adbPath string
	serial  string
	sdk     int
)

func main() {
	consoleUTF8()
	fmt.Println("==============================================")
	fmt.Println("  cleantv : une Fire TV propre, sans pubs")
	fmt.Println("==============================================")
	fmt.Println()
	fmt.Println("Ce programme allège votre Fire TV et remplace son accueil.")
	fmt.Println("Tout est réversible : relancez-le et choisissez « Tout restaurer ».")

	var err error
	if adbPath, err = findAdb(); err != nil {
		die("Impossible de préparer les outils ADB : " + err.Error())
	}
	prepareTV()
	connect()
	detect()

	for {
		fmt.Println()
		fmt.Println("Que voulez-vous faire ?")
		fmt.Println("  1. Installer cleantv (accueil + allègement)")
		fmt.Println("  2. Tout restaurer comme avant")
		fmt.Println("  3. Quitter")
		switch ask("Votre choix", "1") {
		case "1":
			install()
			finish()
		case "2":
			restore()
			finish()
		case "3":
			finish()
		}
	}
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
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func yes(q string, def bool) bool {
	d := "o/N"
	if def {
		d = "O/n"
	}
	for {
		switch strings.ToLower(ask(q+" ("+d+")", "")) {
		case "":
			return def
		case "o", "oui", "y", "yes":
			return true
		case "n", "non", "no":
			return false
		}
	}
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

// ---------- adb ----------

func appDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "cleantv")
}

// findAdb uses an adb already on the PATH, else Google's platform-tools, downloaded once.
func findAdb() (string, error) {
	if p, err := exec.LookPath("adb"); err == nil {
		return p, nil
	}
	exe := "adb"
	if runtime.GOOS == "windows" {
		exe = "adb.exe"
	}
	dir := filepath.Join(appDir(), "platform-tools")
	p := filepath.Join(dir, exe)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	osName := map[string]string{"windows": "windows", "darwin": "darwin", "linux": "linux"}[runtime.GOOS]
	if osName == "" {
		return "", fmt.Errorf("système %s non pris en charge", runtime.GOOS)
	}
	url := "https://dl.google.com/android/repository/platform-tools-latest-" + osName + ".zip"
	fmt.Println()
	fmt.Println("Téléchargement des outils ADB officiels de Google (une seule fois)...")
	fmt.Println("  " + url)
	data, err := download(url)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "platform-tools/")
		// adb and the two DLLs it needs on Windows; nothing else is used.
		if f.FileInfo().IsDir() || name != exe && !strings.HasPrefix(name, "AdbWin") {
			continue
		}
		if err := extract(f, filepath.Join(dir, name)); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("adb introuvable dans l'archive")
	}
	return p, nil
}

func extract(f *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func download(url string) ([]byte, error) {
	var last error
	for i := 0; i < 3; i++ {
		resp, err := http.Get(url)
		if err == nil {
			data, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 && rerr == nil {
				return data, nil
			}
			err = fmt.Errorf("HTTP %d %v", resp.StatusCode, rerr)
		}
		last = err
		time.Sleep(3 * time.Second)
	}
	return nil, last
}

func adb(args ...string) string {
	if serial != "" && args[0] != "connect" && args[0] != "disconnect" {
		args = append([]string{"-s", serial}, args...)
	}
	out, _ := exec.Command(adbPath, args...).CombinedOutput()
	return strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))
}

func sh(cmd string) string { return adb("shell", cmd) }

// ---------- connection ----------

func prepareTV() {
	step("1. Préparer la TV")
	fmt.Println("Sur la TV, avec la télécommande :")
	fmt.Println("  a. Paramètres > Ma Fire TV > À propos")
	fmt.Println("  b. Sélectionnez le nom de l'appareil (première ligne) et appuyez")
	fmt.Println("     7 fois sur OK, jusqu'au message « Vous êtes un développeur ».")
	fmt.Println("  c. Revenez en arrière : Paramètres > Ma Fire TV > Options pour les")
	fmt.Println("     développeurs > Débogage ADB : Activé.")
	fmt.Println("  d. Le PC et la TV doivent être sur le même réseau (même box).")
	ask("Appuyez sur Entrée quand c'est fait", "")
}

func connect() {
	step("2. Se connecter à la TV")
	fmt.Println("L'adresse IP de la TV se trouve dans :")
	fmt.Println("  Paramètres > Ma Fire TV > À propos > Réseau (ligne « Adresse IP »)")
	saved, _ := os.ReadFile(filepath.Join(appDir(), "last-ip"))
	for {
		target, ok := parseTarget(ask("Adresse IP de la TV (ex. 192.168.1.42)", strings.TrimSpace(string(saved))))
		if !ok {
			fmt.Println("Ce n'est pas une adresse IP valide. Elle ressemble à 192.168.1.42.")
			continue
		}
		fmt.Println("Connexion à " + target + "...")
		fmt.Println("Si la TV affiche « Autoriser le débogage USB ? », cochez « Toujours autoriser »")
		fmt.Println("depuis cet ordinateur, puis choisissez OK.")
		if waitDevice(target) {
			serial = target
			_ = os.MkdirAll(appDir(), 0o755)
			_ = os.WriteFile(filepath.Join(appDir(), "last-ip"), []byte(strings.TrimSuffix(target, ":5555")), 0o644)
			fmt.Println("Connecté.")
			return
		}
		fmt.Println()
		fmt.Println("Pas de connexion. Vérifiez :")
		fmt.Println("  - que la TV est allumée et sur le même réseau ;")
		fmt.Println("  - que « Débogage ADB » est bien activé ;")
		fmt.Println("  - l'adresse IP ;")
		fmt.Println("  - que vous avez accepté le message sur la TV.")
	}
}

// parseTarget accepts "192.168.1.42" or "192.168.1.42:5555" and returns "ip:port".
func parseTarget(s string) (string, bool) {
	host, port := s, "5555"
	if h, p, err := net.SplitHostPort(s); err == nil {
		host, port = h, p
	}
	if ip := net.ParseIP(host); ip == nil || ip.To4() == nil {
		return "", false
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", false
	}
	return host + ":" + port, true
}

func waitDevice(target string) bool {
	warned := false
	for i := 0; i < 60; i++ {
		if i%5 == 0 {
			adb("connect", target)
		}
		state := adb("-s", target, "get-state")
		switch {
		case state == "device":
			return true
		case strings.Contains(state, "unauthorized") && !warned:
			fmt.Println(">> Acceptez le message affiché sur la TV.")
			warned = true
		}
		time.Sleep(time.Second)
	}
	return false
}

func detect() {
	sdk, _ = strconv.Atoi(sh("getprop ro.build.version.sdk"))
	model := sh("getprop ro.product.model")
	maker := sh("getprop ro.product.manufacturer")
	fmt.Printf("Appareil : %s %s, %s (Android sdk %d)\n", maker, model, fireOSName(), sdk)
	if !strings.HasPrefix(sh("pm path com.amazon.tv.launcher"), "package:") {
		die("Cet appareil n'est pas une Fire TV (Fire OS). cleantv ne fonctionne que sur Fire OS.")
	}
	if sdk < 25 {
		die("Fire OS 5 et plus ancien refusent la désactivation des applications : rien ne peut fonctionner ici.")
	}
}

func fireOSName() string {
	switch {
	case sdk > 30:
		return "Fire OS plus récent que 8 (non testé)"
	case sdk >= 30:
		return "Fire OS 8"
	case sdk >= 28:
		return "Fire OS 7"
	case sdk >= 25:
		return "Fire OS 6"
	}
	return "Fire OS ancien"
}

// ---------- embedded lists ----------

// readList returns the first field of every non-comment line; Fire OS 6 has its own copy of some lists.
func readList(name string) []string {
	var out []string
	for _, l := range listLines(name) {
		out = append(out, strings.Fields(l)[0])
	}
	return out
}

func listLines(name string) []string {
	data, err := assets.ReadFile("assets/fireos6/" + name)
	if sdk >= 28 || err != nil {
		data, err = assets.ReadFile("assets/fireos/" + name)
	}
	if err != nil {
		die("liste intégrée manquante : " + name)
	}
	return parseList(string(data))
}

func parseList(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "#"); i >= 0 {
			l = l[:i]
		}
		if l = strings.TrimSpace(strings.Replace(l, " = ", " ", 1)); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func set(lines string, prefix string) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(lines, "\n") {
		if strings.HasPrefix(l, prefix) {
			m[strings.TrimSpace(strings.TrimPrefix(l, prefix))] = true
		}
	}
	return m
}

func applySettings(file string) {
	for _, l := range listLines(file) {
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		sh("settings put " + f[0] + " " + f[1] + " " + f[2])
		fmt.Printf("  %-46s %s\n", f[1], f[2])
	}
}

// ---------- install ----------

func install() {
	keepAlexa := yes("Garder Alexa et le bouton vocal de la télécommande ?", true)
	smartTube := yes("Installer SmartTube (YouTube sans pubs) ?", true)

	step("Allègement : désactivation des services Amazon")
	never := map[string]bool{}
	for _, p := range readList("never-disable.txt") {
		never[p] = true
	}
	pkgs := readList("packages.txt")
	if !keepAlexa {
		pkgs = append(pkgs, readList("alexa.txt")...)
	}
	present := set(sh("pm list packages"), "package:")
	off := set(sh("pm list packages -d"), "package:")
	var done, already, protected, other int
	for _, p := range pkgs {
		if never[p] || !present[p] {
			continue
		}
		if off[p] {
			already++
			continue
		}
		out := sh("pm disable-user --user 0 " + p)
		switch {
		case strings.Contains(out, "disabled-user"):
			fmt.Printf("  %-46s désactivé\n", p)
			done++
		case strings.Contains(out, "protected"):
			fmt.Printf("  %-46s protégé par Amazon, laissé\n", p)
			protected++
		default:
			fmt.Printf("  %-46s non modifié : %s\n", p, firstLine(out))
			other++
		}
	}
	fmt.Printf("Résultat : %d désactivés, %d déjà désactivés, %d protégés, %d non modifiés.\n", done, already, protected, other)

	step("Réglages de confidentialité")
	applySettings("settings.txt")
	if !keepAlexa {
		applySettings("settings.alexa.txt")
	}

	step("Accueil cleantv")
	installHome()

	if smartTube {
		step("SmartTube")
		installSmartTube()
	}

	step("Terminé")
	fmt.Println("Votre Fire TV est allégée et utilise l'accueil cleantv.")
	fmt.Println("Spotify et vos autres apps s'installent depuis l'Appstore, comme d'habitude.")
	if yes("Redémarrer la TV maintenant ? (conseillé)", true) {
		adb("reboot")
		fmt.Println("La TV redémarre.")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func installHome() {
	apk, err := assets.ReadFile("assets/dumbtv-home.apk")
	if err != nil {
		die("APK de l'accueil absent de l'installateur.")
	}
	out := installAPK(apk, "cleantv-home.apk")
	if strings.Contains(out, "INSTALL_FAILED_UPDATE_INCOMPATIBLE") {
		fmt.Println("Un ancien accueil dumbtv, signé autrement, est déjà installé.")
		fmt.Println("Le remplacer efface ses réglages (ordre des apps, fond d'écran).")
		if !yes("Le remplacer ?", true) {
			fmt.Println("Accueil laissé tel quel.")
			return
		}
		adb("uninstall", pkgHome)
		out = installAPK(apk, "cleantv-home.apk")
	}
	if !strings.HasPrefix(sh("pm path "+pkgHome), "package:") {
		die("l'installation de l'accueil a échoué : " + lastLine(out))
	}
	fmt.Println("  accueil installé")
	sh("dumpsys deviceidle whitelist +" + pkgHome)

	// Fire OS 6 refuses third-party accessibility services; its Keeper uses usage access instead.
	if sdk < 28 {
		setGuard("")
		sh("appops set " + pkgHome + " GET_USAGE_STATS allow")
		startHome()
		fmt.Println("  bouton Home : cleantv (mode Fire OS 6)")
		return
	}
	sh("cmd package set-home-activity " + pkgHome + "/.Home")
	setFuturity(true)
	setGuard(guard)
	bound := false
	for i := 0; i < 15 && !bound; i++ {
		time.Sleep(time.Second)
		d := sh("dumpsys accessibility")
		bound = strings.Contains(d, guardBind) || strings.Contains(d, guardRaw)
	}
	if !bound {
		fmt.Println("  ATTENTION : Fire OS n'a pas activé le service du bouton Home.")
		if strings.Contains(sh("logcat -d -b all"), "A11yAuthorizer") {
			fmt.Println("  Cette version de Fire OS n'accepte que les services approuvés par Amazon :")
			fmt.Println("  l'accueil cleantv reste installé comme une app, mais Home ouvre l'accueil Amazon.")
		}
	} else {
		fmt.Println("  bouton Home : cleantv")
	}
	startHome()
}

func installAPK(apk []byte, name string) string {
	tmp := filepath.Join(os.TempDir(), name)
	if err := os.WriteFile(tmp, apk, 0o644); err != nil {
		die("écriture impossible : " + err.Error())
	}
	defer os.Remove(tmp)
	return adb("install", "-r", tmp)
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func startHome() {
	sh("input keyevent KEYCODE_WAKEUP")
	sh("am start -n " + pkgHome + "/.Home")
}

func installSmartTube() {
	fmt.Println("Téléchargement de SmartTube...")
	apk, err := download(smartTubeURL)
	if err != nil {
		fmt.Println("  téléchargement impossible : " + err.Error() + " (SmartTube ignoré)")
		return
	}
	sum := sha256.Sum256(apk)
	if hex.EncodeToString(sum[:]) != smartTubeSHA {
		fmt.Println("  fichier différent de la version vérifiée : SmartTube ignoré")
		return
	}
	// Over Wi-Fi the session can drop after the push and before "Success"; the TV's answer counts.
	for i := 0; i < 2; i++ {
		installAPK(apk, "smarttube.apk")
		adb("connect", serial)
		if strings.HasPrefix(sh("pm path "+smartTubePkg), "package:") {
			fmt.Println("  SmartTube installé")
			return
		}
	}
	fmt.Println("  SmartTube n'a pas pu être installé.")
}

// ---------- HOME takeover helpers (ported from launcher/install.sh and scripts/home.sh) ----------

func secureGet(key string) string {
	v := sh("settings get secure " + key)
	if v == "null" {
		return ""
	}
	return v
}

// putSecure falls back to the content provider: Fire OS 6's `settings` silently drops some writes.
func putSecure(key, value string) {
	v := value
	if v == "" {
		v = `""`
	}
	sh("settings put secure " + key + " " + v)
	if got := secureGet(key); got == value {
		return
	}
	sh("content insert --uri content://settings/secure --bind name:s:" + key + " --bind value:s:" + v)
}

// guardList puts svc (or nothing) in place of any launcher's service, keeping every other one.
func guardList(orig, svc string) string {
	var keep []string
	for _, s := range strings.Split(orig, ":") {
		if s != "" && s != guard && s != pkgHome+"/.Guard" && s != oldGuard {
			keep = append(keep, s)
		}
	}
	if svc != "" {
		keep = append(keep, svc)
	}
	return strings.Join(keep, ":")
}

func setGuard(svc string) {
	orig := secureGet("enabled_accessibility_services")
	cur := guardList(orig, svc)
	// Clearing first makes the accessibility manager rebind a service whose process died.
	putSecure("enabled_accessibility_services", "")
	time.Sleep(time.Second)
	if cur != "" {
		putSecure("enabled_accessibility_services", cur)
		putSecure("accessibility_enabled", "1")
	} else {
		putSecure("accessibility_enabled", "0")
	}
}

// Alarms normally fire no sooner than 5 s ahead; the HOME switch needs them immediate.
// Android 11 and older read min_futurity from alarm_manager_constants, 12+ from device_config.

func futurityIn(constants string) string {
	for _, kv := range strings.Split(constants, ",") {
		if strings.HasPrefix(kv, "min_futurity=") {
			return strings.TrimPrefix(kv, "min_futurity=")
		}
	}
	return ""
}

func futurityWith(constants, value string) string {
	var keep []string
	for _, kv := range strings.Split(constants, ",") {
		if kv != "" && !strings.HasPrefix(kv, "min_futurity=") {
			keep = append(keep, kv)
		}
	}
	if value != "" {
		keep = append(keep, "min_futurity="+value)
	}
	return strings.Join(keep, ",")
}

func globalGet(key string) string {
	v := sh("settings get global " + key)
	if v == "null" {
		return ""
	}
	return v
}

func futurityGet(config bool) string {
	if config {
		v := sh("device_config get alarm_manager min_futurity")
		if v == "null" {
			return ""
		}
		return v
	}
	return futurityIn(globalGet("alarm_manager_constants"))
}

func futurityPut(config bool, value string) {
	if config {
		if value != "" {
			sh("device_config put alarm_manager min_futurity " + value)
		} else {
			sh("device_config delete alarm_manager min_futurity")
		}
		return
	}
	if c := futurityWith(globalGet("alarm_manager_constants"), value); c != "" {
		sh("settings put global alarm_manager_constants " + c)
	} else {
		sh("settings delete global alarm_manager_constants")
	}
}

// setFuturity keeps a min_futurity someone set before in dumbtv_min_futurity, and puts it back on undo.
func setFuturity(on bool) {
	config := sdk >= 31
	old := futurityGet(config)
	saved := globalGet("dumbtv_min_futurity")
	if on {
		if old != "" && old != "0" && saved == "" {
			sh("settings put global dumbtv_min_futurity " + old)
		}
		futurityPut(config, "0")
		return
	}
	if _, err := strconv.Atoi(saved); err == nil {
		futurityPut(config, saved)
	} else {
		futurityPut(config, "")
	}
	if saved != "" {
		sh("settings delete global dumbtv_min_futurity")
	}
}

// ---------- restore ----------

func restore() {
	step("Réactivation des services Amazon")
	off := set(sh("pm list packages -d"), "package:")
	n := 0
	for _, p := range append(readList("packages.txt"), readList("alexa.txt")...) {
		if !off[p] {
			continue
		}
		sh("pm enable --user 0 " + p)
		sh("appops set " + p + " RUN_IN_BACKGROUND allow")
		if sdk >= 28 {
			sh("appops set " + p + " RUN_ANY_IN_BACKGROUND allow")
		}
		fmt.Printf("  %-46s réactivé\n", p)
		n++
	}
	fmt.Printf("%d services réactivés.\n", n)

	step("Réglages d'origine")
	applySettings("settings.restore.txt")

	step("Accueil Amazon")
	sh("am force-stop " + pkgHome)
	sh("appops set " + pkgHome + " GET_USAGE_STATS default")
	setFuturity(false)
	setGuard("")
	sh("dumpsys deviceidle whitelist -" + pkgHome)
	sh("am start -a android.intent.action.MAIN -c android.intent.category.HOME")
	fmt.Println("  le bouton Home ouvre de nouveau l'accueil Amazon")
	if yes("Désinstaller aussi l'accueil cleantv ?", false) {
		fmt.Println("  " + adb("uninstall", pkgHome))
	}
	fmt.Println("Restauration terminée. SmartTube et vos apps restent installés.")
}
