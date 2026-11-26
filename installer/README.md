# cleantv installer

One file that sets cleantv up on a Fire TV or Fire TV Stick, with no bash, Git or
Android SDK on the computer. Download `cleantv-installer.exe` (Windows), or the
`-linux` / `-macos` build, from the latest CI run's **cleantv-installer** artifact and
run it. It walks through:

1. turning on ADB debugging on the TV,
2. finding the TV's IP address and accepting the prompt on the TV,
3. **Install**: disables the Amazon packages in `data/fireos/packages.txt` (Alexa too
   if you say so), applies `settings.txt`, installs the cleantv home screen and gives
   it the HOME button, and installs the embedded SmartTube (first on a fresh home screen)
   and, if you say yes, Cineflux (`cineflux/`, second),
4. **Restore**: re-enables those packages, applies `settings.restore.txt` and hands
   HOME back to Amazon.

It downloads Google's platform-tools once if `adb` is not on the PATH. It does not set
up the DNS filter (`scripts/dnsfilter.sh`).

Build: `./launcher/build.sh && ./cineflux/build.sh && ./installer/build.sh` (needs Go 1.22+).

The APK is signed with the key in the `CLEANTV_KEYSTORE_B64` / `CLEANTV_KEYPASS`
repository secrets. Without them each CI run signs with a throwaway key, and an
installed home screen can then only be replaced after uninstalling it.

`samsung/` builds `cleantv-samsung.exe`: it installs TizenBrew (which bundles TizenTube)
on a Samsung TV in developer mode, over sdb (ADB protocol on TCP 26101), as TizenBrew
Installer does for Tizen 3 to 6. The widget is pinned by hash in `build.sh`. Tizen 7+
needs a Samsung-account certificate and is refused. Nothing is removed from the TV:
Tizen gives no shell, so there is no debloat (see `docs/OTHER-TVS.md`). On request it adds
the Cineflux module (`../tizenbrew-cineflux`) to TizenBrew's settings file, keeping the
modules already listed.

Both installers show a window: `ui/` serves a small page on 127.0.0.1 (random port,
random token) and opens it in an Edge app window (Chrome, then the default browser, as
fallbacks); the program ends when the window closes. The page draws every screen from
what the installer prints and asks, so the logic in `main.go` is the same in both modes.
`--console` keeps the old terminal mode (Linux/macOS; `e2e/run.sh` uses it). A picture
named `ui/web/img/<app>-<step>.svg` or `.png` (e.g. `cleantv-preparer-la-tv.svg`) replaces the
drawing next to that step's instructions. `e2e/gui.cjs` drives both windows in Chrome
against fake TVs and saves a screenshot of each screen.

Windows SmartScreen warns about the unsigned .exe: More info > Run anyway.
