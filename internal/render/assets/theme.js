/* theme.js — pre-paint theme resolver + toggle-button wiring, no dependencies */

(function () {
  "use strict";

  var THEME_KEY = "html-theme";
  var PALETTE_KEY = "html-palette";
  var PALETTES = ["sepia", "blue", "green", "rose", "catppuccin"];
  var root = document.documentElement;

  function isPalette(value) {
    return PALETTES.indexOf(value) !== -1;
  }

  function resolveTheme() {
    try {
      var s = localStorage.getItem(THEME_KEY);
      if (s === "light" || s === "dark") return s;
    } catch (e) {}
    // Optional config default (window.HTML_DEFAULT_THEME) takes precedence over
    // the system preference, but never over an explicit saved choice above.
    var d = window.HTML_DEFAULT_THEME;
    if (d === "light" || d === "dark") return d;
    return matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }

  function resolvePalette() {
    try {
      var s = localStorage.getItem(PALETTE_KEY);
      if (isPalette(s)) return s;
    } catch (e) {}
    var d = window.HTML_DEFAULT_PALETTE;
    if (isPalette(d)) return d;
    return "sepia";
  }

  function applyTheme(t) {
    root.dataset.theme = t;
  }

  function applyPalette(p) {
    root.dataset.palette = isPalette(p) ? p : "sepia";
  }

  // Runs synchronously during <head> parse — sets data-theme before first paint.
  applyTheme(resolveTheme());
  applyPalette(resolvePalette());

  document.addEventListener("DOMContentLoaded", function () {
    var btn = document.getElementById("theme-toggle");
    var paletteButtons = document.querySelectorAll("[data-palette-choice]");

    function sync() {
      if (btn) {
        var dark = root.dataset.theme === "dark";
        btn.setAttribute("aria-pressed", String(dark));
        btn.textContent = dark ? "☀" : "☾"; // ☀ / ☾
        btn.title = dark ? "Switch to light theme" : "Switch to dark theme";
      }
      paletteButtons.forEach(function (paletteButton) {
        var selected = paletteButton.dataset.paletteChoice === root.dataset.palette;
        paletteButton.setAttribute("aria-pressed", String(selected));
      });
    }

    sync();

    if (btn) btn.addEventListener("click", function () {
      var next = root.dataset.theme === "dark" ? "light" : "dark";
      applyTheme(next);
      try {
        localStorage.setItem(THEME_KEY, next);
      } catch (e) {}
      sync();
    });

    paletteButtons.forEach(function (paletteButton) {
      paletteButton.addEventListener("click", function () {
        var next = paletteButton.dataset.paletteChoice;
        applyPalette(next);
        try {
          localStorage.setItem(PALETTE_KEY, root.dataset.palette);
        } catch (e) {}
        sync();
      });
    });
  });
})();
