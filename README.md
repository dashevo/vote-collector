# vote-collector

[![Build Status](https://travis-ci.com/dashevo/vote-collector.svg?branch=master)](https://travis-ci.com/dashevo/vote-collector)
[![Go Report](https://goreportcard.com/badge/github.com/dashevo/vote-collector)](https://goreportcard.com/badge/github.com/dashevo/vote-collector)

> Simple HTTP vote collection API service in Go

## Table of Contents

- [Install](#install)
  - [Dependencies](#dependencies)
- [Usage](#usage)
- [Configuration](#configuration)
  - [Quick start](#quick-start)
  - [Generating a JWT](#generating-a-jwt)
- [Maintainer](#maintainer)
- [Contributing](#contributing)
- [License](#license)

## API

- POST `/api/vote`
  ```json
  {
    "addr": "XyBmeuLa8y3D3XmzPvCTj5PVh7WvMPkLn1",
    "msg": "dte2022-afrancis|ctafti",
    "sig": "IIm+2++GxT4OtTTY4aZK0iKIWh21yxiwomfY76l197qtVB42KVpy53QxS65zq1R9eN2XLcGh2YsedsVtsmrw2OE="
  }
  ```
- GET `/api/allVotes`
- GET `/api/validVotes`
- GET `/api/candidates`
  ```json
  [
    {
      "name": "John Doe, III",
      "handle": "@johndoe",
      "email": "john.doe@example.com"
    }
  ]
  ```

## Install

Clone the repo and build the project.

```sh
git clone https://github.com/dashevo/vote-collector.git
pushd ./vote-collector/
```

### Pre-Reqs

Install `go` and `dotenv`:

```sh
# dotenv - for running with .env config
curl https://webinstall.dev/dotenv | bash

# Go + standard tooling and "x" tools
curl https://webinstall.dev/go | bash
```

### Dependencies

The vote collector simply logs votes to a Postgres database, therefore a running instance of the Postgres database is required as a dependency. The connection is configured via environment variables. See [Configuration](#configuration) for more info.

## Usage

First, copy `example.env` to `.env` and modify accordingly. Postgres variables need to be configured to point to an accessible, running Postgres instance.

```sh
# config
cp example.env .env
vi .env #  (edit accordingly)

# run
go run -mod=vendor vote-collector

# -or-
go build -mod=vendor
dotenv ./vote-collector
```

## Configuration

The vote collector uses environment variables for configuration. Variables are read from a `.env` file and can be overwritten by variables defined in the environment or directly passed to the process. See all available settings in [example.env](example.env).

### Quick start

A `docker-compose` file is included for testing purposes, which also sets up a Postgres database.

```sh
cp example.env .env
vi .env #  (edit accordingly)

docker-compose up
```

To verify:

```sh
curl -i http://127.0.0.1:7001/api/health
```

### Generating a JWT

Some routes in the API are only available with authentication. These are the audit routes, which allow reading vote entries:

- GET `/api/allVotes`
- GET `/api/validVotes`

For these, a JWT token must be sent in the header (see `curl_examples.sh` in this repo). There is currently no authentication table or route, so this must be manually generated.

To generate the JWT token, you can use the [JWT Debugger](https://jwt.io/#debugger-io). Simply visit the site, adjust the payload data accordingly, and in place of `your-256-bit-secret`, use the value of `$JWT_SECRET_KEY` that you set in the .env file (can be any secret string). Then click the "Share JWT" button to retrieve the JWT. This is the value you should send in the header after "Authorization: Bearer ".

An example JWT token looks like:

```jwt
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJUZXN0IFRlc3RlcnNvbiIsInN1YiI6IkpvaG4gRG9udXQiLCJpYXQiOjE1NTE0NjYyMjN9.Z03u0ZogZZ4W2C9E7FgisQxWqp-XsnuS48JAxzRxQ1I
```

_Note that this is just an example and will not work with any production deployment._

## Maintainers

- 2022 [@coolaj86](https://github.com/coolaj86)
- 2019 [@nmarley](https://github.com/nmarley)

## Contributing

Feel free to dive in! [Open an issue](https://github.com/dashevo/vote-collector/issues/new) or submit PRs.

## License

[MIT](LICENSE) &copy; Dash Core Group, Inc.
