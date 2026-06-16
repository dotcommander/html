/* headings.js — adds an unobtrusive anchor link to each heading with an id,
   for copying a direct same-page link. No dependencies. */

(function () {
  "use strict";

  document.addEventListener("DOMContentLoaded", function () {
    var headings = document.querySelectorAll(
      ".markdown-body h1[id], .markdown-body h2[id], .markdown-body h3[id]," +
        ".markdown-body h4[id], .markdown-body h5[id], .markdown-body h6[id]",
    );

    headings.forEach(function (h) {
      var a = document.createElement("a");
      a.className = "heading-anchor";
      a.href = "#" + h.id;
      a.textContent = "#";
      a.setAttribute("aria-label", "Link to this section");
      h.appendChild(a);
    });
  });
})();
