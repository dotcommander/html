(() => {
  document.querySelectorAll("[data-report-tabs]").forEach((tabs) => {
    const buttons = Array.from(tabs.querySelectorAll('[role="tab"]'));
    const panels = Array.from(tabs.querySelectorAll('[role="tabpanel"]'));

    const selectTab = (index, focus) => {
      buttons.forEach((button, i) => {
        const selected = i === index;
        button.setAttribute("aria-selected", String(selected));
        button.setAttribute("tabindex", selected ? "0" : "-1");
        if (selected && focus) {
          button.focus();
        }
      });
      panels.forEach((panel, i) => {
        panel.hidden = i !== index;
      });
    };

    const selectedIndex = buttons.findIndex((button) => button.getAttribute("aria-selected") === "true");
    selectTab(selectedIndex >= 0 ? selectedIndex : 0, false);

    buttons.forEach((button, index) => {
      button.addEventListener("click", () => {
        selectTab(index, false);
      });
      button.addEventListener("keydown", (event) => {
        const last = buttons.length - 1;
        let next = null;
        switch (event.key) {
          case "ArrowLeft":
          case "ArrowUp":
            next = index === 0 ? last : index - 1;
            break;
          case "ArrowRight":
          case "ArrowDown":
            next = index === last ? 0 : index + 1;
            break;
          case "Home":
            next = 0;
            break;
          case "End":
            next = last;
            break;
          default:
            return;
        }
        event.preventDefault();
        selectTab(next, true);
      });
    });
  });

  document.querySelectorAll(".report-table-wrap").forEach((wrap) => {
    const input = wrap.querySelector(".report-filter");
    const status = wrap.querySelector(".report-filter-status");
    const table = wrap.querySelector("[data-report-table]");
    if (!table) return;
    const tbody = table.tBodies[0];
    if (!tbody) return;

    const updateStatus = () => {
      if (!status) return;
      const visible = Array.from(tbody.rows).filter((row) => !row.hidden).length;
      status.textContent = visible === 0 ? "No rows match" : `${visible} ${visible === 1 ? "row" : "rows"}`;
    };

    if (input) {
      input.addEventListener("input", () => {
        const needle = input.value.toLowerCase();
        Array.from(tbody.rows).forEach((row) => {
          row.hidden = needle && !row.textContent.toLowerCase().includes(needle);
        });
        updateStatus();
      });
    }
    updateStatus();

    Array.from(table.tHead.rows[0].cells).forEach((cell, index) => {
      const button = cell.querySelector("button");
      if (!button) return;
      button.addEventListener("click", () => {
        const asc = cell.getAttribute("aria-sort") !== "ascending";
        const rows = Array.from(tbody.rows);
        rows.sort((a, b) => {
          const av = a.cells[index]?.textContent || "";
          const bv = b.cells[index]?.textContent || "";
          const cmp = av.localeCompare(bv, undefined, { numeric: true, sensitivity: "base" });
          return asc ? cmp : -cmp;
        });
        Array.from(table.tHead.rows[0].cells).forEach((header) => header.removeAttribute("aria-sort"));
        cell.setAttribute("aria-sort", asc ? "ascending" : "descending");
        rows.forEach((row) => tbody.appendChild(row));
      });
    });
  });
})();
