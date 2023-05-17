package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alecthomas/units"

	"github.com/Duncaen/ccache_exporter/ccache"
)

const (
	namespace = "ccache"
)

var (
	addr      = flag.String("listen-address", ":9508", "The address to listen on for HTTP requests.")
	ccacheDir = flag.String("ccache-dir", "", "Path to the ccache directory.")
	ccacheSize units.Base2Bytes = 0
)

var (
	parsingErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "collector",
			Name:      "parsing_errors_total",
			Help:      "Collector parsing errors (total)",
		},
	)
)

type collector struct {
	ccacheDir string
	ccacheSize units.MetricBytes

	call                     *prometheus.Desc
	callCachable             *prometheus.Desc
	callHit                  *prometheus.Desc
	cacheHitRatio            *prometheus.Desc
	calledForLink            *prometheus.Desc
	calledForPreprocessing   *prometheus.Desc
	compilationFailed        *prometheus.Desc
	preprocessingFailed      *prometheus.Desc
	unsupportedCodeDirective *prometheus.Desc
	noInputFile              *prometheus.Desc
	cleanupsPerformed        *prometheus.Desc
	filesInCache             *prometheus.Desc
	cacheSizeBytes           *prometheus.Desc
	maxCacheSizeBytes        *prometheus.Desc
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.call
	ch <- c.callHit
	ch <- c.cacheHitRatio
	ch <- c.calledForLink
	ch <- c.calledForPreprocessing
	ch <- c.compilationFailed
	ch <- c.preprocessingFailed
	ch <- c.unsupportedCodeDirective
	ch <- c.noInputFile
	ch <- c.cleanupsPerformed
	ch <- c.filesInCache
	ch <- c.cacheSizeBytes
	ch <- c.maxCacheSizeBytes
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	counters := ccache.Counters{}
	if err := counters.ReadAll(c.ccacheDir); err != nil {
		parsingErrors.Inc()
		return
	}
	// counters
	miss := counters[ccache.CacheMiss]
	hits := counters[ccache.DirectCacheHit] + counters[ccache.PreprocessedCacheHit]
	uncacheable := counters[ccache.AutoconfTest] +
		counters[ccache.BadCompilerArguments] +
		counters[ccache.CalledForLink] +
		counters[ccache.CalledForPreprocessing] +
		counters[ccache.CompileFailed] +
		counters[ccache.CompilerProducedNoOutput] +
		counters[ccache.CompilerProducedEmptyOutput] +
		counters[ccache.CouldNotUseModules]+
		counters[ccache.CouldNotUsePrecompiledHeader] +
		counters[ccache.Disabled] +
		counters[ccache.MultipleSourceFiles] +
		counters[ccache.NoInputFile] +
		counters[ccache.OutputToStdout] +
		counters[ccache.PreprocessorError] +
		counters[ccache.Recache] +
		counters[ccache.UnsupportedCodeDirective] +
		counters[ccache.UnsupportedCompilerOption] +
		counters[ccache.UnsupportedEnvironmentVariable] +
		counters[ccache.UnsupportedSourceLanguage]
	errors := counters[ccache.BadOutputFile] +
		counters[ccache.CompilerCheckFailed] +
		counters[ccache.CouldNotFindCompiler] +
		counters[ccache.ErrorHashingExtraFile] +
		counters[ccache.InternalError] +
		counters[ccache.MissingCacheFile]
	cachable := miss + hits
	call := cachable + uncacheable + errors
	ch <- prometheus.MustNewConstMetric(c.call, prometheus.CounterValue, float64(call))
	ch <- prometheus.MustNewConstMetric(c.callCachable, prometheus.CounterValue, float64(cachable))
	ch <- prometheus.MustNewConstMetric(c.callHit, prometheus.CounterValue, float64(counters[ccache.DirectCacheHit]), "direct")
	ch <- prometheus.MustNewConstMetric(c.callHit, prometheus.CounterValue, float64(counters[ccache.PreprocessedCacheHit]), "preprocessed")
	ch <- prometheus.MustNewConstMetric(c.calledForLink, prometheus.CounterValue, float64(counters[ccache.CalledForLink]))
	ch <- prometheus.MustNewConstMetric(c.calledForPreprocessing, prometheus.CounterValue, float64(counters[ccache.CalledForPreprocessing]))
	ch <- prometheus.MustNewConstMetric(c.compilationFailed, prometheus.CounterValue, float64(counters[ccache.CompileFailed]))
	ch <- prometheus.MustNewConstMetric(c.preprocessingFailed, prometheus.CounterValue, float64(counters[ccache.PreprocessorError]))
	ch <- prometheus.MustNewConstMetric(c.unsupportedCodeDirective, prometheus.CounterValue, float64(counters[ccache.UnsupportedCodeDirective]))
	ch <- prometheus.MustNewConstMetric(c.noInputFile, prometheus.CounterValue, float64(counters[ccache.NoInputFile]))
	ch <- prometheus.MustNewConstMetric(c.cleanupsPerformed, prometheus.CounterValue, float64(counters[ccache.CleanupsPerformed]))

	// gauges
	var hitRatio float64
	if cachable > 0 {
		hitRatio = float64(hits) / float64(cachable)
	}
	ch <- prometheus.MustNewConstMetric(c.cacheHitRatio, prometheus.GaugeValue, hitRatio)
	ch <- prometheus.MustNewConstMetric(c.filesInCache, prometheus.GaugeValue, float64(counters[ccache.FilesInCache]))
	ch <- prometheus.MustNewConstMetric(c.cacheSizeBytes, prometheus.GaugeValue, float64(counters[ccache.CacheSizeKibibyte]*1024))
	ch <- prometheus.MustNewConstMetric(c.maxCacheSizeBytes, prometheus.GaugeValue, float64(c.ccacheSize))
}

func main() {
	flag.TextVar(&ccacheSize, "ccache-size", 0 * units.GiB, "The ccache size")
	flag.Parse()

	if *ccacheDir == "" {
		s := os.Getenv("CCACHE_DIR")
		if s == "" {
			log.Fatal("-ccache-dir flag or CCACHE_DIR environment variable required")
		}
		*ccacheDir = s
	}
	if ccacheSize == 0 {
		s := os.Getenv("CCACHE_MAXSIZE")
		if s != "" {
			if err := ccacheSize.UnmarshalText([]byte(s)); err != nil {
				log.Fatal(err)
			}
		}
	}
	collector := &collector{
		ccacheDir: *ccacheDir,
		ccacheSize: units.MetricBytes(ccacheSize),
		call: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "call_total"),
			"Cache calls (total)",
			nil,
			nil,
		),
		callCachable: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "call_cachable_total"),
			"Cachable cache calls (total)",
			nil,
			nil,
		),
		callHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "call_hit_total"),
			"Cache hits",
			[]string{"mode"},
			nil,
		),
		cacheHitRatio: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "cache_hit_ratio"),
			"Cache hit ratio (direct + preprocessed) / miss",
			nil,
			nil,
		),
		calledForLink: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "called_for_link_total"),
			"Called for link",
			nil,
			nil,
		),
		calledForPreprocessing: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "called_for_preprocessing_total"),
			"Called for preprocessing",
			nil,
			nil,
		),
		compilationFailed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "compilation_failed_total"),
			"Compilation failed",
			nil,
			nil,
		),
		preprocessingFailed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "preprocessing_failed_total"),
			"Preprocessing failed",
			nil,
			nil,
		),
		unsupportedCodeDirective: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "unsupported_code_directive_total"),
			"Unsupported code directive",
			nil,
			nil,
		),
		noInputFile: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "no_input_file_total"),
			"No input file",
			nil,
			nil,
		),
		cleanupsPerformed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "cleanups_performed_total"),
			"Cleanups performed",
			nil,
			nil,
		),
		filesInCache: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "cached_files"),
			"Cached files",
			nil,
			nil,
		),
		cacheSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "cache_size_bytes"),
			"Cache size (bytes)",
			nil,
			nil,
		),
		maxCacheSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "cache_size_max_bytes"),
			"Maximum cache size (bytes)",
			nil,
			nil,
		),
	}
	prometheus.MustRegister(parsingErrors)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

