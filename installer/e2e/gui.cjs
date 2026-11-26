// Drives the window of both installers in Chromium, against the fake Fire TV (fake-adb)
// and a fake Samsung TV (below), and checks what reaches the TV. With SHOTS=dir it also
// saves a screenshot of every screen.
// Usage: node installer/e2e/gui.cjs FIRETV_BINARY SAMSUNG_BINARY
"use strict";
const { spawn } = require("child_process");
const fs = require("fs");
const net = require("net");
const os = require("os");
const path = require("path");
let pw;
try { pw = require("playwright"); } catch { pw = require("playwright-core"); }

const [fireBin, samsungBin] = process.argv.slice(2).map((p) => path.resolve(p));
const shots = process.env.SHOTS && path.resolve(process.env.SHOTS);
const here = __dirname;
let fails = 0;

function check(name, ok) {
  console.log((ok ? "ok    " : "FAIL  ") + name);
  if (!ok) fails++;
}

// start runs an installer in window mode and resolves with its URL and a way to wait for its exit.
function start(bin, mode, extraEnv = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cleantv-gui-"));
  fs.mkdirSync(path.join(dir, "bin"));
  fs.mkdirSync(path.join(dir, "cache"));
  fs.symlinkSync(path.join(here, "fake-adb"), path.join(dir, "bin", "adb"));
  const proc = spawn(bin, [], {
    env: {
      ...process.env, ...extraEnv, FAKE_DIR: dir, FAKE_MODE: mode, CLEANTV_NO_BROWSER: "1",
      XDG_CACHE_HOME: path.join(dir, "cache"), HOME: dir, PATH: path.join(dir, "bin") + ":" + process.env.PATH,
    },
  });
  const exited = new Promise((r) => proc.on("exit", (code) => r(code)));
  const url = new Promise((resolve, reject) => {
    let out = "";
    proc.stdout.on("data", (d) => {
      out += d;
      const m = out.match(/Interface : (http:\/\/\S+)/);
      if (m) resolve(m[1]);
    });
    proc.stderr.on("data", (d) => process.stderr.write(d));
    exited.then((c) => reject(new Error("exited " + c + " before showing its window")));
  });
  const read = (f) => { try { return fs.readFileSync(path.join(dir, f), "utf8"); } catch { return ""; } };
  return { url, exited, proc, log: () => read("log"), pushed: () => read("pushed") };
}

let n = 0;
async function shot(page, name) {
  await page.waitForTimeout(450); // let the entrance animation finish
  if (shots) await page.screenshot({ path: path.join(shots, `${String(++n).padStart(2, "0")}-${name}.png`) });
}

const visible = (page, text) => page.getByText(text, { exact: false }).first().waitFor({ timeout: 20000 });
const click = (page, name) => page.getByRole("button", { name, exact: false }).first().click();

async function closeWindow(page, run, name) {
  await click(page, "Fermer");
  const code = await Promise.race([run.exited, new Promise((r) => setTimeout(() => r("still running"), 10000))]);
  check(name + ": the program ends when its window is closed", code === 0 || code === 1);
}

async function fireInstall(browser) {
  const run = start(fireBin, "slowauth", { FAKE_SLOW: "0.15" });
  const page = await browser.newPage();
  await page.goto(await run.url);
  await visible(page, "Bienvenue");
  await shot(page, "firetv-bienvenue");
  await click(page, "Commencer");
  await visible(page, "Préparer la TV");
  await shot(page, "firetv-preparer");
  await page.locator(".art img").click();
  await page.locator(".zoom").waitFor();
  await shot(page, "firetv-preparer-agrandi");
  await page.locator(".zoom").click();
  await click(page, "C'est fait");
  await page.locator("#answer").waitFor();
  await shot(page, "firetv-adresse");
  await page.fill("#answer", "192.168.1");
  await click(page, "Continuer");
  await visible(page, "pas une adresse IP valide");
  await shot(page, "firetv-adresse-invalide");
  check("fire tv window: refuses a wrong address and asks again", true);
  await page.fill("#answer", "203.0.113.7");
  await click(page, "Continuer");
  await visible(page, "Regardez la TV");
  await shot(page, "firetv-accepter-sur-la-tv");
  await visible(page, "Que voulez-vous faire");
  await shot(page, "firetv-menu");
  check("fire tv window: shows the device", await page.getByText("Amazon Fake TV").count() > 0);
  await click(page, "Installer cleantv");
  await visible(page, "Garder Alexa");
  await shot(page, "firetv-question-alexa");
  await click(page, "Oui");
  await visible(page, "Installer Cineflux");
  await shot(page, "firetv-question-cineflux");
  await click(page, "Oui");
  await page.locator("#busyTitle").waitFor({ timeout: 20000 });
  await page.waitForTimeout(600);
  await shot(page, "firetv-installation");
  await visible(page, "Redémarrer la TV maintenant");
  await shot(page, "firetv-question-redemarrer");
  await click(page, "Oui");
  await visible(page, "C'est terminé");
  await shot(page, "firetv-termine");
  const log = run.log();
  check("fire tv window: disables an ad package", log.includes("pm disable-user --user 0 com.amazon.hybridadidservice"));
  check("fire tv window: never touches Amazon's launcher", !log.includes("disable-user --user 0 com.amazon.tv.launcher"));
  check("fire tv window: SmartTube then Cineflux on the home screen", run.pushed().includes("apps.order=org.smarttube.stable,party.linetax.cineflux"));
  check("fire tv window: takes over HOME", log.includes("set-home-activity tv.dumbtv.home/.Home"));
  check("fire tv window: reboots when asked", /^-s 203\.0\.113\.7:5555 reboot$/m.test(log));
  await closeWindow(page, run, "fire tv install");
}

async function fireRestore(browser) {
  const run = start(fireBin, "fireos8");
  const page = await browser.newPage();
  await page.goto(await run.url);
  await click(page, "Commencer");
  await click(page, "C'est fait");
  await page.fill("#answer", "203.0.113.7");
  await click(page, "Continuer");
  await click(page, "Tout restaurer");
  await visible(page, "Désinstaller aussi");
  await shot(page, "firetv-restaurer-question");
  await click(page, "Non");
  await visible(page, "C'est terminé");
  await shot(page, "firetv-restaurer-termine");
  const log = run.log();
  check("restore window: re-enables a disabled package", log.includes("pm enable --user 0 com.amazon.shoptv.client"));
  check("restore window: keeps the home screen when told no", !log.includes("uninstall tv.dumbtv.home"));
  await closeWindow(page, run, "restore");
}

async function fireError(browser) {
  const run = start(fireBin, "notfire");
  const page = await browser.newPage();
  await page.goto(await run.url);
  await click(page, "Commencer");
  await click(page, "C'est fait");
  await page.fill("#answer", "203.0.113.7");
  await click(page, "Continuer");
  await visible(page, "Un problème est survenu");
  await shot(page, "firetv-erreur");
  check("error window: explains a TV that is not a Fire TV", await page.getByText("pas une Fire TV").count() > 0);
  await closeWindow(page, run, "error");
}

// fakeSamsung answers like sdbd on a Tizen 6 TV: sysinfo, sync push and pull, vd_appinstall.
function fakeSamsung() {
  const C = { CNXN: 0x4e584e43, OPEN: 0x4e45504f, OKAY: 0x59414b4f, CLSE: 0x45534c43, WRTE: 0x45545257 };
  const got = { pushed: 0, installed: false, files: {} };
  const msg = (id, data = Buffer.alloc(0)) => {
    const h = Buffer.alloc(8);
    h.write(id); h.writeUInt32LE(data.length, 4);
    return Buffer.concat([h, data]);
  };
  // syncRequest reads one complete request (SEND path,mode DATA... DONE | RECV path | QUIT): [used, kind, path, data], used 0 if incomplete.
  const syncRequest = (b) => {
    if (b.length < 8) return [0];
    const kind = b.toString("latin1", 0, 4), n = b.readUInt32LE(4);
    if (kind === "QUIT") return [8, kind];
    if (b.length < 8 + n) return [0];
    let path = b.toString("utf8", 8, 8 + n), used = 8 + n;
    if (kind === "RECV") return [used, kind, path];
    path = path.split(",")[0];
    const parts = [];
    while (b.length >= used + 8) {
      const id = b.toString("latin1", used, used + 4), len = b.readUInt32LE(used + 4);
      if (id === "DONE") return [used + 8, kind, path, Buffer.concat(parts)];
      if (b.length < used + 8 + len) break;
      parts.push(b.subarray(used + 8, used + 8 + len));
      used += 8 + len;
    }
    return [0];
  };
  const server = net.createServer((sock) => {
    let buf = Buffer.alloc(0);
    const svc = {};
    const send = (cmd, a0, a1, data = Buffer.alloc(0)) => {
      const h = Buffer.alloc(24);
      let sum = 0;
      for (const b of data) sum += b;
      h.writeUInt32LE(cmd, 0); h.writeUInt32LE(a0, 4); h.writeUInt32LE(a1, 8);
      h.writeUInt32LE(data.length, 12); h.writeUInt32LE(sum >>> 0, 16); h.writeUInt32LE((cmd ^ 0xffffffff) >>> 0, 20);
      sock.write(Buffer.concat([h, data]));
    };
    let sync = Buffer.alloc(0);
    sock.on("data", (d) => {
      buf = Buffer.concat([buf, d]);
      while (buf.length >= 24 && buf.length >= 24 + buf.readUInt32LE(12)) {
        const cmd = buf.readUInt32LE(0), a0 = buf.readUInt32LE(4), a1 = buf.readUInt32LE(8), len = buf.readUInt32LE(12);
        const data = buf.subarray(24, 24 + len);
        buf = buf.subarray(24 + len);
        if (cmd === C.CNXN) send(C.CNXN, 0x01000000, 4096, Buffer.from("device::\0"));
        if (cmd === C.OPEN) {
          const name = data.toString().replace(/\0+$/, ""), remote = a0 + 100;
          svc[remote] = name;
          send(C.OKAY, remote, a0);
          if (name === "sysinfo:") { const info = Buffer.alloc(320); info.write("6.0", 192); send(C.WRTE, remote, a0, info); }
          if (name.startsWith("shell:0 vd_appinstall xvvl3S1bvH ")) {
            got.installed = true;
            setTimeout(() => {
              send(C.WRTE, remote, a0, Buffer.from("app_id[xvvl3S1bvH.TizenBrewStandalone] install completed\nspend time for wascmd is [900]ms\n"));
              send(C.CLSE, remote, a0);
            }, 1500);
          }
        }
        if (cmd === C.WRTE) {
          send(C.OKAY, a1, a0);
          if (svc[a1] === "sync:") {
            sync = Buffer.concat([sync, data]);
            for (let r = syncRequest(sync); r[0]; r = syncRequest(sync)) {
              const [used, kind, file, content] = r;
              sync = sync.subarray(used);
              if (kind === "SEND") {
                got.files[file] = content;
                if (file.endsWith("package.wgt")) got.pushed = content.length;
                send(C.WRTE, a1, a0, Buffer.from("OKAY\0\0\0\0"));
              }
              if (kind === "RECV") {
                send(C.WRTE, a1, a0, got.files[file] ? Buffer.concat([msg("DATA", got.files[file]), msg("DONE")]) : msg("FAIL", Buffer.from("No such file or directory")));
              }
            }
          }
        }
      }
    });
    sock.on("error", () => {});
  });
  return new Promise((r) => server.listen(26101, "127.0.0.1", () => r({ server, got })));
}

async function samsung(browser) {
  const tv = await fakeSamsung();
  const run = start(samsungBin, "");
  const page = await browser.newPage();
  await page.goto(await run.url);
  await visible(page, "Bienvenue");
  await shot(page, "samsung-bienvenue");
  await click(page, "Commencer");
  await visible(page, "Activer le mode développeur");
  await shot(page, "samsung-mode-developpeur");
  await page.locator(".main").evaluate((m) => m.scrollTo(0, m.scrollHeight)); // step D sits below the fold
  await shot(page, "samsung-mode-developpeur-suite");
  await click(page, "La TV a redémarré");
  await page.fill("#answer", "127.0.0.1");
  await shot(page, "samsung-adresse");
  await click(page, "Continuer");
  await visible(page, "Ajouter Cineflux");
  await shot(page, "samsung-question-cineflux");
  await click(page, "Oui");
  await page.locator("#busyTitle", { hasText: "Installer TizenBrew" }).waitFor({ timeout: 20000 });
  await shot(page, "samsung-installation");
  await visible(page, "C'est terminé");
  await shot(page, "samsung-termine");
  check("samsung window: sends TizenBrew to the TV", tv.got.pushed > 0);
  check("samsung window: installs it", tv.got.installed);
  const cfg = String(tv.got.files["/home/owner/share/tizenbrewConfig.json"] || "");
  check("samsung window: adds Cineflux next to TizenTube in TizenBrew", cfg.includes("npm/@foxreis/tizentube") && cfg.includes("gh/Linetax/cineflux-tizenbrew"));
  check("samsung window: shows the last settings to make", await page.getByText("127.0.0.1").count() > 0);
  await closeWindow(page, run, "samsung");
  tv.server.close();
}

(async () => {
  if (shots) fs.mkdirSync(shots, { recursive: true });
  const browser = await pw.chromium.launch(process.env.CHROME ? { executablePath: process.env.CHROME } : {});
  const ctx = await browser.newContext({ viewport: { width: 1080, height: 720 }, deviceScaleFactor: shots ? 2 : 1 });
  browser.newPage = () => ctx.newPage();
  for (const t of [fireInstall, fireRestore, fireError, samsung]) {
    try { await t(browser); } catch (e) { check(t.name + ": " + e.message.split("\n")[0], false); }
  }
  await browser.close();
  console.log(fails ? `\n${fails} check(s) failed` : "\nall window scenarios passed");
  process.exit(fails ? 1 : 0);
})();
