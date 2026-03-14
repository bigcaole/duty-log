;(function () {
  var storageKey = "theme-preference";
  var modeAuto = "auto";
  var modeLight = "light";
  var modeDark = "dark";
  var modeSequence = [modeAuto, modeLight, modeDark];
  var darkQuery = null;

  function initDarkQuery() {
    if (window.matchMedia) {
      darkQuery = window.matchMedia("(prefers-color-scheme: dark)");
    }
  }

  function getSystemMode() {
    if (darkQuery && darkQuery.matches) {
      return modeDark;
    }
    return modeLight;
  }

  function readStoredPreference() {
    try {
      return localStorage.getItem(storageKey);
    } catch (e) {
      return null;
    }
  }

  function writeStoredPreference(value) {
    try {
      localStorage.setItem(storageKey, value);
    } catch (e) {
      // ignore localStorage write errors
    }
  }

  function getPreference() {
    var saved = readStoredPreference();
    if (saved === modeAuto || saved === modeLight || saved === modeDark) {
      return saved;
    }
    return modeAuto;
  }

  function normalizePreference(preference) {
    if (preference === modeLight || preference === modeDark) {
      return preference;
    }
    return modeAuto;
  }

  function resolveAppliedMode(preference) {
    if (preference === modeAuto) {
      return getSystemMode();
    }
    return preference;
  }

  function applyPreference(preference) {
    var normalized = normalizePreference(preference);
    var appliedMode = resolveAppliedMode(normalized);
    document.documentElement.classList.toggle("dark", appliedMode === modeDark);
    document.documentElement.setAttribute("data-theme-preference", normalized);
  }

  function setPreference(preference) {
    var normalized = normalizePreference(preference);
    writeStoredPreference(normalized);
    applyPreference(normalized);
    refreshButtonText();
  }

  function getNextPreference(current) {
    var idx = modeSequence.indexOf(current);
    if (idx < 0) {
      return modeAuto;
    }
    return modeSequence[(idx + 1) % modeSequence.length];
  }

  function cyclePreference() {
    var current = getPreference();
    var next = getNextPreference(current);
    setPreference(next);
  }

  function preferenceLabel(preference) {
    if (preference === modeLight) {
      return "浅色";
    }
    if (preference === modeDark) {
      return "深色";
    }
    return "自动";
  }

  function refreshButtonText() {
    var preference = getPreference();
    var appliedMode = resolveAppliedMode(preference);
    var detail = preference === modeAuto ? "（当前" + (appliedMode === modeDark ? "深色" : "浅色") + "）" : "";
    var text = "主题：" + preferenceLabel(preference) + detail;
    var title = "点击切换主题：自动 / 浅色 / 深色";

    var headerBtn = document.getElementById("theme-toggle");
    if (headerBtn) {
      headerBtn.textContent = text;
      headerBtn.setAttribute("title", title);
    }
    var floatingBtn = document.getElementById("theme-toggle-floating");
    if (floatingBtn) {
      floatingBtn.textContent = text;
      floatingBtn.setAttribute("title", title);
    }
  }

  function bindExistingButton() {
    var btn = document.getElementById("theme-toggle");
    if (!btn) {
      return false;
    }
    btn.addEventListener("click", cyclePreference);
    return true;
  }

  function ensureFloatingButton() {
    if (document.getElementById("theme-toggle") || document.getElementById("theme-toggle-floating")) {
      return;
    }
    var btn = document.createElement("button");
    btn.id = "theme-toggle-floating";
    btn.type = "button";
    btn.className = "fixed bottom-4 right-4 z-50 rounded-lg border border-gray-300 dark:border-gray-600 bg-white/90 dark:bg-gray-800/90 px-3 py-2 text-sm shadow hover:bg-gray-100 dark:hover:bg-gray-700";
    btn.addEventListener("click", cyclePreference);
    document.body.appendChild(btn);
  }

  function initPageTransitions() {
    var styleId = "page-transition-style";
    if (!document.getElementById(styleId)) {
      var style = document.createElement("style");
      style.id = styleId;
      style.textContent = "body.page-transition{opacity:0;transition:opacity 160ms ease}body.page-transition.page-ready{opacity:1}body.page-transition.page-exit{opacity:0}";
      document.head.appendChild(style);
    }
    if (!document.body) {
      return;
    }
    document.body.classList.add("page-transition");
    requestAnimationFrame(function () {
      document.body.classList.add("page-ready");
    });

    document.addEventListener(
      "click",
      function (event) {
        var target = event.target;
        if (!target || !target.closest) {
          return;
        }
        var link = target.closest("a");
        if (!link) {
          return;
        }
        if (link.hasAttribute("download")) {
          return;
        }
        if (link.target && link.target !== "_self") {
          return;
        }
        if (link.getAttribute("data-no-transition") === "true") {
          return;
        }
        var href = link.getAttribute("href");
        if (!href || href.charAt(0) === "#" || href.indexOf("javascript:") === 0) {
          return;
        }
        var url;
        try {
          url = new URL(href, window.location.href);
        } catch (e) {
          return;
        }
        if (url.origin !== window.location.origin) {
          return;
        }
        if (url.pathname === window.location.pathname && url.search === window.location.search && url.hash) {
          return;
        }
        event.preventDefault();
        document.body.classList.add("page-exit");
        setTimeout(function () {
          window.location.href = url.href;
        }, 160);
      },
      true
    );
  }

  function initUITheme() {
    var styleId = "ui-theme-style";
    if (document.getElementById(styleId)) {
      return;
    }
    var style = document.createElement("style");
    style.id = styleId;
    style.textContent =
      "@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=Noto+Sans+SC:wght@400;500;700&display=swap');" +
      ":root{--ui-surface:rgba(255,255,255,0.86);--ui-surface-strong:rgba(255,255,255,0.94);--ui-border:rgba(148,163,184,0.24);--ui-shadow:0 18px 45px rgba(15,23,42,0.12);--ui-shadow-soft:0 10px 30px rgba(15,23,42,0.08);--ui-text-muted:#64748b;}" +
      ".dark{--ui-surface:rgba(15,23,42,0.72);--ui-surface-strong:rgba(15,23,42,0.9);--ui-border:rgba(148,163,184,0.18);--ui-shadow:0 18px 45px rgba(2,6,23,0.5);--ui-shadow-soft:0 10px 30px rgba(2,6,23,0.4);--ui-text-muted:#94a3b8;}" +
      "html,body{font-family:'Manrope','Noto Sans SC',sans-serif;}" +
      "body{background:radial-gradient(circle at 15% 0%,rgba(56,189,248,0.15),transparent 35%),radial-gradient(circle at 85% 100%,rgba(59,130,246,0.14),transparent 35%),#f1f5f9;}" +
      ".dark body{background:radial-gradient(circle at 15% 0%,rgba(14,116,144,0.35),transparent 40%),radial-gradient(circle at 85% 100%,rgba(37,99,235,0.32),transparent 38%),#0b1220;}" +
      ".bg-white{background-color:var(--ui-surface-strong)!important;}"+
      ".dark .dark\\:bg-gray-800{background-color:var(--ui-surface)!important;}"+
      ".dark .dark\\:bg-gray-900{background-color:rgba(2,6,23,0.7)!important;}"+
      ".shadow{box-shadow:var(--ui-shadow)!important;}"+
      ".shadow-lg{box-shadow:var(--ui-shadow)!important;}"+
      ".rounded-xl,.rounded-2xl,.rounded-lg{border:1px solid var(--ui-border);}"+
      ".border-gray-300{border-color:var(--ui-border)!important;}"+
      ".dark .dark\\:border-gray-600{border-color:var(--ui-border)!important;}"+
      "input,select,textarea{transition:border-color .15s ease,box-shadow .15s ease,background-color .2s ease;}"+
      "input:focus,select:focus,textarea:focus{outline:none;box-shadow:0 0 0 3px rgba(59,130,246,0.2);}"+
      "a,button{transition:transform .15s ease,box-shadow .15s ease,background-color .2s ease;}"+
      "a:hover,button:hover{transform:translateY(-1px);}"+
      "table thead th{font-weight:600;color:var(--ui-text-muted);letter-spacing:.02em;}"+
      "tbody tr:hover{background-color:rgba(148,163,184,0.08);}"+
      ".dark tbody tr:hover{background-color:rgba(148,163,184,0.12);}"+
      ".bg-blue-600{background-image:linear-gradient(135deg,#2563eb,#3b82f6)!important;}"+
      ".bg-blue-600:hover{filter:brightness(1.05);}"+
      ".bg-emerald-600{background-image:linear-gradient(135deg,#059669,#10b981)!important;}"+
      ".bg-emerald-600:hover{filter:brightness(1.05);}"+
      ".bg-rose-600{background-image:linear-gradient(135deg,#e11d48,#fb7185)!important;}"+
      ".bg-rose-600:hover{filter:brightness(1.05);}"+
      ".bg-slate-200{background-color:rgba(226,232,240,0.85)!important;}"+
      ".dark .dark\\:bg-slate-700{background-color:rgba(51,65,85,0.6)!important;}"+
      ".text-blue-600{color:#2563eb!important;}"+
      "a{color:#2563eb;}"+
      "a:hover{text-decoration:none;}";
    document.head.appendChild(style);
  }

  initDarkQuery();
  applyPreference(getPreference());

  if (darkQuery) {
    var onThemeChanged = function () {
      if (getPreference() !== modeAuto) {
        return;
      }
      applyPreference(modeAuto);
      refreshButtonText();
    };
    if (darkQuery.addEventListener) {
      darkQuery.addEventListener("change", onThemeChanged);
    } else if (darkQuery.addListener) {
      darkQuery.addListener(onThemeChanged);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      bindExistingButton();
      ensureFloatingButton();
      refreshButtonText();
      initPageTransitions();
      initUITheme();
    });
  } else {
    bindExistingButton();
    ensureFloatingButton();
    refreshButtonText();
    initPageTransitions();
    initUITheme();
  }
})();
