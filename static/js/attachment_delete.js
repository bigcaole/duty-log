(() => {
  const attachDelete = (btn) => {
    const value = btn.getAttribute("data-attachment-delete");
    if (!value) return;
    const item = btn.closest("[data-attachment-item]");
    if (!item) return;

    if (value.startsWith("db:")) {
      const id = value.slice(3);
      if (!id) return;
      btn.disabled = true;
      fetch(`/attachments/${id}/delete`, { method: "POST" })
        .then((res) => res.json().catch(() => null))
        .then((data) => {
          if (data && data.ok) {
            item.remove();
            return;
          }
          btn.disabled = false;
          alert((data && data.error) || "删除失败");
        })
        .catch(() => {
          btn.disabled = false;
          alert("删除失败");
        });
      return;
    }

    if (value.startsWith("fs:")) {
      const form = btn.closest("form");
      if (form) {
        const hidden = document.createElement("input");
        hidden.type = "hidden";
        hidden.name = "remove_attachments";
        hidden.value = value;
        form.appendChild(hidden);
      }
      item.remove();
    }
  };

  document.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;
    const btn = target.closest("[data-attachment-delete]");
    if (!btn) return;
    event.preventDefault();
    attachDelete(btn);
  });
})();
