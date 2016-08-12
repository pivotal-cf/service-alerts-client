package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

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

	alertsClient := client.New(config)
	must(alertsClient.SendServiceAlert(*product, *subject, *serviceInstanceID, *content))
}

func mustNot(err error) {
	if err != nil {
		switch err.(type) {
		case client.HTTPRequestError:
			log.Println(err)
			fmt.Fprintf(os.Stdout, err.(client.HTTPRequestError).ErrorMessageForUser())
			os.Exit(2)
		default:
			log.Fatalln(err)
		}
	}
}

var must = mustNot
