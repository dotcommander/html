/* theme.js — pre-paint theme resolver + toggle-button wiring, no dependencies */

(function () {
  "use strict";

  var KEY = "html-theme";
  var root = document.documentElement;

  function resolve() {
    try {
      var s = localStorage.getItem(KEY);
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

  function apply(t) {
    root.dataset.theme = t;
  }

  // Runs synchronously during <head> parse — sets data-theme before first paint.
  apply(resolve());

  document.addEventListener("DOMContentLoaded", function () {
    var btn = document.getElementById("theme-toggle");
    if (!btn) return;

    function sync() {
      var dark = root.dataset.theme === "dark";
      btn.setAttribute("aria-pressed", String(dark));
      btn.textContent = dark ? "☀" : "☾"; // ☀ / ☾
      btn.title = dark ? "Switch to light theme" : "Switch to dark theme";
    }

    sync();

    btn.addEventListener("click", function () {
      var next = root.dataset.theme === "dark" ? "light" : "dark";
      apply(next);
      try {
        localStorage.setItem(KEY, next);
      } catch (e) {}
      sync();
    });
  });
})();
