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

  function getTransitionTarget() {
    return document.querySelector("[data-app-main]") || document.querySelector("main") || document.body;
  }

  function initPageTransitions() {
    var styleId = "page-transition-style";
    if (!document.getElementById(styleId)) {
      var style = document.createElement("style");
      style.id = styleId;
      style.textContent =
        ".page-transition{opacity:0;transform:translateY(6px);transition:opacity 220ms ease,transform 220ms ease}"+
        ".page-transition.page-ready{opacity:1;transform:translateY(0)}"+
        ".page-transition.page-exit{opacity:0;transform:translateY(-4px)}"+
        "@media (prefers-reduced-motion: reduce){.page-transition{transition:none;transform:none}}";
      document.head.appendChild(style);
    }
    var target = getTransitionTarget();
    if (!target) {
      return;
    }
    target.classList.add("page-transition");
    requestAnimationFrame(function () {
      target.classList.add("page-ready");
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
        target.classList.add("page-exit");
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
      ":root{--ui-surface:rgba(255,255,255,0.88);--ui-surface-strong:rgba(255,255,255,0.96);--ui-border:rgba(148,163,184,0.18);--ui-shadow:0 18px 40px rgba(15,23,42,0.12);--ui-shadow-soft:0 8px 20px rgba(15,23,42,0.08);--ui-text-muted:#6b7280;--ui-accent:#3b82f6;--ui-accent-2:#22c55e;--ui-danger:#ef4444;--ui-warn:#f59e0b;}" +
      ".dark{--ui-surface:rgba(24,34,48,0.94);--ui-surface-strong:rgba(20,30,44,0.98);--ui-border:rgba(86,102,126,0.45);--ui-shadow:0 18px 45px rgba(2,6,23,0.6);--ui-shadow-soft:0 8px 22px rgba(2,6,23,0.45);--ui-text-muted:#9aa8ba;--ui-accent:#7aa2d6;--ui-accent-2:#7cc9b0;--ui-danger:#f17878;--ui-warn:#f3c27a;}" +
      "html,body{font-family:'Manrope','Noto Sans SC',sans-serif;letter-spacing:.01em;}" +
      "body{background:linear-gradient(180deg,#f8fafc 0%,#eef2f7 100%);}" +
      ".dark body{background:linear-gradient(180deg,#223246 0%,#1a2738 100%);}" +
      ".bg-white{background-color:var(--ui-surface-strong)!important;}"+
      ".dark .dark\\:bg-gray-800{background-color:var(--ui-surface)!important;}"+
      ".dark .dark\\:bg-gray-900{background-color:rgba(18,28,42,0.98)!important;}"+
      ".shadow{box-shadow:var(--ui-shadow)!important;}"+
      ".shadow-lg{box-shadow:var(--ui-shadow)!important;}"+
      ".rounded-xl,.rounded-2xl,.rounded-lg{border:1px solid var(--ui-border);backdrop-filter:blur(12px);}"+
      ".bg-gray-50{background-color:rgba(248,250,252,0.72)!important;}"+
      ".dark .dark\\:bg-gray-700{background-color:rgba(28,40,56,0.9)!important;}"+
      ".border-gray-300{border-color:var(--ui-border)!important;}"+
      ".dark .dark\\:border-gray-600{border-color:var(--ui-border)!important;}"+
      "input,select,textarea{transition:border-color .15s ease,box-shadow .15s ease,background-color .2s ease;}"+
      "input:focus,select:focus,textarea:focus{outline:none;box-shadow:0 0 0 3px rgba(90,141,183,0.25);}"+
      "a,button{transition:transform .15s ease,box-shadow .15s ease,background-color .2s ease;border-radius:9999px!important;}"+
      ".metric-card{border-radius:1rem!important;}"+
      "a:hover,button:hover{transform:translateY(-1px);}"+
      "table thead th{font-weight:600;color:var(--ui-text-muted);letter-spacing:.02em;}"+
      "tbody tr:hover{background-color:rgba(148,163,184,0.08);}"+
      ".dark tbody tr:hover{background-color:rgba(148,163,184,0.12);}"+
      ".bg-blue-600{background-image:linear-gradient(135deg,#3b82f6,#2563eb)!important;}"+
      ".bg-blue-600:hover{filter:brightness(1.05);}"+
      ".bg-emerald-600{background-image:linear-gradient(135deg,#22c55e,#16a34a)!important;}"+
      ".bg-indigo-600{background-image:linear-gradient(135deg,#3b82f6,#1d4ed8)!important;}"+
      ".bg-emerald-600:hover{filter:brightness(1.05);}"+
      ".bg-rose-600{background-image:linear-gradient(135deg,#ef4444,#dc2626)!important;}"+
      ".bg-rose-600:hover{filter:brightness(1.05);}"+
      ".bg-slate-200{background-color:rgba(226,232,240,0.85)!important;}"+
      ".dark .dark\\:bg-slate-700{background-color:rgba(51,65,85,0.6)!important;}"+
      ".text-blue-600{color:var(--ui-accent)!important;}"+
      ".text-indigo-600{color:var(--ui-accent)!important;}"+
      ".text-rose-600{color:var(--ui-warn)!important;}"+
      ".dark .text-blue-600{color:#9bb2c6!important;}"+
      ".dark .text-indigo-600{color:#9bb2c6!important;}"+
      ".dark .text-rose-600{color:#c8a07a!important;}"+
      ".dark .bg-blue-600{background-image:linear-gradient(135deg,#5b7a97,#4a677f)!important;}"+
      ".dark .bg-indigo-600{background-image:linear-gradient(135deg,#5b7a97,#4a677f)!important;}"+
      ".dark .bg-rose-600{background-image:linear-gradient(135deg,#b47a6e,#a4685f)!important;}"+
      ".dark .bg-red-600{background-image:linear-gradient(135deg,#b47a6e,#a4685f)!important;}"+
      ".dark .bg-red-700{background-image:linear-gradient(135deg,#a46a60,#935b53)!important;}"+
      ".dark .bg-blue-500{background-color:#5b7a97!important;}"+
      ".dark .bg-indigo-500{background-color:#5b7a97!important;}"+
      ".dark .text-blue-300{color:#b6c7d6!important;}"+
      ".dark .text-indigo-300{color:#b6c7d6!important;}"+
      ".dark .text-rose-300{color:#d7b08d!important;}"+
      "a{color:var(--ui-accent);}"+
      ".dark a{color:#9bb2c6;}"+
      "a:hover{text-decoration:none;}"+
      ".section-shell{border-radius:1rem;border:1px solid var(--ui-border);background:var(--ui-surface-strong);box-shadow:var(--ui-shadow-soft);padding:1.25rem;}"+
      ".section-title{font-size:1.125rem;font-weight:600;letter-spacing:.02em;}"+
      ".section-subtitle{color:var(--ui-text-muted);font-size:.85rem;}"+
      ".metric-card{position:relative;overflow:hidden;border-radius:1rem;border:1px solid var(--ui-border);background:var(--ui-surface-strong);box-shadow:var(--ui-shadow-soft);padding:1.1rem;transition:transform .2s ease,box-shadow .2s ease;}"+
      ".metric-card::before{content:'';position:absolute;inset:0 0 auto 0;height:4px;background:linear-gradient(90deg,var(--accent),transparent);}"+
      ".metric-card:hover{transform:translateY(-3px);box-shadow:var(--ui-shadow);}"+
      ".metric-value{font-size:1.85rem;font-weight:700;color:var(--accent);}"+
      ".metric-label{font-size:.75rem;color:var(--ui-text-muted);letter-spacing:.08em;text-transform:uppercase;}"+
      ".metric-card[data-accent='teal']{--accent:#3b82f6;}"+
      ".metric-card[data-accent='cyan']{--accent:#4f8bff;}"+
      ".metric-card[data-accent='amber']{--accent:#f59e0b;}"+
      ".metric-card[data-accent='slate']{--accent:#64748b;}"+
      ".metric-card[data-accent='violet']{--accent:#8b5cf6;}"+
      ".dark .metric-card[data-accent='teal']{--accent:#8ab4ff;}"+
      ".dark .metric-card[data-accent='cyan']{--accent:#8ab4ff;}"+
      ".dark .metric-card[data-accent='amber']{--accent:#fbbf24;}"+
      ".dark .metric-card[data-accent='slate']{--accent:#cbd5e1;}"+
      ".dark .metric-card[data-accent='violet']{--accent:#c4b5fd;}"+
      ".chart-bar{height:0.5rem;border-radius:999px;background:rgba(148,163,184,0.18);overflow:hidden;}"+
      ".chart-bar > span{display:block;height:100%;background:linear-gradient(90deg,var(--accent),rgba(255,255,255,0.1));}"+
      ".chart-card{border-radius:1rem;border:1px solid var(--ui-border);background:var(--ui-surface-strong);box-shadow:var(--ui-shadow-soft);padding:1.1rem;}"+
      ".chip{display:inline-flex;align-items:center;gap:.35rem;border-radius:999px;padding:.2rem .65rem;font-size:.7rem;font-weight:600;background:rgba(15,23,42,0.06);color:#334155;}"+
      ".dark .chip{background:rgba(148,163,184,0.16);color:#e2e8f0;}";
    document.head.appendChild(style);
  }

  function initAppShell() {
    if (document.body && document.body.dataset && document.body.dataset.noSidebar === "true") {
      return;
    }
    if (document.querySelector("[data-app-sidebar]")) {
      return;
    }
    var shell = document.createElement("div");
    shell.setAttribute("data-app-shell", "true");
    shell.className = "min-h-screen lg:grid lg:grid-cols-[260px_1fr]";

    var aside = document.createElement("aside");
    aside.setAttribute("data-app-sidebar", "true");
    aside.className =
      "hidden lg:flex lg:flex-col lg:sticky lg:top-0 lg:h-screen lg:overflow-y-auto border-b lg:border-b-0 lg:border-r border-gray-200/80 dark:border-gray-700 bg-white/85 dark:bg-gray-900/75 backdrop-blur px-4 py-5";
    aside.innerHTML =
      '<div class="mb-5">' +
      '<div class="text-lg font-bold tracking-wide">Duty-Log-System</div>' +
      '<div class="text-xs text-gray-500 mt-1">值班后台工作台</div>' +
      "</div>" +
      '<nav class="space-y-2 text-sm" data-app-nav>' +
      '<a href="/dashboard" data-nav="/dashboard" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">首页总览</a>' +
      '<a href="/work-tickets" data-nav="/work-tickets" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">网络运维工单</a>' +
      '<a href="/fault-records" data-nav="/fault-records" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">网络故障记录</a>' +
      '<a href="/idc-ops-tickets" data-nav="/idc-ops-tickets" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">IDC运维工单</a>' +
      '<a href="/idc-duty" data-nav="/idc-duty" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">IDC值班记录</a>' +
      '<a href="/ipam" data-nav="/ipam" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">IPAM资产管理</a>' +
      '<a href="/reminders" data-nav="/reminders" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">提醒事项</a>' +
      '<a href="/duty-logs" data-nav="/duty-logs" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">值班日志</a>' +
      '<a href="/reports" data-nav="/reports" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">报表中心</a>' +
      '<a href="/auth/setup-2fa" data-nav="/auth/setup-2fa" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">设置 2FA</a>' +
      '<a href="/admin" data-nav="/admin" class="block px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800">管理后台</a>' +
      "</nav>" +
      '<div class="mt-6 space-y-2">' +
      '<button id="theme-toggle" class="w-full px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800">切换亮暗模式</button>' +
      '<a href="/auth/logout" class="block text-center px-3 py-2 text-sm rounded-lg bg-red-600 text-white hover:bg-red-700">退出登录</a>' +
      "</div>";

    var main = document.createElement("main");
    main.setAttribute("data-app-main", "true");
    main.className = "p-0";

    var nodes = Array.prototype.slice.call(document.body.childNodes);
    var scripts = [];
    nodes.forEach(function (node) {
      if (node === shell) {
        return;
      }
      if (node.nodeType === 1 && node.tagName === "SCRIPT") {
        scripts.push(node);
      } else {
        main.appendChild(node);
      }
    });

    shell.appendChild(aside);
    shell.appendChild(main);
    document.body.appendChild(shell);
    scripts.forEach(function (script) {
      document.body.appendChild(script);
    });

    var path = window.location.pathname || "";
    aside.querySelectorAll("[data-nav]").forEach(function (link) {
      var target = link.getAttribute("data-nav") || "";
      var active = path === target || (target !== "/" && path.indexOf(target) === 0);
      if (active) {
        link.classList.add("bg-blue-600", "text-white");
        link.classList.remove("text-gray-700", "dark:text-gray-200");
      }
    });
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
      initAppShell();
      bindExistingButton();
      ensureFloatingButton();
      refreshButtonText();
      initPageTransitions();
      initUITheme();
    });
  } else {
    initAppShell();
    bindExistingButton();
    ensureFloatingButton();
    refreshButtonText();
    initPageTransitions();
    initUITheme();
  }
})();
