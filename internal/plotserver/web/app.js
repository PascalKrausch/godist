async function loadPayload(bins) {
  const url = bins ? `/api/plot?bins=${bins}` : "/api/plot";
  const response = await fetch(url);

  if (!response.ok) {
    throw new Error(`API request failed: HTTP ${response.status}`);
  }

  return response.json();
}

// Obergrenze der Klassenanzahl, identisch mit plot.MaxBins im Backend.
const MAX_BINS = 500;

// uPlot zeichnet ohne explizite Farbe nichts sichtbar, daher gibt es eine feste Palette.
const PALETTE = ["#d62728", "#1f77b4", "#2ca02c", "#9467bd", "#ff7f0e", "#8c564b", "#e377c2"];

// Zustand des Dashboards: Diagramme werden einmal erzeugt und danach nur mit neuen Daten aktualisiert.
const state = {
  pdf: null,
  cdf: null,
  qq: [],
  summaryRows: [],
  descriptiveBuilt: false,
  hidden: new Set(),
  togglesBuilt: false,
};

function seriesOptions(curve, color) {
  const opts = {
    label: curve.name,
    stroke: color,
    width: 2,
    points: { show: false },
    show: !state.hidden.has(curve.name),
  };

  if (curve.stepped) {
    // Stufenfunktion: Der Wert eines Punktes gilt bis zum nächsten Punkt.
    opts.paths = uPlot.paths.stepped({ align: 1 });
  }
  if (curve.fill) opts.fill = curve.fill;
  if (curve.dash) opts.dash = curve.dash;
  if (curve.markers) {
    // Nur Punkte zeichnen, ohne verbindende Linie.
    opts.paths = () => null;
    opts.points = { show: true, size: 5, stroke: color, fill: color };
  }

  return opts;
}

function makeChart(element, x, curves, yLabel) {
  const data = [x, ...curves.map(curve => curve.y)];

  const series = [{}, ...curves.map((curve, i) => seriesOptions(curve, curve.color || PALETTE[i % PALETTE.length]))];

  const opts = {
    width: element.clientWidth || 800,
    height: 320,
    scales: {
      x: { time: false },
    },
    axes: [
      { label: "x" },
      { label: yLabel },
    ],
    series,
    legend: {
      show: true,
    },
    cursor: {
      drag: {
        x: true,
        y: false,
      },
    },
  };

  const chart = new uPlot(opts, data, element);

  // Breite an den Container anpassen, auch wenn sich das Layout später ändert.
  new ResizeObserver(() => {
    const width = element.clientWidth;
    if (width > 0 && width !== chart.width) {
      chart.setSize({ width, height: 320 });
    }
  }).observe(element);

  return chart;
}

function pdfCurves(payload) {
  return [
    {
      name: "Histogramm",
      y: payload.histogram.grid_density,
      color: "#4682b4",
      stepped: true,
      fill: "rgba(70, 130, 180, 0.35)",
    },
    ...payload.models.map(model => ({ name: model.name, y: model.pdf })),
  ];
}

function cdfCurves(payload) {
  return [
    { name: "Empirische CDF", y: payload.ecdf, stepped: true, color: "#000000" },
    ...payload.models.map(model => ({ name: model.name, y: model.cdf })),
  ];
}

function renderPDF(payload) {
  const curves = pdfCurves(payload);
  if (state.pdf) {
    state.pdf.setData([payload.grid, ...curves.map(curve => curve.y)]);
    return;
  }
  state.pdf = makeChart(document.querySelector("#pdf-chart"), payload.grid, curves, "Dichte");
}

function renderCDF(payload) {
  const curves = cdfCurves(payload);
  if (state.cdf) {
    state.cdf.setData([payload.grid, ...curves.map(curve => curve.y)]);
    return;
  }
  state.cdf = makeChart(document.querySelector("#cdf-chart"), payload.grid, curves, "F(x)");
}

function renderQQ(payload) {
  const container = document.querySelector("#qq-charts");

  if (state.qq.length === 0) {
    for (const model of payload.models) {
      const section = document.createElement("section");
      const heading = document.createElement("h3");
      const element = document.createElement("div");

      heading.textContent = model.name;
      section.append(heading, element);
      container.append(section);
      state.qq.push({ name: model.name, section, element, chart: null });
    }
  }

  payload.models.forEach((model, i) => {
    const entry = state.qq[i];
    entry.section.hidden = state.hidden.has(model.name);

    if (model.qq.x.length === 0) return;

    // Die Referenzlinie y = x zeigt die perfekte Übereinstimmung.
    const curves = [
      { name: "Empirische Quantile", y: model.qq.y, markers: true },
      { name: "y = x", y: model.qq.x, dash: [6, 4], color: "#555555" },
    ];
    if (entry.chart) {
      entry.chart.setData([model.qq.x, ...curves.map(curve => curve.y)]);
      return;
    }
    entry.chart = makeChart(entry.element, model.qq.x, curves, "Empirisches Quantil");
  });
}

// setModelVisible blendet ein Modell in allen Diagrammen ein oder aus, ohne die Diagramme neu zu erzeugen.
function setModelVisible(name, visible) {
  if (visible) {
    state.hidden.delete(name);
  } else {
    state.hidden.add(name);
  }

  for (const chart of [state.pdf, state.cdf]) {
    if (!chart) continue;
    const index = chart.series.findIndex(series => series.label === name);
    if (index > 0) chart.setSeries(index, { show: visible });
  }

  for (const entry of state.qq) {
    if (entry.name === name) entry.section.hidden = !visible;
  }

  for (const entry of state.summaryRows) {
    if (entry.name === name) entry.row.hidden = !visible;
  }
}

// formatNumber zeigt sechs signifikante Stellen; fehlende (null) Werte erscheinen als Gedankenstrich.
function formatNumber(value) {
  return value === null ? "–" : value.toPrecision(6);
}

// Parameter und Gütemaße ändern sich nicht mit der Klassenanzahl, daher wird die Tabelle nur einmal gefüllt.
function renderSummary(payload) {
  if (state.summaryRows.length > 0) return;

  const body = document.querySelector("#summary-body");
  for (const model of payload.models) {
    const row = document.createElement("tr");
    const params = model.params.map(p => `${p.name} = ${formatNumber(p.value)}`).join(", ");
    const cells = [
      model.name,
      params,
      formatNumber(model.metrics.aic),
      formatNumber(model.metrics.bic),
      formatNumber(model.metrics.ks),
      formatNumber(model.metrics.rmse),
    ];

    cells.forEach((text, i) => {
      const cell = document.createElement("td");
      cell.textContent = text;
      if (i >= 2) cell.className = "num";
      row.append(cell);
    });
    row.hidden = state.hidden.has(model.name);
    body.append(row);
    state.summaryRows.push({ name: model.name, row });
  }
}

function renderDescriptive(payload) {
  if (state.descriptiveBuilt) return;
  state.descriptiveBuilt = true;

  const d = payload.descriptive;
  const rows = [
    ["Mittelwert", formatNumber(d.mean)],
    ["Median", formatNumber(d.median)],
    ["Varianz", formatNumber(d.variance)],
    ["Standardabweichung", formatNumber(d.std_dev)],
    ["Minimum", formatNumber(d.min)],
    ["Q1 (25 %-Quantil)", formatNumber(d.q1)],
    ["Q3 (75 %-Quantil)", formatNumber(d.q3)],
    ["Maximum", formatNumber(d.max)],
    ["Schiefe", formatNumber(d.skewness)],
    ["Kurtosis (Excess)", formatNumber(d.kurtosis)],
  ];

  const body = document.querySelector("#descriptive-body");
  for (const [label, text] of rows) {
    const row = document.createElement("tr");
    const name = document.createElement("td");
    name.textContent = label;
    const value = document.createElement("td");
    value.textContent = text;
    value.className = "num";
    row.append(name, value);
    body.append(row);
  }
}

function renderModelToggles(payload) {
  if (state.togglesBuilt) return;
  state.togglesBuilt = true;

  const container = document.querySelector("#model-toggles");
  for (const model of payload.models) {
    const label = document.createElement("label");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = true;
    input.addEventListener("change", () => setModelVisible(model.name, input.checked));
    label.append(input, model.name);
    container.append(label);
  }
}

// Die Klassenanzahl wird vom Server berechnet, daher zeigt das Eingabefeld die tatsächlich verwendete Anzahl.
function renderBinsControls(payload, bins) {
  document.querySelector("#bins-input").value = payload.histogram.edges.length - 1;
  document.querySelector("#bins-info").textContent = bins
    ? "manuell"
    : "automatisch (Freedman-Diaconis)";
}

let latestRequest = 0;

async function update(bins) {
  // Nur die jeweils neueste Anfrage darf das Dashboard aktualisieren.
  const request = ++latestRequest;
  const payload = await loadPayload(bins);
  if (request !== latestRequest) return;

  document.querySelector("#sample-size").textContent = `n = ${payload.sample_size}`;

  if (!payload.grid) {
    document.querySelector("#error").textContent = "Keine Daten zum Darstellen.";
    return;
  }

  renderModelToggles(payload);
  renderSummary(payload);
  renderDescriptive(payload);
  renderPDF(payload);
  renderCDF(payload);
  renderQQ(payload);
  renderBinsControls(payload, bins);
}

function reportError(error) {
  document.querySelector("#error").textContent = error.message;
  console.error(error);
}

async function main() {
  const binsInput = document.querySelector("#bins-input");

  binsInput.addEventListener("change", () => {
    const bins = Number(binsInput.value);
    if (!Number.isInteger(bins) || bins < 1 || bins > MAX_BINS) {
      document.querySelector("#error").textContent =
        `Die Klassenanzahl muss zwischen 1 und ${MAX_BINS} liegen.`;
      return;
    }
    document.querySelector("#error").textContent = "";
    update(bins).catch(reportError);
  });

  document.querySelector("#bins-auto").addEventListener("click", () => {
    document.querySelector("#error").textContent = "";
    update(null).catch(reportError);
  });

  try {
    await update(null);
  } catch (error) {
    reportError(error);
  }
}

main();
