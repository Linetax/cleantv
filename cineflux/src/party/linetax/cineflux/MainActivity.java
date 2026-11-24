package party.linetax.cineflux;

import android.app.Activity;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;

/**
 * Cineflux pour Fire TV / Android TV : affiche cineflux.linetax.party en plein écran.
 * L'interface vient du site : elle se met à jour sans réinstaller l'APK.
 * Le mot de passe du site est demandé une fois, puis retenu (cookie).
 */
public class MainActivity extends Activity {
    private static final String URL = "https://cineflux.linetax.party/";

    private FrameLayout root;
    private WebView web;
    private View fullscreenView;
    private WebChromeClient.CustomViewCallback fullscreenCallback;
    private boolean failed;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);

        root = new FrameLayout(this);
        root.setBackgroundColor(0xFF0B0B0C);
        web = new WebView(this);
        web.setBackgroundColor(0xFF0B0B0C);
        root.addView(web, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        setContentView(root);

        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setMediaPlaybackRequiresUserGesture(false);
        s.setUserAgentString(s.getUserAgentString() + " CinefluxTV/1");

        CookieManager cookies = CookieManager.getInstance();
        cookies.setAcceptCookie(true);
        cookies.setAcceptThirdPartyCookies(web, true);

        web.setWebViewClient(new WebViewClient() {
            @Override
            @SuppressWarnings("deprecation")
            public void onReceivedError(WebView view, int code, String description, String failingUrl) {
                // Appelé seulement pour la page principale : on affiche un écran "réessayer".
                failed = true;
                view.loadDataWithBaseURL(null,
                    "<html><body style='margin:0;height:100vh;display:flex;flex-direction:column;align-items:center;"
                    + "justify-content:center;background:#0b0b0c;color:#f2f2f0;font-family:sans-serif;text-align:center'>"
                    + "<h1 style='font-size:5vw;margin:0 0 2vw'>Connexion impossible</h1>"
                    + "<p style='font-size:2.2vw;color:#b5b5b2'>Vérifie le Wi-Fi, puis appuie sur OK pour réessayer.</p>"
                    + "</body></html>", "text/html", "utf-8", null);
            }
        });

        web.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onShowCustomView(View view, CustomViewCallback callback) {
                if (fullscreenView != null) { callback.onCustomViewHidden(); return; }
                fullscreenView = view;
                fullscreenCallback = callback;
                root.addView(view, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
                web.setVisibility(View.GONE);
                hideSystemBars();
            }

            @Override
            public void onHideCustomView() {
                exitFullscreen();
            }
        });

        load();
    }

    private void load() {
        failed = false;
        web.loadUrl(URL);
    }

    private void exitFullscreen() {
        if (fullscreenView == null) return;
        root.removeView(fullscreenView);
        fullscreenView = null;
        web.setVisibility(View.VISIBLE);
        if (fullscreenCallback != null) fullscreenCallback.onCustomViewHidden();
        fullscreenCallback = null;
        web.requestFocus();
    }

    private void hideSystemBars() {
        getWindow().getDecorView().setSystemUiVisibility(
            View.SYSTEM_UI_FLAG_FULLSCREEN | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
            | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
            | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN);
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        if (failed && event.getAction() == KeyEvent.ACTION_UP
                && (event.getKeyCode() == KeyEvent.KEYCODE_DPAD_CENTER || event.getKeyCode() == KeyEvent.KEYCODE_ENTER)) {
            load();
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    @Override
    public void onBackPressed() {
        if (fullscreenView != null) { exitFullscreen(); return; }
        if (failed) { finish(); return; }
        // La page gère son propre retour (fiche -> accueil...). Elle répond false quand il faut quitter.
        web.evaluateJavascript("window.cinefluxBack ? window.cinefluxBack() : false", new ValueCallback<String>() {
            @Override
            public void onReceiveValue(String handled) {
                if (!"true".equals(handled)) finish();
            }
        });
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) hideSystemBars();
    }

    @Override
    protected void onPause() {
        CookieManager.getInstance().flush();
        web.onPause();
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        web.onResume();
    }

    @Override
    protected void onDestroy() {
        root.removeAllViews();
        web.destroy();
        super.onDestroy();
    }
}
