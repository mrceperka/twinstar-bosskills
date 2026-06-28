// bk-ui.js — small DOM wiring for declarative UI patterns.
//
// CSP blocks inline `onchange="..."` handlers, so we use `data-*` attributes
// here instead. Add the relevant attribute to an element in templ; this
// script attaches the listener.
//
// Supported patterns:
//
//   <select data-navigate>
//     <option value="/path/somewhere">Label</option>
//   </select>
//     → on change, navigates to the selected option's value (which must be
//       a full path/URL).
//
//   <select data-form-submit>
//     ...
//   </select>
//     → on change, submits the enclosing <form>. Use this when the option
//       values are query-parameter values and the form has method=GET.
//
//   <li class="bk-tip-row">
//     <img .../>
//     <a href="...">Item name</a>
//     <span class="bk-tip">tooltip HTML</span>
//   </li>
//     → CSS :hover shows the tooltip on desktop. This script adds a
//       click/tap toggle (class bk-tip-open) so the tooltip works on touch
//       devices too. Clicking a link inside the row navigates normally.

(function () {
  function closeAllTooltips() {
    document.querySelectorAll('.bk-tip-row.bk-tip-open').forEach(function (r) {
      r.classList.remove('bk-tip-open');
    });
  }

  function loadTooltip(row) {
    var tip = row.querySelector('.bk-tip');
    if (!tip || tip._bkLoaded) return;
    var tooltipUrl = row.dataset.tooltipUrl;
    if (!tooltipUrl) return;
    if (tip.childNodes.length > 0) {
      // Already populated server-side; nothing to fetch.
      tip._bkLoaded = true;
      return;
    }
    tip._bkLoaded = true; // mark before fetch to prevent duplicate requests
    fetch(tooltipUrl)
      .then(function (res) {
        if (!res.ok) throw new Error('tooltip fetch ' + res.status);
        return res.text();
      })
      .then(function (html) {
        tip.innerHTML = html;
      })
      .catch(function () {
        tip._bkLoaded = false; // allow retry on next hover
      });
  }

  function initTooltips(root) {
    root.querySelectorAll('.bk-tip-row').forEach(function (row) {
      if (row._bkTipBound) return;
      row._bkTipBound = true;
      // Load on first hover (desktop) or first focus-within (keyboard nav).
      row.addEventListener('mouseenter', function () { loadTooltip(row); });
      row.addEventListener('focusin', function () { loadTooltip(row); });
      // Toggle open/close on tap (touch devices); load at same time.
      row.addEventListener('click', function (e) {
        // Let link clicks navigate; tooltip tap targets are icon/row background.
        if (e.target.closest('a')) return;
        loadTooltip(row);
        var wasOpen = row.classList.contains('bk-tip-open');
        closeAllTooltips();
        if (!wasOpen) {
          row.classList.add('bk-tip-open');
          e.stopPropagation();
        }
      });
    });
  }

  // Close tooltips when clicking outside any .bk-tip-row.
  document.addEventListener('click', function (e) {
    if (!e.target.closest('.bk-tip-row')) {
      closeAllTooltips();
    }
  });

  function init(root) {
    root.querySelectorAll('select[data-navigate]').forEach(function (el) {
      if (el._bkBound) return;
      el._bkBound = true;
      el.addEventListener('change', function () {
        var v = el.value;
        if (v) window.location.href = v;
      });
    });
    root.querySelectorAll('select[data-form-submit]').forEach(function (el) {
      if (el._bkBound) return;
      el._bkBound = true;
      el.addEventListener('change', function () {
        if (el.form) el.form.submit();
      });
    });
    initTooltips(root);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { init(document); });
  } else {
    init(document);
  }

  // Re-bind after htmx swaps in new content.
  document.addEventListener('htmx:afterSwap', function (e) { init(e.target || document); });
})();
