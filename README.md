# ogcapi-operator

_Kubernetes controller/operator to serve an OGC API according to spec._

[![Build](https://github.com/PDOK/ogcapi-operator/actions/workflows/build-and-publish-image.yml/badge.svg)](https://github.com/PDOK/ogcapi-operator/actions/workflows/build-and-publish-image.yml)
[![Lint (go)](https://github.com/PDOK/ogcapi-operator/actions/workflows/lint-go.yml/badge.svg)](https://github.com/PDOK/ogcapi-operator/actions/workflows/lint-go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/PDOK/ogcapi-operator)](https://goreportcard.com/report/github.com/PDOK/ogcapi-operator)
[![Coverage (go)](https://github.com/PDOK/ogcapi-operator/wiki/coverage.svg)](https://raw.githack.com/wiki/PDOK/ogcapi-operator/coverage.html)
[![GitHub license](https://img.shields.io/github/license/PDOK/ogcapi-operator)](https://github.com/PDOK/ogcapi-operator/blob/master/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/pdok/ogcapi-operator.svg)](https://hub.docker.com/r/pdok/ogcapi-operator)

## Description

This Kubernetes controller cq operator (an operator could be described as a specialized controller)
ensures that the necessary resources are created or kept up2date in a cluster
to serve [OGC API](https://ogcapi.ogc.org/)s.
The specification for such an OGC API is expressed as a [GoKoala](https://github.com/PDOK/gokoala) Config
(which is the engine used for actually serving the API)
and is placed in the [Custom Resource Definition (CRD)](./config/crd/bases/pdok.nl_ogcapis.yaml)
in this repository.
The naming and descriptions in this CRD should make it pretty self-explanatory.

The resources created/owned by this operator will be deleted again when the corresponding OGCAPI CR is deleted itself. Resources that are created are:

* A ConfigMap holding the GoKoala Config
* A Deployment for the GoKoala runtime, using said ConfigMap
* A HorizontalPodAutoScaler to automatically scale said Deployment according to CPU load or memory pressure
* A Service to expose the GoKoala Deployment in the cluster
* A Traefik IngressRoute to expose said Service to the outside world
  * Along with some Middleware

The resources will be in the same namespace as the corresponding OGCAPI CR.

## Run/usage

```shell
go build github.com/PDOK/ogcapi-operator/cmd -o manager
```

or

```shell
docker build -t pdok/ogcapi-operator .
```

```text
USAGE:
   <ogcapi-controller-manager> [OPTIONS]

OPTIONS:
  -h, --help
        Display these options.
  --enable-http2
        If set, HTTP/2 will be enabled for the metrics and webhook servers
  --gokoala-image string
        The image to use in the gokoala pod (defaults to the latest compatible image)
  --health-probe-bind-address string
        The address the probe endpoint binds to. (default ":8081")
  --kubeconfig string
        Paths to a kubeconfig. Only required if out-of-cluster.
        Alternatively use the KUBECONFIG environment variable.
  --leader-elect
        Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  --metrics-bind-address string
        The address the metric endpoint binds to. (default ":8080")
  --metrics-secure
        If set the metrics endpoint is served securely
  --zap-devel
        Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
  --zap-encoder value
        Zap log encoding (one of 'json' or 'console')
  --zap-log-level value
        Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  --zap-stacktrace-level value
        Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
  --zap-time-encoding value
        Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
        
OPTIONS can also be set via environment variables (with SCREAMING_SNAKE_CASE instead of kebab-case). CLI flags have precedence. 
```

### Setup in the cluster

This depends on your own way of maintaining your cluster config.
Inspiration can be found in the [config](./config) dir
which holds "kustomizations" to use with Kustomize.
You could use `make install` for this.
Please refer to the general documentation in the [kubebuilder book](https://kubebuilder.io) for more info.

## Develop

The project is written in Go and scaffolded with [kubebuilder](https://kubebuilder.io).

### kubebuilder

This operator was scaffolded with [kubebuilder](https://kubebuilder.io)

Read the manual when you want/need to make changes.
E.g. run `make test` before committing.

### Linting

Install [golangci-lint](https://golangci-lint.run/usage/install/) and run `golangci-lint run`
from the root.
(Don't run `make lint`, it uses an old version of golangci-lint.)

## Misc

### How to Contribute

Make a pull request...

### Contact

Contacting the maintainers can be done through the issue tracker.

## License

MIT License

Copyright (c) 2024 Publieke Dienstverlening op de Kaart

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

