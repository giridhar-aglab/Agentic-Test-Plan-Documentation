package render

// planStylesheet styles the report for reading rather than for decoration. The
// document is long and the reader is scanning for the rows that matter, so the
// work goes into table legibility, badge contrast and a table of contents that
// tracks position — not into ornament.
const planStylesheet = `
:root {
  --bg: #fbfaf9;
  --surface: #ffffff;
  --border: #e4e0da;
  --text: #23201d;
  --muted: #6b645c;
  --accent: #3d5a80;
  --code-bg: #f3f1ee;
  --p0: #a4243b;  --p0-bg: #fbe9ec;
  --p1: #99632c;  --p1-bg: #fdf0e0;
  --p2: #3d5a80;  --p2-bg: #e9eff6;
  --p3: #5a6b5d;  --p3-bg: #eaefea;
  color-scheme: light dark;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #191715; --surface: #211f1d; --border: #383430;
    --text: #eae6e1; --muted: #a09990; --accent: #8fb0d6; --code-bg: #2a2724;
    --p0: #f2a2ae; --p0-bg: #3d2027;
    --p1: #e8c08a; --p1-bg: #3a2d1c;
    --p2: #a9c4e4; --p2-bg: #1f2c3c;
    --p3: #b3c4b5; --p3-bg: #232c24;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0; background: var(--bg); color: var(--text);
  font: 16px/1.65 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}
main {
  max-width: 60rem; margin: 0 auto; padding: 3rem 1.5rem 6rem;
}
h1 {
  font-size: 2rem; line-height: 1.2; margin: 0 0 1.5rem;
  letter-spacing: -0.02em;
}
h2 {
  font-size: 1.35rem; margin: 3rem 0 1rem; padding-top: 1.25rem;
  border-top: 1px solid var(--border); letter-spacing: -0.01em;
}
h3 { font-size: 1.1rem; margin: 2rem 0 .6rem; }
/* H4 carries scenario titles, which are content — never shout them. */
h4 { font-size: 1rem; margin: 1.75rem 0 .4rem; font-weight: 650; }
h4 code { font-size: .85em; font-weight: 500; }
p { margin: 0 0 1rem; }
ul, ol { margin: 0 0 1rem; padding-left: 1.4rem; }
li { margin: .3rem 0; }
a { color: var(--accent); text-decoration-thickness: 1px; text-underline-offset: 2px; }
hr { border: 0; border-top: 1px solid var(--border); margin: 2.5rem 0; }
code {
  background: var(--code-bg); padding: .12em .38em; border-radius: 4px;
  font: .875em/1.4 ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
  word-break: break-word;
}
pre {
  background: var(--code-bg); padding: 1rem; border-radius: 8px;
  overflow-x: auto; border: 1px solid var(--border);
}
pre code { background: none; padding: 0; }
blockquote {
  margin: 1.25rem 0; padding: .75rem 1.1rem; border-left: 3px solid var(--accent);
  background: var(--surface); border-radius: 0 6px 6px 0; color: var(--muted);
}
blockquote p:last-child { margin-bottom: 0; }

/* Wide tables scroll inside their own box; the page never scrolls sideways. */
.table-wrap { overflow-x: auto; margin: 0 0 1.5rem; }
table {
  border-collapse: collapse; width: 100%; font-size: .9rem;
  background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
}
th, td { text-align: left; padding: .55rem .7rem; border-bottom: 1px solid var(--border); }
th {
  font-size: .75rem; text-transform: uppercase; letter-spacing: .05em;
  color: var(--muted); font-weight: 600; white-space: nowrap;
  position: sticky; top: 0; background: var(--surface);
}
tbody tr:last-child td { border-bottom: 0; }
tbody tr:hover { background: var(--code-bg); }
td code { white-space: nowrap; }

.badge {
  display: inline-block; padding: .1rem .45rem; border-radius: 4px;
  font-size: .75rem; font-weight: 650; letter-spacing: .02em; white-space: nowrap;
}
.badge-p0, .badge-critical { color: var(--p0); background: var(--p0-bg); }
.badge-p1, .badge-high     { color: var(--p1); background: var(--p1-bg); }
.badge-p2, .badge-medium   { color: var(--p2); background: var(--p2-bg); }
.badge-p3, .badge-low      { color: var(--p3); background: var(--p3-bg); }

#toc {
  position: fixed; top: 0; right: 0; width: 15rem; height: 100vh;
  overflow-y: auto; padding: 3rem 1rem; border-left: 1px solid var(--border);
  background: var(--bg); font-size: .82rem;
}
#toc a { display: block; padding: .25rem 0; color: var(--muted); text-decoration: none; }
#toc a:hover { color: var(--text); }
#toc a.current { color: var(--accent); font-weight: 600; }
#toc .lvl3 { padding-left: .8rem; font-size: .78rem; }
@media (max-width: 1180px) { #toc { display: none; } }
@media (min-width: 1181px) { main { margin-right: 16rem; } }

@media print {
  #toc { display: none; }
  main { max-width: none; margin: 0; padding: 0; }
  h2 { page-break-after: avoid; }
  table, blockquote { page-break-inside: avoid; }
}
`

// planScript adds a table of contents built from the headings already in the
// document. Nothing here is required to read the report: with scripting off the
// page is the same document, minus the sidebar.
const planScript = `
(function () {
  var headings = document.querySelectorAll('main h2, main h3');
  if (headings.length < 3) return;

  var nav = document.createElement('nav');
  nav.id = 'toc';
  headings.forEach(function (heading) {
    var link = document.createElement('a');
    link.href = '#' + heading.id;
    link.textContent = heading.textContent;
    if (heading.tagName === 'H3') link.className = 'lvl3';
    nav.appendChild(link);
  });
  document.body.appendChild(nav);

  var links = nav.querySelectorAll('a');
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      if (!entry.isIntersecting) return;
      links.forEach(function (link) {
        link.classList.toggle('current', link.getAttribute('href') === '#' + entry.target.id);
      });
    });
  }, { rootMargin: '0px 0px -75% 0px' });
  headings.forEach(function (heading) { observer.observe(heading); });
})();
`
