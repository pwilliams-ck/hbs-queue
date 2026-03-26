package main

import (
	"expvar"
	"html/template"
	"net/http"
	"net/http/pprof"
	"runtime/metrics"
)

func init() {
	// Publish runtime/metrics as expvar so /debug/vars includes
	// goroutine counts, GC stats, memory breakdown, and scheduler
	// metrics alongside any custom app counters.
	expvar.Publish("runtime", expvar.Func(func() any {
		descs := metrics.All()
		samples := make([]metrics.Sample, len(descs))
		for i, d := range descs {
			samples[i].Name = d.Name
		}
		metrics.Read(samples)

		out := make(map[string]any, len(samples))
		for _, s := range samples {
			switch s.Value.Kind() {
			case metrics.KindUint64:
				out[s.Name] = s.Value.Uint64()
			case metrics.KindFloat64:
				out[s.Name] = s.Value.Float64()
			case metrics.KindBad:
				// Metric not supported on this platform.
			default:
				// Skip histograms and other complex types for the
				// JSON endpoint — they are available via pprof.
			}
		}
		return out
	}))
}

// newDebugMux returns an isolated HTTP mux for the debug/profiling listener.
// Handlers are registered explicitly rather than using http.DefaultServeMux
// to prevent pprof routes from leaking onto the main API port.
func newDebugMux() *http.ServeMux {
	mux := http.NewServeMux()

	// pprof handlers — explicit registration instead of blank import.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// expvar handler.
	mux.Handle("/debug/vars", expvar.Handler())

	// Live dashboard.
	mux.Handle("/debug/dashboard", handleDebugDashboard())

	return mux
}

var dashboardTmpl = template.Must(template.New("dashboard").Parse(dashboardHTML))

// handleDebugDashboard serves a live HTML dashboard that auto-refreshes
// runtime metrics from /debug/vars. Zero cost when no one is viewing it.
func handleDebugDashboard() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = dashboardTmpl.Execute(w, nil)
	})
}

const dashboardHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>hbs-queue debug dashboard</title>
<style>
  body { font-family: monospace; background: #1a1a2e; color: #e0e0e0; margin: 2em; }
  h1 { color: #00d4ff; }
  table { border-collapse: collapse; width: 100%; max-width: 800px; margin-top: 1em; }
  th, td { text-align: left; padding: 6px 12px; border-bottom: 1px solid #333; }
  th { color: #00d4ff; }
  td.val { font-weight: bold; color: #7fff7f; }
  .status { margin-top: 1em; color: #888; }
  #error { color: #ff4444; }
</style>
</head>
<body>
<h1>hbs-queue debug</h1>
<div class="status">
  next refresh in <span id="countdown">10</span>s
  <span id="error"></span>
</div>

<h2>goroutines</h2>
<table id="goroutines"><tr><td>loading...</td></tr></table>

<h2>memory</h2>
<table id="memory"><tr><td>loading...</td></tr></table>

<h2>gc</h2>
<table id="gc"><tr><td>loading...</td></tr></table>

<h2>scheduler</h2>
<table id="scheduler"><tr><td>loading...</td></tr></table>

<h2>links</h2>
<ul>
  <li><a href="/debug/pprof/">pprof index</a></li>
  <li><a href="/debug/pprof/goroutine?debug=1">goroutine stacks</a></li>
  <li><a href="/debug/vars">expvar JSON</a></li>
</ul>

<script>
(function() {
  const params = new URLSearchParams(window.location.search);
  const intervalSec = parseInt(params.get('interval') || '10', 10);
  const interval = intervalSec * 1000;
  var remaining = intervalSec;
  const countdownEl = document.getElementById('countdown');

  setInterval(function() {
    remaining--;
    if (remaining < 0) remaining = intervalSec;
    countdownEl.textContent = remaining;
  }, 1000);

  function fmt(n) {
    if (typeof n !== 'number') return String(n);
    if (n > 1024*1024*1024) return (n / (1024*1024*1024)).toFixed(1) + ' GiB';
    if (n > 1024*1024) return (n / (1024*1024)).toFixed(1) + ' MiB';
    if (n > 1024) return (n / 1024).toFixed(1) + ' KiB';
    return String(n);
  }

  function renderTable(id, rows) {
    const el = document.getElementById(id);
    if (!rows.length) { el.innerHTML = '<tr><td>no data</td></tr>'; return; }
    el.innerHTML = rows.map(function(r) {
      return '<tr><td>' + r[0] + '</td><td class="val">' + r[1] + '</td></tr>';
    }).join('');
  }

  function refresh() {
    remaining = intervalSec;
    countdownEl.textContent = remaining;
    fetch('/debug/vars')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        document.getElementById('error').textContent = '';
        const rt = data.runtime || {};

        renderTable('goroutines', [
          ['/sched/goroutines/total:goroutines', fmt(rt['/sched/goroutines/total:goroutines'])],
          ['/sched/goroutines-created:goroutines', fmt(rt['/sched/goroutines-created:goroutines'])],
          ['/sched/threads:threads', fmt(rt['/sched/threads:threads'])],
        ]);

        renderTable('memory', [
          ['/memory/classes/heap/objects:bytes', fmt(rt['/memory/classes/heap/objects:bytes'])],
          ['/memory/classes/heap/stacks:bytes', fmt(rt['/memory/classes/heap/stacks:bytes'])],
          ['/memory/classes/heap/released:bytes', fmt(rt['/memory/classes/heap/released:bytes'])],
          ['/memory/classes/total:bytes', fmt(rt['/memory/classes/total:bytes'])],
        ]);

        renderTable('gc', [
          ['/gc/cycles/total:gc-cycles', fmt(rt['/gc/cycles/total:gc-cycles'])],
          ['/gc/gogc:percent', fmt(rt['/gc/gogc:percent'])],
          ['/gc/gomemlimit:bytes', fmt(rt['/gc/gomemlimit:bytes'])],
        ]);

        renderTable('scheduler', [
          ['/sched/gomaxprocs:threads', fmt(rt['/sched/gomaxprocs:threads'])],
          ['/sched/threads:threads', fmt(rt['/sched/threads:threads'])],
        ]);
      })
      .catch(function(err) {
        document.getElementById('error').textContent = ' error: ' + err;
      });
  }

  refresh();
  setInterval(refresh, interval);
})();
</script>
</body>
</html>`
