(() => {
  const inputs = document.querySelectorAll(".date-input");
  if (!inputs.length) return;

  const formatDate = (value) => {
    const digits = value.replace(/\D/g, "").slice(0, 8);
    const parts = [];
    if (digits.length > 0) {
      parts.push(digits.slice(0, 4));
    }
    if (digits.length > 4) {
      parts.push(digits.slice(4, 6));
    }
    if (digits.length > 6) {
      parts.push(digits.slice(6, 8));
    }
    return parts.join("-");
  };

  inputs.forEach((input) => {
    if (!input.getAttribute("placeholder")) {
      input.setAttribute("placeholder", "YYYY-MM-DD");
    }
    input.setAttribute("inputmode", "numeric");
    input.setAttribute("maxlength", "10");

    const handleInput = () => {
      const before = input.value;
      const cursor = input.selectionStart || 0;
      const digitsBefore = before.slice(0, cursor).replace(/\D/g, "").length;
      const formatted = formatDate(before);
      input.value = formatted;
      let nextCursor = digitsBefore;
      if (digitsBefore > 4) {
        nextCursor += 1;
      }
      if (digitsBefore > 6) {
        nextCursor += 1;
      }
      if (typeof input.setSelectionRange === "function") {
        input.setSelectionRange(nextCursor, nextCursor);
      }
    };

    input.addEventListener("input", handleInput);
  });
})();
