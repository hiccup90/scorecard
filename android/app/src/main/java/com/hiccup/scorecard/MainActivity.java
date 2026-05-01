package com.hiccup.scorecard;

import android.annotation.SuppressLint;
import android.content.SharedPreferences;
import android.graphics.Bitmap;
import android.net.Uri;
import android.os.Bundle;
import android.text.InputType;
import android.view.View;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.ImageButton;
import android.widget.ProgressBar;
import android.widget.Toast;

import androidx.activity.OnBackPressedCallback;
import androidx.appcompat.app.AlertDialog;
import androidx.appcompat.app.AppCompatActivity;
import androidx.swiperefreshlayout.widget.SwipeRefreshLayout;
import androidx.webkit.WebViewFeature;
import androidx.webkit.WebSettingsCompat;

public class MainActivity extends AppCompatActivity {
    private static final String DEFAULT_HOME_URL = BuildConfig.HOME_URL;
    private static final String PREFS_NAME = "scorecard_prefs";
    private static final String KEY_HOME_URL = "home_url";

    private WebView webView;
    private SwipeRefreshLayout swipeRefreshLayout;
    private ProgressBar progressBar;
    private ImageButton settingsButton;
    private SharedPreferences preferences;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        preferences = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);

        webView = findViewById(R.id.webview);
        swipeRefreshLayout = findViewById(R.id.swipeRefresh);
        progressBar = findViewById(R.id.progressBar);
        settingsButton = findViewById(R.id.settingsButton);

        swipeRefreshLayout.setOnRefreshListener(() -> webView.reload());
        settingsButton.setOnClickListener(v -> showServerSettingsDialog());

        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        settings.setLoadWithOverviewMode(true);
        settings.setUseWideViewPort(true);
        settings.setSupportZoom(false);
        settings.setBuiltInZoomControls(false);
        settings.setDisplayZoomControls(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);

        if (WebViewFeature.isFeatureSupported(WebViewFeature.FORCE_DARK)) {
            WebSettingsCompat.setForceDark(settings, WebSettingsCompat.FORCE_DARK_OFF);
        }

        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onProgressChanged(WebView view, int newProgress) {
                progressBar.setProgress(newProgress);
                progressBar.setVisibility(newProgress < 100 ? View.VISIBLE : View.GONE);
            }
        });

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageStarted(WebView view, String url, Bitmap favicon) {
                swipeRefreshLayout.setRefreshing(true);
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                swipeRefreshLayout.setRefreshing(false);
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                Uri uri = request.getUrl();
                String host = uri.getHost();
                Uri homeUri = Uri.parse(getHomeUrl());
                String homeHost = homeUri.getHost();
                if (host != null && host.equals(homeHost)) {
                    return false;
                }
                Toast.makeText(MainActivity.this, "已阻止跳转到外部网站", Toast.LENGTH_SHORT).show();
                return true;
            }
        });

        if (savedInstanceState != null) {
            webView.restoreState(savedInstanceState);
        } else {
            webView.loadUrl(getHomeUrl());
        }

        getOnBackPressedDispatcher().addCallback(this, new OnBackPressedCallback(true) {
            @Override
            public void handleOnBackPressed() {
                if (webView.canGoBack()) {
                    webView.goBack();
                } else {
                    finish();
                }
            }
        });
    }

    @Override
    protected void onSaveInstanceState(Bundle outState) {
        super.onSaveInstanceState(outState);
        webView.saveState(outState);
    }

    private String getHomeUrl() {
        return preferences.getString(KEY_HOME_URL, DEFAULT_HOME_URL);
    }

    private void saveHomeUrl(String url) {
        preferences.edit().putString(KEY_HOME_URL, url).apply();
    }

    private void showServerSettingsDialog() {
        EditText input = new EditText(this);
        input.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        input.setText(getHomeUrl());
        input.setSelection(input.getText().length());
        int padding = (int) (20 * getResources().getDisplayMetrics().density);
        input.setPadding(padding, padding, padding, padding);

        new AlertDialog.Builder(this)
                .setTitle("服务器地址")
                .setMessage("请输入孩子端要打开的地址，例如 http://10.10.10.16:3003/")
                .setView(input)
                .setNegativeButton("取消", null)
                .setNeutralButton("恢复默认", (dialog, which) -> {
                    saveHomeUrl(DEFAULT_HOME_URL);
                    webView.loadUrl(DEFAULT_HOME_URL);
                    Toast.makeText(this, "已恢复默认地址", Toast.LENGTH_SHORT).show();
                })
                .setPositiveButton("保存", (dialog, which) -> {
                    String raw = input.getText().toString().trim();
                    if (raw.isEmpty()) {
                        Toast.makeText(this, "地址不能为空", Toast.LENGTH_SHORT).show();
                        return;
                    }
                    String normalized = normalizeUrl(raw);
                    if (!isValidHttpUrl(normalized)) {
                        Toast.makeText(this, "请输入正确的 http:// 或 https:// 地址", Toast.LENGTH_LONG).show();
                        return;
                    }
                    saveHomeUrl(normalized);
                    webView.loadUrl(normalized);
                    Toast.makeText(this, "服务器地址已更新", Toast.LENGTH_SHORT).show();
                })
                .show();
    }

    private String normalizeUrl(String value) {
        String url = value;
        if (!url.startsWith("http://") && !url.startsWith("https://")) {
            url = "http://" + url;
        }
        if (!url.endsWith("/")) {
            url = url + "/";
        }
        return url;
    }

    private boolean isValidHttpUrl(String value) {
        try {
            Uri uri = Uri.parse(value);
            String scheme = uri.getScheme();
            String host = uri.getHost();
            return host != null && ("http".equalsIgnoreCase(scheme) || "https".equalsIgnoreCase(scheme));
        } catch (Exception e) {
            return false;
        }
    }

    @Override
    protected void onDestroy() {
        if (webView != null) {
            webView.destroy();
        }
        super.onDestroy();
    }
}
