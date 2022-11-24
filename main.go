package main

import (
	"Go-prometheus-exporter/collector"
	"Go-prometheus-exporter/config"
	zdataLog "Go-prometheus-exporter/log"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	configPath = flag.String(
		"c", "conf/etcdvalue_exporter.cfg",
		"Configuration file path",
	)
)

const (
	scrapeRateMedium = "mr"
)

func basicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		//if requiredUser or requiredPassword is null ,continue process
		if requiredUser == "" || requiredPassword == "" {
			h(w, r, ps)
			return
		}
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

func buildMetricsHandler(scrapeRate string) http.Handler {
	registry := prometheus.NewRegistry()
	exp := collector.NewExporter(scrapers[scrapeRate], scrapeRate)
	registry.MustRegister(exp)

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func nodeMetricsHandler(r *http.Request) http.Handler {
	nodeIP := r.FormValue("node_ip")
	domainID := r.FormValue("domain_id")
	zdataLog.Infof("params node ip: %v", nodeIP)
	zdataLog.Infof("params domain id: %v", domainID)
	registry := prometheus.NewRegistry()
	sc := []collector.Scraper{iceman.NodeScraper{NodeIP: nodeIP, DomainID: domainID}}
	exp := collector.NewExporter(sc, scrapeRateMedium)
	registry.MustRegister(exp)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// test receive webhook
func receiveWebHook(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("webhook response read error: %v\n", err)
	} else {
		log.Println(string(response))
	}
	w.Write([]byte{})
}

var (
	showVersion = flag.Bool(
		"version", false,
		"Print version information.",
	)
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, "v1.0.0")
		os.Exit(0)
	}

	var (
		cfg *config.Config
		err error
	)

	// initialize config in loop if etcd config file does not exist
	for {
		cfg, err = config.Initialize(*configPath)
		if err == nil {
			break
		}
		if err.Error() == config.ETCDConfNotExistMsg {
			log.Printf("%s, sleep a while and check again.\n", err.Error())
		} else {
			log.Fatalf("config.Initialize() error: %v", err)
		}
		time.Sleep(time.Duration(config.ETCDConfCheckInterval) * time.Second)
	}

	auth := cfg.Auth
	if auth.Username != "" && auth.Password != "" {
		log.Println("HTTP basic authentication enabled")
	}

	mediumMetricHandler := buildMetricsHandler(scrapeRateMedium)

	var landingPage = []byte(`<html>
<head><title>Enmotech etcdvalue_exporter</title></head>
<body>
<h1>Enmotech etcdvalue_exporter, exports values stored in ETCD cluster.</h1>
<h4>If you want to export metrics of ETCD cluster itself, just check http(s)://{etcd_host}:{etcd_port}/metrics</h4>
<h4>Check <a target="_blank" href="https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/monitoring.md">here</a> for more details</h4>
</body>
</html>
`)

	router := httprouter.New()
	router.GET(
		"/",
		basicAuth(
			func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
				w.Write(landingPage)
			},
			auth.Username, auth.Password,
		),
	)

	router.GET(
		"/metrics-mr",
		basicAuth(
			func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
				mediumMetricHandler.ServeHTTP(w, r)
			},
			auth.Username, auth.Password,
		),
	)

	router.GET(
		"/node-mr",
		basicAuth(
			func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
				nodeMetricsHandler(r).ServeHTTP(w, r)
			},
			auth.Username, auth.Password,
		),
	)

	// test receive webhook
	router.POST("/api/v1/alert_msg/webhook", basicAuth(receiveWebHook, "", ""))
	log.Println("Listening on: ", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, router))
}
