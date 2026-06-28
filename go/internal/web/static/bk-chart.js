// bk-chart — a thin custom element wrapping echarts.
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
// Stays deliberately tiny — no Lit, no shadow DOM, no framework. The full
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
      // Merge with sensible defaults — dark mode, no toolbox, sane fonts.
      const merged = Object.assign(
        {
          textStyle: { fontFamily: "ui-sans-serif, system-ui, -apple-system, sans-serif" },
          backgroundColor: "transparent",
        },
        option,
      );
      this._chart.setOption(merged, { notMerge: true });
    }
  }

  customElements.define("bk-chart", BKChart);
})();
