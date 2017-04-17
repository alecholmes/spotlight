package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"github.com/alecholmes/spotlight/app"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
)

func main() {
	flag.Parse() // Due to glog being needy
	defer glog.Flush()

	config, err := loadConfig()
	if err != nil {
		glog.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	signal.Notify(sigCh, os.Kill)

	// Start the app in a goroutine and wait an OS kill signal to shut it down
	var wg sync.WaitGroup
	wg.Add(1)
	stopCh := make(chan struct{})
	go func() {
		app.NewApp(config).Run(stopCh)
		wg.Done()
	}()

	go func() {
		select {
		case <-sigCh:
			close(stopCh)
		}
	}()

	wg.Wait()
}

func loadConfig() (*app.AppConfig, error) {
	env := os.Getenv("ENVIRONMENT")
	if len(env) == 0 {
		return nil, errors.Errorf("Expected ENVIRONMENT variable to be set")
	}

	configFile := fmt.Sprintf("app/config/%s.yaml", strings.ToLower(env))
	glog.Infof("Using config file %s", configFile)
	config, err := app.ParseConfig(configFile)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if len(os.Getenv("RDS_DB_NAME")) > 0 {
		glog.Info("Overriding DB config with RDS environment variable values")

		port, err := strconv.Atoi(os.Getenv("RDS_PORT"))
		if err != nil {
			return nil, errors.WrapPrefix(err, "Unable to parse RDS_PORT", 0)
		}

		config.Database.HostName = os.Getenv("RDS_HOSTNAME")
		config.Database.Port = port
		config.Database.User = os.Getenv("RDS_USERNAME")
		config.Database.Password = os.Getenv("RDS_PASSWORD")
		config.Database.Database = os.Getenv("RDS_DB_NAME")
	}

	return config, nil
}
