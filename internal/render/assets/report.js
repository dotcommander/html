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

  document.querySelectorAll("[data-report-slides]").forEach((deck) => {
    const slides = Array.from(deck.querySelectorAll(".report-slide"));
    if (slides.length <= 1) return;
    const prev = deck.querySelector("[data-slide-prev]");
    const nextButton = deck.querySelector("[data-slide-next]");
    const status = deck.querySelector("[data-slide-status]");

    let index = 0;
    const show = (next) => {
      index = Math.max(0, Math.min(slides.length - 1, next));
      slides.forEach((slide, i) => {
        const active = i === index;
        slide.hidden = !active;
        slide.setAttribute("aria-current", active ? "true" : "false");
      });
      if (prev) {
        prev.disabled = index === 0;
      }
      if (nextButton) {
        nextButton.disabled = index === slides.length - 1;
      }
      if (status) {
        status.textContent = `${index + 1} / ${slides.length}`;
      }
      const active = slides[index];
      active.tabIndex = -1;
      active.focus({ preventScroll: true });
    };

    show(0);

    if (prev) {
      prev.addEventListener("click", () => show(index - 1));
    }
    if (nextButton) {
      nextButton.addEventListener("click", () => show(index + 1));
    }

    document.addEventListener("keydown", (event) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) {
        return;
      }
      const t = event.target;
      if (t && (t.isContentEditable || t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT")) {
        return;
      }
      let next = null;
      switch (event.key) {
        case "ArrowRight":
        case "ArrowDown":
        case "PageDown":
        case " ":
          next = index + 1;
          break;
        case "ArrowLeft":
        case "ArrowUp":
        case "PageUp":
          next = index - 1;
          break;
        case "Home":
          next = 0;
          break;
        case "End":
          next = slides.length - 1;
          break;
        default:
          return;
      }
      event.preventDefault();
      show(next);
    });
  });

  document.querySelectorAll(".report-table-wrap").forEach((wrap) => {
    const input = wrap.querySelector(".report-filter");
    const status = wrap.querySelector(".report-filter-status");
    const table = wrap.querySelector("[data-report-table]");
    const mobileSort = wrap.querySelector(".report-mobile-sort select");
    if (!table) return;
    const tbody = table.tBodies[0];
    if (!tbody) return;
    const emptyRow = tbody.querySelector("[data-report-empty-row]");
    const dataRows = () => Array.from(tbody.rows).filter((row) => !row.hasAttribute("data-report-empty-row"));

    const updateStatus = () => {
      const rows = dataRows();
      const visible = rows.filter((row) => !row.hidden).length;
      if (emptyRow) {
        emptyRow.hidden = visible !== 0;
        const cell = emptyRow.cells[0];
        if (cell) {
          cell.textContent = rows.length === 0 ? "No rows" : "No rows match";
        }
      }
      if (!status) return;
      status.textContent = rows.length === 0 ? "No rows" : visible === 0 ? "No rows match" : `${visible} ${visible === 1 ? "row" : "rows"}`;
    };

    if (input) {
      input.addEventListener("input", () => {
        const needle = input.value.toLowerCase();
        dataRows().forEach((row) => {
          row.hidden = needle && !row.textContent.toLowerCase().includes(needle);
        });
        updateStatus();
      });
    }
    updateStatus();

    const headers = Array.from(table.tHead.rows[0].cells);
    const syncSortLabels = () => {
      headers.forEach((cell, index) => {
        const button = cell.querySelector("button");
        if (!button) return;
        const label = button.dataset.sortLabel || button.textContent.trim() || `column ${index + 1}`;
        const nextDirection = cell.getAttribute("aria-sort") === "ascending" ? "descending" : "ascending";
        button.setAttribute("aria-label", `Sort by ${label} ${nextDirection}`);
      });
      if (mobileSort) {
        const activeIndex = headers.findIndex((cell) => cell.hasAttribute("aria-sort"));
        if (activeIndex < 0) {
          mobileSort.value = "";
          return;
        }
        mobileSort.value = `${activeIndex}:${headers[activeIndex].getAttribute("aria-sort")}`;
      }
    };
    syncSortLabels();

    const sortBy = (index, asc) => {
      const rows = dataRows();
      rows.sort((a, b) => {
        const av = a.cells[index]?.textContent || "";
        const bv = b.cells[index]?.textContent || "";
        const cmp = av.localeCompare(bv, undefined, { numeric: true, sensitivity: "base" });
        return asc ? cmp : -cmp;
      });
      headers.forEach((header) => header.removeAttribute("aria-sort"));
      if (headers[index]) {
        headers[index].setAttribute("aria-sort", asc ? "ascending" : "descending");
      }
      syncSortLabels();
      rows.forEach((row) => tbody.appendChild(row));
    };

    headers.forEach((cell, index) => {
      const button = cell.querySelector("button");
      if (!button) return;
      button.addEventListener("click", () => {
        const asc = cell.getAttribute("aria-sort") !== "ascending";
        sortBy(index, asc);
      });
    });

    if (mobileSort) {
      mobileSort.addEventListener("change", () => {
        const [index, direction] = mobileSort.value.split(":");
        const column = Number(index);
        if (!Number.isInteger(column) || column < 0 || column >= headers.length) return;
        sortBy(column, direction !== "descending");
      });
    }
  });
})();

(() => {
  const comments = Array.from(document.querySelectorAll(".review-card .review-comment"));
  if (comments.length === 0) return;

  const keyFor = (el) => `html-review:${el.dataset.reviewId || ""}`;

  comments.forEach((el) => {
    try {
      const saved = localStorage.getItem(keyFor(el));
      if (saved !== null) el.value = saved;
    } catch (e) {}
    el.addEventListener("input", () => {
      try {
        localStorage.setItem(keyFor(el), el.value);
      } catch (e) {}
    });
  });

  const copyButton = document.querySelector(".review-copy");
  if (!copyButton) return;

  const buildExport = () => {
    const blocks = [];
    comments.forEach((el) => {
      const value = el.value.trim();
      if (!value) return;
      const id = el.dataset.reviewId || "";
      blocks.push(`## ${id}\n${value}`);
    });
    if (blocks.length === 0) return "";
    return `# Review comments\n\n${blocks.join("\n\n")}`;
  };

  copyButton.addEventListener("click", () => {
    const text = buildExport();
    if (!text) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).catch(() => fallbackCopy(text));
    } else {
      fallbackCopy(text);
    }
  });

  function fallbackCopy(text) {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "absolute";
    area.style.left = "-9999px";
    document.body.appendChild(area);
    area.select();
    try {
      document.execCommand("copy");
    } catch (e) {}
    document.body.removeChild(area);
  }
})();
