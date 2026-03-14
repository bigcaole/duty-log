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
    return;
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
      ":root{--ui-surface:rgba(255,255,255,0.88);--ui-surface-strong:rgba(255,255,255,0.96);--ui-border:rgba(148,163,184,0.18);--ui-shadow:0 18px 40px rgba(15,23,42,0.12);--ui-shadow-soft:0 8px 20px rgba(15,23,42,0.08);--ui-text-muted:#6b7280;--ui-accent:#3b82f6;--ui-accent-2:#f59e0b;--ui-danger:#ef4444;--ui-warn:#f59e0b;--ui-list-item:rgba(148,163,184,0.12);--ui-list-item-alt:rgba(148,163,184,0.2);}" +
      ".dark{--ui-surface:rgba(24,34,48,0.94);--ui-surface-strong:rgba(20,30,44,0.98);--ui-border:rgba(86,102,126,0.45);--ui-shadow:0 18px 45px rgba(2,6,23,0.6);--ui-shadow-soft:0 8px 22px rgba(2,6,23,0.45);--ui-text-muted:#9aa8ba;--ui-accent:#93a8bd;--ui-accent-2:#c6a37c;--ui-danger:#c7847b;--ui-warn:#c6a37c;--ui-list-item:rgba(255,255,255,0.06);--ui-list-item-alt:rgba(255,255,255,0.1);}" +
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
      ".metric-card{position:relative;overflow:hidden;border-radius:1rem;border:1px solid var(--ui-border);background:var(--ui-surface-strong);box-shadow:var(--ui-shadow-soft);padding:1.1rem;transition:transform .3s ease,box-shadow .3s ease;}"+
      ".metric-card::before{content:'';position:absolute;inset:0 0 auto 0;height:4px;background:linear-gradient(90deg,var(--accent),transparent);}"+
      ".metric-card:hover{transform:translateY(-4px);box-shadow:0 22px 46px rgba(2,6,23,0.35);}"+
      ".metric-value.is-zero{color:var(--ui-text-muted)!important;}"+
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
      ".chip{display:inline-flex;align-items:center;gap:.35rem;border-radius:999px;padding:.2rem .6rem;font-size:.7rem;font-weight:500;border:1px solid var(--ui-border);background:transparent;color:var(--ui-text-muted);}"+
      ".dark .chip{background:transparent;color:var(--ui-text-muted);}"+
      ".list-item{background:var(--ui-list-item);}"+
      ".list-item-alt{background:var(--ui-list-item-alt);}"+
      ".list-zebra > .list-item:nth-child(even){background:var(--ui-list-item-alt);}"+
      "table tbody tr{border-bottom:1px solid var(--ui-border);}"+
      "table tbody tr:nth-child(even){background:var(--ui-list-item);}"+
      ".scroll-beautify::-webkit-scrollbar,.overflow-auto::-webkit-scrollbar{width:6px;height:6px;}"+
      ".scroll-beautify::-webkit-scrollbar-thumb,.overflow-auto::-webkit-scrollbar-thumb{border-radius:999px;background:rgba(148,163,184,0.45);}"+
      ".scroll-beautify::-webkit-scrollbar-track,.overflow-auto::-webkit-scrollbar-track{background:transparent;}"+
      ".nav-active{position:relative;color:var(--ui-accent)!important;border-left:4px solid var(--ui-accent);padding-left:0.75rem;box-shadow:0 0 16px rgba(147,168,189,0.35);}"+
      ".logout-ghost{background:transparent!important;border:1px solid rgba(199,132,123,0.5);color:#d3a49c!important;}"+
      ".logout-ghost:hover{background:linear-gradient(135deg,#b47a6e,#a4685f)!important;color:#fff!important;border-color:transparent!important;}"+
      ".empty-state{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:.5rem;text-align:center;color:var(--ui-text-muted);padding:1.25rem 0;}"+
      ".empty-state::before{content:'';width:42px;height:42px;opacity:.6;background-image:url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'><path d='M7 3h7l4 4v14H7z'/><path d='M14 3v4h4'/><path d='M9 11h6'/><path d='M9 15h6'/></svg>\");background-size:contain;background-repeat:no-repeat;}"+
      ".empty-state-cell{padding:1.25rem 0;}";
    document.head.appendChild(style);
  }

  function initAppShell() {
    if (document.body && document.body.dataset && document.body.dataset.noSidebar === "true") {
      return;
    }
    var aside = document.querySelector("[data-app-sidebar]");
    if (!aside) {
      return;
    }
    var path = window.location.pathname || "";
    aside.querySelectorAll("[data-nav]").forEach(function (link) {
      var target = link.getAttribute("data-nav") || "";
      var active = path === target || (target !== "/" && path.indexOf(target) === 0);
      if (active) {
        link.classList.add("nav-active");
        link.classList.remove("text-gray-700", "dark:text-gray-200");
      }
    });
  }

  function initCountUp() {
    var nodes = Array.prototype.slice.call(document.querySelectorAll("[data-count-value]"));
    if (nodes.length === 0) {
      return;
    }
    nodes.forEach(function (node) {
      if (node.dataset.animated === "true") {
        return;
      }
      var target = parseInt(node.dataset.countValue || "0", 10);
      if (isNaN(target)) {
        return;
      }
      node.dataset.animated = "true";
      if (target <= 0) {
        node.textContent = "0";
        node.classList.add("is-zero");
        return;
      }
      node.classList.remove("is-zero");
      var start = 0;
      var duration = 600;
      var startTime = null;
      var step = function (ts) {
        if (!startTime) {
          startTime = ts;
        }
        var progress = Math.min((ts - startTime) / duration, 1);
        var value = Math.floor(start + (target - start) * progress);
        node.textContent = value.toString();
        if (progress < 1) {
          requestAnimationFrame(step);
        } else {
          node.textContent = target.toString();
        }
      };
      node.textContent = "0";
      requestAnimationFrame(step);
    });
  }

  function initEmptyStates() {
    var nodes = Array.prototype.slice.call(document.querySelectorAll("div, p, td"));
    if (nodes.length === 0) {
      return;
    }
    nodes.forEach(function (node) {
      if (node.dataset && node.dataset.emptyStateBound === "true") {
        return;
      }
      if (node.children && node.children.length > 0) {
        return;
      }
      var text = (node.textContent || "").trim();
      if (!text) {
        return;
      }
      if (!/^(暂无|未找到|没有|无(记录|数据|事项|内容|结果))/.test(text)) {
        return;
      }
      if (node.tagName === "TD") {
        var wrapper = document.createElement("div");
        wrapper.className = "empty-state";
        wrapper.textContent = text;
        node.textContent = "";
        node.appendChild(wrapper);
        node.classList.add("empty-state-cell");
      } else {
        node.classList.add("empty-state");
      }
      if (node.dataset) {
        node.dataset.emptyStateBound = "true";
      }
    });
  }

  initDarkQuery();
  applyPreference(getPreference());
  initUITheme();

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
      initCountUp();
      initEmptyStates();
    });
  } else {
    initAppShell();
    bindExistingButton();
    ensureFloatingButton();
    refreshButtonText();
    initPageTransitions();
    initUITheme();
    initCountUp();
    initEmptyStates();
  }
})();
