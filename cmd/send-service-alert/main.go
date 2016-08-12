package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"code.cloudfoundry.org/lager"

	"github.com/pivotal-cf/service-alerts-client/client"

	"gopkg.in/yaml.v2"
)

func main() {
	configFilePath := flag.String("config", "", "config file path")
	product := flag.String("product", "", "name of product")
	serviceInstanceID := flag.String("service-instance", "", "service instance ID (optional)")
	subject := flag.String("subject", "", "email subject")
	content := flag.String("content", "", "email body content")
	flag.Parse()

	configBytes, err := ioutil.ReadFile(*configFilePath)
	mustNot(err)

	var config client.Config
	must(yaml.Unmarshal(configBytes, &config))

	logger := lager.NewLogger("service alerts client")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	alertsClient := client.New(config, logger)
	clientErr := alertsClient.SendServiceAlert(*product, *subject, *serviceInstanceID, *content)
	if clientErr != nil {
		switch clientErr.(type) {
		case client.HTTPRequestError:
			logger.Info(clientErr.Error())
			fmt.Fprintln(os.Stdout, clientErr.(client.HTTPRequestError).ErrorMessageForUser())
			os.Exit(2)
		default:
			mustNot(clientErr)
		}
	}
}

func mustNot(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

var must = mustNot
