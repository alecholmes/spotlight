package app

import (
	"io/ioutil"
	"os"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/notifiers"
	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/app/requests"

	"github.com/go-errors/errors"
	yaml "gopkg.in/yaml.v2"
)

type HTTPServerConfig struct {
	Port int
}

type AppConfig struct {
	AppBaseURL  string                  `yaml:"app_base_url"`
	AppEmail    string                  `yaml:"app_email"`
	Database    *model.DBConfig         `yaml:"database"`
	Email       *notifiers.MailerConfig `yaml:"email"`
	HTTPServer  *HTTPServerConfig       `yaml:"http_server"`
	HTTPSession *requests.SessionConfig `yaml:"http_session"`
	OAuth       *oauth.Config           `yaml:"oauth"`
}

func ParseConfig(filename string) (*AppConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer file.Close()

	rawYAML, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	appConfig := &AppConfig{}
	if err := yaml.Unmarshal(rawYAML, &appConfig); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return appConfig, nil
}
