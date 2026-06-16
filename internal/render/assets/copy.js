/* copy.js — adds a Copy button to every <pre> block, no dependencies */

(function () {
  "use strict";

  document.addEventListener("DOMContentLoaded", function () {
    var pres = document.querySelectorAll("pre");

    pres.forEach(function (pre) {
      var btn = document.createElement("button");
      btn.className = "copy-btn";
      btn.textContent = "Copy";
      btn.setAttribute("aria-label", "Copy code to clipboard");
      pre.appendChild(btn);

      btn.addEventListener("click", function () {
        // Read from the <code> child, not the whole pre (avoids including "Copy")
        var code = pre.querySelector("code");
        var text = code ? code.textContent : pre.textContent;

        // Remove a trailing newline that some renderers inject before the button
        if (text.endsWith("\n")) {
          text = text.slice(0, -1);
        }

        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(
            function () {
              markCopied(btn);
            },
            function () {
              fallbackCopy(text, btn);
            },
          );
        } else {
          fallbackCopy(text, btn);
        }
      });
    });

    function markCopied(btn) {
      btn.textContent = "Copied";
      btn.classList.add("copied");
      setTimeout(function () {
        btn.textContent = "Copy";
        btn.classList.remove("copied");
      }, 1500);
    }

    function fallbackCopy(text, btn) {
      // execCommand fallback for environments without clipboard API
      try {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        var ok = document.execCommand("copy");
        document.body.removeChild(ta);
        if (ok) {
          markCopied(btn);
        }
      } catch (_) {
        // No-op: clipboard unavailable, button stays silent
      }
    }
  });
})();
