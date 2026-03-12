(() => {
  const cards = document.querySelectorAll("[data-reminder-card]");
  if (!cards.length) return;

  cards.forEach((card) => {
    const toggleBtn = card.querySelector("[data-reminder-toggle]");
    const expandBtn = card.querySelector("[data-reminder-expand]");
    const enabledInput = card.querySelector("[data-reminder-enabled-input]");
    const fields = card.querySelector("[data-reminder-fields]");

    let enabled = card.getAttribute("data-reminder-enabled") === "true";
    let expanded = enabled;

    const render = () => {
      if (toggleBtn) {
        toggleBtn.textContent = enabled ? "已开启" : "已关闭";
        toggleBtn.classList.toggle("bg-emerald-600", enabled);
        toggleBtn.classList.toggle("bg-gray-500", !enabled);
      }
      if (enabledInput) {
        enabledInput.value = enabled ? "1" : "0";
      }
      if (fields) {
        fields.style.display = expanded ? "" : "none";
      }
      if (expandBtn) {
        expandBtn.textContent = expanded ? "收起" : "展开";
      }
    };

    if (toggleBtn) {
      toggleBtn.addEventListener("click", () => {
        enabled = !enabled;
        if (enabled && !expanded) {
          expanded = true;
        }
        render();
      });
    }
    if (expandBtn) {
      expandBtn.addEventListener("click", () => {
        expanded = !expanded;
        render();
      });
    }

    render();
  });
})();
