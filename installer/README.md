# cleantv installer

One file that sets cleantv up on a Fire TV or Fire TV Stick, with no bash, Git or
Android SDK on the computer. Download `cleantv-installer.exe` (Windows), or the
`-linux` / `-macos` build, from the latest CI run's **cleantv-installer** artifact and
run it. It walks through:

1. turning on ADB debugging on the TV,
2. finding the TV's IP address and accepting the prompt on the TV,
3. **Install**: disables the Amazon packages in `data/fireos/packages.txt` (Alexa too
   if you say so), applies `settings.txt`, installs the cleantv home screen and gives
   it the HOME button, and optionally installs SmartTube,
4. **Restore**: re-enables those packages, applies `settings.restore.txt` and hands
   HOME back to Amazon.

It downloads Google's platform-tools once if `adb` is not on the PATH. It does not set
up the DNS filter (`scripts/dnsfilter.sh`).

Build: `./launcher/build.sh && ./installer/build.sh` (needs Go 1.22+).

The APK is signed with the key in the `CLEANTV_KEYSTORE_B64` / `CLEANTV_KEYPASS`
repository secrets. Without them each CI run signs with a throwaway key, and an
installed home screen can then only be replaced after uninstalling it.

Windows SmartScreen warns about the unsigned .exe: More info > Run anyway.
