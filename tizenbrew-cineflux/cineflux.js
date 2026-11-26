// Injected by TizenBrew into cineflux.linetax.party. Same Back behaviour as the Android app:
// the page handles its own back (sheet -> home...) and answers false when the app should close.
(function () {
  if (window.__cinefluxTizen) return;
  window.__cinefluxTizen = true;
  document.addEventListener("keydown", function (e) {
    if (e.keyCode !== 10009) return; // Back on a Samsung remote
    e.preventDefault();
    if (window.cinefluxBack && window.cinefluxBack()) return;
    try { tizen.application.getCurrentApplication().exit(); } catch (err) { history.back(); }
  }, true);
})();
