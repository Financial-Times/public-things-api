package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Financial-Times/go-ft-http/fthttp"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/public-things-api/v2/things"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
	_ "github.com/joho/godotenv/autoload"
	"github.com/rcrowley/go-metrics"
)

func main() {
	app := cli.App("public-things-api", "A public RESTful API for accessing Things in neo4j")
	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "public-things-api",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})
	env := app.String(cli.StringOpt{
		Name:  "env",
		Value: "local",
		Desc:  "environment this app is running in",
	})
	cacheDuration := app.String(cli.StringOpt{
		Name:   "cache-duration",
		Value:  "30s",
		Desc:   "Duration Get requests should be cached for. e.g. 2h45m would set the max-age value to '7440' seconds",
		EnvVar: "CACHE_DURATION",
	})
	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "info",
		Desc:   "Log level of the app",
		EnvVar: "LOG_LEVEL",
	})
	publicConceptsApiURL := app.String(cli.StringOpt{
		Name:   "publicConceptsApiURL",
		Value:  "http://localhost:8080",
		Desc:   "Public concepts API endpoint URL.",
		EnvVar: "CONCEPTS_API",
	})

	log.InitLogger(*appSystemCode, *logLevel)
	log.Infof("[Startup] public-things-api is starting ")

	httpClient := fthttp.NewClient(30*time.Second, "PAC", *appSystemCode)
	app.Action = func() {
		log.Infof("public-things-api will listen on port: %s", *port)
		runServer(*port, *cacheDuration, *env, *publicConceptsApiURL, httpClient)

	}
	log.InitLogger(*appSystemCode, *logLevel)
	log.WithFields(map[string]interface{}{
		"CACHE_DURATION": *cacheDuration,
		"LOG_LEVEL":      *logLevel,
	}).Info("Starting app with arguments")
	app.Run(os.Args)
}

func runServer(port string, cacheDuration string, env string, publicConceptsApiURL string,
	httpClient *http.Client) {

	if duration, durationErr := time.ParseDuration(cacheDuration); durationErr != nil {
		log.Fatalf("Failed to parse cache duration string, %v", durationErr)
	} else {
		things.CacheControlHeader = fmt.Sprintf("max-age=%s, public", strconv.FormatFloat(duration.Seconds(), 'f', 0, 64))
	}

	servicesRouter := mux.NewRouter()

	handler := things.NewHandler(httpClient, publicConceptsApiURL)

	// Healthchecks and standards first
	healthCheck := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "public-things-api",
			Name:        "PublicThingsRead Healthcheck",
			Description: "Checks downstream services health",
			Checks:      []fthealth.Check{handler.HealthCheck()},
		},
		Timeout: 10 * time.Second,
	}

	servicesRouter.HandleFunc("/__health", fthealth.Handler(healthCheck))

	// Then API specific ones:
	handler.RegisterHandlers(servicesRouter)

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.Logger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	// The following endpoints should not be monitored or logged (varnish calls one of these every second, depending on config)
	// The top one of these build info endpoints feels more correct, but the lower one matches what we have in Dropwizard,
	// so it's what apps expect currently same as ping, the content of build-info needs more definition
	// Healthchecks and standards first

	http.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
	http.HandleFunc(status.BuildInfoPathDW, status.BuildInfoHandler)
	servicesRouter.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(handler.GTG))
	http.Handle("/", monitoringRouter)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Unable to start server: %v", err)
	}
}
