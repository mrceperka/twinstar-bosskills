// bk-ui.js - small DOM wiring for declarative UI patterns.
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
//
//   .bk-table-wrap table
//     → first-column cells are capped in CSS. This script marks truncated
//       first-column cells and shows their full text on hover/focus/tap.
//
//   <table data-sortable>
//     <thead><tr><th>Name</th><th>DPS</th><th data-no-sort>Details</th></tr></thead>
//     <tbody><tr><td>Foo</td><td data-sort="1234">1,234</td><td>...</td></tr></tbody>
//   </table>
//     → every <th> becomes a click/keyboard sort toggle. Cell values come
//       from td[data-sort] when present and from the cell text otherwise -
//       formatted numbers MUST carry data-sort with the raw value, because
//       locale thousand separators ("1.234" in de) are not parseable. Mark
//       headers that must not sort with th[data-no-sort].
//
//   [data-mobile-nav]
//     → fixed mobile nav hides on downward scroll and reappears on upward
//       scroll, page edges, keyboard navigation, and resize.

(function () {

  var tableTip;
  var tableTipOwner;

  function getTableTip() {
    if (tableTip) return tableTip;
    tableTip = document.createElement('div');
    tableTip.className = 'bk-table-fulltext-tip hidden';
    tableTip.setAttribute('role', 'tooltip');
    document.body.appendChild(tableTip);
    return tableTip;
  }

  function cellText(cell) {
    return (cell.textContent || '').replace(/\s+/g, ' ').trim();
  }

  function isTruncated(cell) {
    return cell.scrollWidth > cell.clientWidth + 1 || cell.scrollHeight > cell.clientHeight + 1;
  }

  function placeTableTip(cell) {
    var tip = getTableTip();
    var rect = cell.getBoundingClientRect();
    tip.style.left = '0px';
    tip.style.top = '0px';
    tip.classList.remove('hidden');

    var tipRect = tip.getBoundingClientRect();
    var left = Math.min(Math.max(rect.left, 8), window.innerWidth - tipRect.width - 8);
    var top = rect.bottom + 6;
    if (top + tipRect.height > window.innerHeight - 8) {
      top = rect.top - tipRect.height - 6;
    }
    if (top < 8) top = 8;
    tip.style.left = left + 'px';
    tip.style.top = top + 'px';
  }

  function showTableFullText(cell) {
    if (!cell || !cell.classList.contains('bk-table-truncated')) return;
    var text = cell.dataset.bkFullText || cellText(cell);
    if (!text) return;
    var tip = getTableTip();
    tip.textContent = text;
    tableTipOwner = cell;
    placeTableTip(cell);
  }

  function hideTableFullText(cell) {
    if (cell && tableTipOwner && cell !== tableTipOwner) return;
    tableTipOwner = null;
    if (tableTip) tableTip.classList.add('hidden');
  }

  function initStickyTableCells(root) {
    root.querySelectorAll('.bk-table-wrap tbody td:first-child, .bk-table-wrap thead th:first-child').forEach(function (cell) {
      var text = cellText(cell);
      if (!text) return;
      cell.dataset.bkFullText = text;
      cell.setAttribute('title', text);
      if (isTruncated(cell)) {
        cell.classList.add('bk-table-truncated');
        if (!cell.hasAttribute('tabindex')) cell.setAttribute('tabindex', '0');
        cell.setAttribute('aria-label', text);
      } else {
        cell.classList.remove('bk-table-truncated');
        if (cell.getAttribute('tabindex') === '0') cell.removeAttribute('tabindex');
        cell.removeAttribute('aria-label');
      }
      if (cell._bkTableTipBound) return;
      cell._bkTableTipBound = true;
      cell.addEventListener('mouseenter', function () { showTableFullText(cell); });
      cell.addEventListener('mouseleave', function () { hideTableFullText(cell); });
      cell.addEventListener('focusin', function () { showTableFullText(cell); });
      cell.addEventListener('focusout', function () { hideTableFullText(cell); });
      cell.addEventListener('click', function (e) {
        if (!cell.classList.contains('bk-table-truncated')) return;
        if (tableTipOwner === cell) {
          hideTableFullText(cell);
          return;
        }
        showTableFullText(cell);
        e.stopPropagation();
      });
    });
  }

  // The .bk-tip currently shown/hovered, tracked so scroll/resize can keep it
  // anchored to its row (position: fixed does not scroll with the page).
  var activeTipRow;

  function closeAllTooltips() {
    document.querySelectorAll('.bk-tip-row.bk-tip-open').forEach(function (r) {
      r.classList.remove('bk-tip-open');
    });
    activeTipRow = null;
  }

  // placeItemTip anchors row's .bk-tip below the row, flipping above when it
  // would overflow the viewport bottom, and clamps it horizontally. The tip is
  // position: fixed and kept in layout while hidden, so it is measurable here.
  function placeItemTip(row) {
    if (!row) return;
    var tip = row.querySelector('.bk-tip');
    if (!tip) return;
    var rect = row.getBoundingClientRect();
    var tr = tip.getBoundingClientRect();
    var left = Math.min(Math.max(rect.left, 8), window.innerWidth - tr.width - 8);
    if (left < 8) left = 8;
    var top = rect.bottom + 6;
    if (top + tr.height > window.innerHeight - 8) {
      top = rect.top - tr.height - 6;
    }
    if (top < 8) top = 8;
    tip.style.left = left + 'px';
    tip.style.top = top + 'px';
  }

  function loadTooltip(row) {
    activeTipRow = row;
    placeItemTip(row);
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
        // Content changes size; re-anchor if this row is still the active one.
        if (activeTipRow === row) placeItemTip(row);
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
      row.addEventListener('mouseleave', function () {
        if (activeTipRow === row && !row.classList.contains('bk-tip-open')) {
          activeTipRow = null;
        }
      });
      // Toggle open/close on tap (touch devices); load at same time.
      row.addEventListener('click', function (e) {
        // Let link clicks navigate; tooltip tap targets are icon/row background.
        if (e.target.closest('a')) return;
        loadTooltip(row);
        var wasOpen = row.classList.contains('bk-tip-open');
        closeAllTooltips();
        if (!wasOpen) {
          row.classList.add('bk-tip-open');
          activeTipRow = row;
          placeItemTip(row);
          e.stopPropagation();
        }
      });
    });
  }

  function initMobileNav() {
    var nav = document.querySelector('[data-mobile-nav]');
    if (!nav || nav._bkMobileNavBound) return;
    nav._bkMobileNavBound = true;

    var lastY = window.scrollY || window.pageYOffset || 0;
    var ticking = false;
    var mobileQuery = window.matchMedia('(max-width: 640px)');

    function setHidden(hidden) {
      nav.classList.toggle('bk-mobile-nav-hidden', hidden);
    }

    function isNearBottom(y) {
      var doc = document.documentElement;
      var height = Math.max(doc.scrollHeight, document.body.scrollHeight);
      return y + window.innerHeight >= height - 24;
    }

    function update() {
      ticking = false;
      if (!mobileQuery.matches) {
        setHidden(false);
        lastY = window.scrollY || window.pageYOffset || 0;
        return;
      }

      var y = Math.max(0, window.scrollY || window.pageYOffset || 0);
      var delta = y - lastY;
      if (y <= 24 || isNearBottom(y)) {
        setHidden(false);
      } else if (delta > 8 && y > 120) {
        setHidden(true);
      } else if (delta < -8) {
        setHidden(false);
      }
      lastY = y;
    }

    function requestUpdate() {
      if (ticking) return;
      ticking = true;
      window.requestAnimationFrame(update);
    }

    window.addEventListener('scroll', requestUpdate, { passive: true });
    window.addEventListener('resize', function () {
      setHidden(false);
      lastY = window.scrollY || window.pageYOffset || 0;
    });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Tab') setHidden(false);
    });
    nav.addEventListener('focusin', function () { setHidden(false); });
    nav.addEventListener('touchstart', function () { setHidden(false); }, { passive: true });
  }

  // Close tooltips when clicking outside any .bk-tip-row.
  document.addEventListener('click', function (e) {
    if (!e.target.closest('.bk-tip-row')) {
      closeAllTooltips();
    }
    if (!e.target.closest('.bk-table-truncated')) {
      hideTableFullText();
    }
  });
  window.addEventListener('scroll', function () {
    hideTableFullText();
    if (activeTipRow) placeItemTip(activeTipRow);
  }, true);
  window.addEventListener('resize', function () {
    if (activeTipRow) placeItemTip(activeTipRow);
  });
  window.addEventListener('resize', function () {
    hideTableFullText();
    initStickyTableCells(document);
  });

  // ---- sortable tables ---------------------------------------------------

  function sortCellValue(row, index) {
    var cell = row.cells[index];
    if (!cell) return '';
    var raw = cell.getAttribute('data-sort');
    return raw !== null ? raw : cellText(cell);
  }

  // A column counts as numeric only when every non-empty value parses as a
  // number; one stray label ("3 days ago") makes the whole column textual.
  function isNumericColumn(rows, index) {
    var seen = false;
    for (var i = 0; i < rows.length; i++) {
      var v = sortCellValue(rows[i], index);
      if (v === '' || v === '-') continue;
      if (isNaN(Number(v))) return false;
      seen = true;
    }
    return seen;
  }

  function sortTableRows(tbody, rows, index, numeric, dir) {
    var sign = dir === 'asc' ? 1 : -1;
    // Array.prototype.sort is stable, so equal keys keep the server's order.
    rows.slice().sort(function (a, b) {
      var av = sortCellValue(a, index);
      var bv = sortCellValue(b, index);
      if (numeric) return sign * ((Number(av) || 0) - (Number(bv) || 0));
      return sign * av.localeCompare(bv);
    }).forEach(function (row) {
      tbody.appendChild(row);
    });
  }

  function markSortedHeader(headRow, active, dir) {
    Array.prototype.forEach.call(headRow.cells, function (cell) {
      var indicator = cell.querySelector('.bk-sort-indicator');
      if (cell !== active) {
        cell.removeAttribute('aria-sort');
        if (indicator) indicator.remove();
        return;
      }
      cell.setAttribute('aria-sort', dir === 'asc' ? 'ascending' : 'descending');
      if (!indicator) {
        indicator = document.createElement('span');
        indicator.className = 'bk-sort-indicator';
        cell.appendChild(indicator);
      }
      indicator.textContent = dir === 'asc' ? ' \u2191' : ' \u2193';
    });
  }

  function initSortableTables(root) {
    root.querySelectorAll('table[data-sortable]').forEach(function (table) {
      if (table._bkSortBound) return;
      table._bkSortBound = true;
      var headRow = table.tHead && table.tHead.rows[0];
      var tbody = table.tBodies[0];
      if (!headRow || !tbody) return;

      Array.prototype.forEach.call(headRow.cells, function (th, index) {
        if (th.hasAttribute('data-no-sort')) return;
        th.classList.add('bk-sortable-th');
        th.setAttribute('role', 'button');
        th.setAttribute('tabindex', '0');

        function activate() {
          var rows = Array.prototype.slice.call(tbody.rows);
          if (rows.length < 2) return;
          var numeric = isNumericColumn(rows, index);
          var current = th.getAttribute('aria-sort');
          var dir;
          if (current === 'ascending') {
            dir = 'desc';
          } else if (current === 'descending') {
            dir = 'asc';
          } else {
            // First click: leaderboards want the biggest number on top,
            // name columns want A-Z.
            dir = numeric ? 'desc' : 'asc';
          }
          sortTableRows(tbody, rows, index, numeric, dir);
          markSortedHeader(headRow, th, dir);
        }

        th.addEventListener('click', function () { activate(); });
        th.addEventListener('keydown', function (e) {
          if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
            e.preventDefault();
            activate();
          }
        });
      });
    });
  }

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
    initSortableTables(root);
    initStickyTableCells(root);
    initMobileNav();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { init(document); });
  } else {
    init(document);
  }

  // Re-bind after htmx swaps in new content.
  document.addEventListener('htmx:afterSwap', function (e) { init(e.target || document); });
})();
