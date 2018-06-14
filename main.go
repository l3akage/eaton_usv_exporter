package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

const version string = "0.1"

var (
	showVersion   = flag.Bool("version", false, "Print version information.")
	listenAddress = flag.String("listen-address", ":9332", "Address on which to expose metrics.")
	metricsPath   = flag.String("path", "/metrics", "Path under which to expose metrics.")
	snmpTargets   = flag.String("targets", "", "targets to scrape")
	snmpCommunity = flag.String("community", "", "SNMP community")
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: eaton_usv_exporter [ ... ]\n\nParameters:")
		fmt.Println()
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	startServer()
}

func printVersion() {
	fmt.Println("eaton_usv_exporter")
	fmt.Printf("Version: %s\n", version)
}

func startServer() {
	log.Infof("Starting EATON usv exporter (Version: %s)\n", version)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>EATON usv Exporter (Version ` + version + `)</title></head>
			<body>
			<h1>EATON usv Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			<h2>More information:</h2>
			<p><a href="https://github.com/l3akage/eaton_usv_exporter">github.com/l3akage/eaton_usv_exporter</a></p>
			</body>
			</html>`))
	})
	http.HandleFunc(*metricsPath, handleMetricsRequest)

	log.Infof("Listening for %s on %s\n", *metricsPath, *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(&eatonUsvCollector{})

	promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorLog:      log.NewErrorLogger(),
		ErrorHandling: promhttp.ContinueOnError}).ServeHTTP(w, r)
}
