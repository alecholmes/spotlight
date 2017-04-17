# Spotlight (BETA)

Spotlight is a webapp to for sharing music using Spotify's collaborative playlists. It is currently running [here](http://spotlight.alecholmes.com/). This repository has the source code along with instructions for running locally. The app can also be easily to deployed as a Docker image, including on AWS Elastic Beanstalk.

## Development

### Adding a new dependency

1. `go get -u DEP`
2. Update code to use new dependency
3. `godep save`

## Running locally

### Requirements

* Spotify account
* Go 1.7 or newer
* MySQL 5.6 or newer
* Git

### Initial setup

Running locally for the first time requires some manual setup.

*Step 0*, create a Spotify application in Spotify's [developer portal](https://developer.spotify.com/my-applications/#!/applications).


*Step 1*, decide where to keep this repository locally:

```
export SPOTLIGHT_ROOT=/Users/you/spotlight
export SPOTLIGHT_APP_ROOT=$SPOTLIGHT_ROOT/src/github.com/alecholmes/spotlight
```


*Step 2*, clone the repo this that directory:

```
mkdir -p $SPOTLIGHT_APP_ROOT
cd $SPOTLIGHT_APP_ROOT
git clone https://github.com/alecholmes/spotlight.git .
```


*Step 3*, create a development config file based on the sample template:

```
cd $SPOTLIGHT_APP_ROOT

cp app/config/sample-development.yaml app/config/development.yaml

# Open up development.yaml and replace all the `REPLACE_ME` values.
vim app/config/development.yaml
```


*Step 4*, create a new database in MySQL and manually create the schema:

```
sudo mysql

mysql> CREATE DATABASE spotlight_development;

mysql> USE spotlight_development;

mysql> [paste contents of app/model/migrations/v001_initial_schema.sql]
```

### Running

```
cd $SPOTLIGHT_APP_ROOT

export GOPATH=$SPOTLIGHT_ROOT

ENVIRONMENT=development go run main.go -stderrthreshold=INFO
```

Then, open a browser to [http://localhost:8989](http://localhost:8989).


## Running in AWS Elastic Beanstalk (incomplete instructions)

Spotlight can be easily run in AWS using Elastic Beanstalk. RDS MySQL may be used as a datastore.

To create a build artifact to deploy as zip file:

```
rm spotlight.zip; zip -r spotlight.zip .
```

TODO: complete instructions.

# License

This source code is open source through the GNU Affero General Public License v3.0. See the LICENSE file for full details or [this helpful summary](https://choosealicense.com/licenses/agpl-3.0/).

