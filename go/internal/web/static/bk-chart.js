// bk-chart - a thin custom element wrapping echarts.
//
// Usage:
//   <bk-chart class="h-64 w-full">
//     <script type="application/json">{ ...echarts option... }</script>
//   </bk-chart>
//
// The element reads the first <script type="application/json"> child as an
// echarts option object, instantiates a chart, and re-renders on resize. It
// also redraws when its config is swapped (htmx fragment replacement) by
// listening to MutationObserver.
//
// Stays deliberately tiny - no Lit, no shadow DOM, no framework. The full
// expressiveness of echarts is reached by editing the JSON server-side.

(function () {
  if (customElements.get("bk-chart")) return;

  class BKChart extends HTMLElement {
    constructor() {
      super();
      this._chart = null;
      this._ro = null;
      this._mo = null;
    }

    connectedCallback() {
      // Mount echarts on a host div so the <script type=application/json>
      // child doesn't show up in the chart.
      this._host = document.createElement("div");
      this._host.style.width = "100%";
      this._host.style.height = "100%";
      this.appendChild(this._host);

      this._chart = window.echarts && window.echarts.init(this._host, "dark", {
        renderer: this.getAttribute("renderer") || "svg",
      });
      if (!this._chart) {
        this._host.textContent = "echarts not loaded";
        return;
      }

      this._apply();

      this._ro = new ResizeObserver(() => this._chart && this._chart.resize());
      this._ro.observe(this);

      // If a parent swaps our config via htmx, redraw.
      this._mo = new MutationObserver(() => this._apply());
      this._mo.observe(this, { attributes: true, attributeFilter: ["data-config"] });
    }

    disconnectedCallback() {
      if (this._ro) this._ro.disconnect();
      if (this._mo) this._mo.disconnect();
      if (this._chart) this._chart.dispose();
      this._chart = null;
    }

    _apply() {
      if (!this._chart) return;
      const raw = this.dataset.config || "";
      if (!raw) return;
      let option;
      try {
        option = JSON.parse(raw);
      } catch (e) {
        this._host.textContent = "invalid chart config";
        return;
      }
      hydrateFormatters(option);
      const onClick = pickClickHandler(option);
      // Merge with sensible defaults - dark mode, no toolbox, sane fonts.
      const merged = Object.assign(
        {
          textStyle: { fontFamily: "ui-sans-serif, system-ui, -apple-system, sans-serif" },
          backgroundColor: "transparent",
        },
        option,
      );
      this._chart.setOption(merged, { notMerge: true });
      this._chart.off("click");
      if (onClick) {
        this._chart.on("click", onClick);
        this._host.style.cursor = "pointer";
      } else {
        this._host.style.cursor = "";
      }
    }
  }

  function hydrateFormatters(option) {
    if (!option || !option.tooltip) return;
    if (option.bkTooltip === "characterPerf") {
      option.tooltip.formatter = characterPerfTooltip;
      delete option.bkTooltip;
    } else if (option.bkTooltip === "bossBoxplot") {
      option.tooltip.formatter = bossBoxplotTooltip;
      delete option.bkTooltip;
    }
  }

  function pickClickHandler(option) {
    if (!option) return null;
    const name = option.bkOnClick;
    delete option.bkOnClick;
    if (name === "openDetailUrl") return openDetailUrlClick;
    return null;
  }

  function openDetailUrlClick(params) {
    const url = params && params.data && params.data.detailUrl;
    if (!url) return;
    window.location.href = url;
  }

  function characterPerfTooltip(params) {
    const points = Array.isArray(params) ? params : [params];
    if (!points.length) return "";

    const firstPoint = points[0];
    const firstData = firstPoint.data || {};
    const firstValue = Array.isArray(firstData.value) ? firstData.value : firstPoint.value;
    const rows = [
      `<div class="font-medium">${escapeHTML(formatDate(Array.isArray(firstValue) ? firstValue[0] : firstPoint.axisValue))}</div>`,
    ];

    for (const point of points) {
      const data = point.data || {};
      const value = Array.isArray(data.value) ? data.value : point.value;
      const metric = Array.isArray(value) ? value[1] : value;
      rows.push(
        `<div>${point.marker || ""}${escapeHTML(point.seriesName)}: ${escapeHTML(formatNumber(metric))}</div>`,
      );
    }

    if (firstData.avgItemLvl !== undefined && firstData.avgItemLvl !== null) {
      rows.push(`<div>iLvl: ${escapeHTML(formatNumber(firstData.avgItemLvl))}</div>`);
    }
    if (firstData.detailUrl) {
      rows.push(`<div class="text-xs opacity-70">click on the marker for the detail</div>`);
    }
    return rows.join("");
  }

  function bossBoxplotTooltip(params) {
    const point = Array.isArray(params) ? params[0] : params;
    if (!point) return "";

    const data = point.data || {};
    const stats = boxplotStats(data, point.value);
    const metric = data.metric || point.seriesName || "";
    const specLabel = data.specLabel || data.name || point.name || "Unknown";
    const icons = [];
    if (data.classIcon) {
      icons.push(
        `<img src="${escapeAttribute(data.classIcon)}" alt="" width="18" height="18" style="border-radius: 3px;">`,
      );
    }
    if (data.specIcon) {
      icons.push(
        `<img src="${escapeAttribute(data.specIcon)}" alt="" width="18" height="18" style="border-radius: 3px;">`,
      );
    }

    return [
      `<div style="min-width: 12rem;">`,
      `<div style="margin-bottom: 0.35rem; font-weight: 600;">${escapeHTML(metric)}</div>`,
      `<div style="display: flex; align-items: center; gap: 0.35rem; margin-bottom: 0.45rem; font-weight: 600;">${icons.join("")}<span>${escapeHTML(specLabel)}</span></div>`,
      `<div style="display: grid; grid-template-columns: max-content max-content; gap: 0.2rem 1rem;">`,
      boxplotStatRow("min", stats.min),
      boxplotStatRow("Q1", stats.q1),
      boxplotStatRow("median", stats.median, true),
      boxplotStatRow("Q3", stats.q3),
      boxplotStatRow("max", stats.max),
      `</div>`,
      `</div>`,
    ].join("");
  }

  function boxplotStats(data, value) {
    if (data && data.stats) return data.stats;
    const values = Array.isArray(value) ? value : [];
    const offset = values.length >= 6 ? 1 : 0;
    return {
      min: values[offset],
      q1: values[offset + 1],
      median: values[offset + 2],
      q3: values[offset + 3],
      max: values[offset + 4],
    };
  }

  function boxplotStatRow(label, value, strong) {
    const weight = strong ? "font-weight: 700;" : "";
    return `<span>${escapeHTML(label)}</span><span style="text-align: right; ${weight}">${escapeHTML(formatNumber(value))}</span>`;
  }

  function formatDate(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value == null ? "" : String(value);
    return date.toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function formatNumber(value, maximumFractionDigits) {
    const number = Number(value);
    if (!Number.isFinite(number)) return value == null ? "" : String(value);
    return number.toLocaleString(undefined, { maximumFractionDigits: maximumFractionDigits || 0 });
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, (char) => {
      switch (char) {
        case "&":
          return "&amp;";
        case "<":
          return "&lt;";
        case ">":
          return "&gt;";
        case '"':
          return "&quot;";
        default:
          return "&#39;";
      }
    });
  }

  function escapeAttribute(value) {
    return escapeHTML(value).replace(/`/g, "&#96;");
  }

  customElements.define("bk-chart", BKChart);
})();
