package crd

import (
	_ "embed"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

//go:embed bases/pdok.nl_ogcapis.yaml
var ogcapiCRD []byte

func GetOGCApiCRD() (v1.CustomResourceDefinition, error) {
	crd := v1.CustomResourceDefinition{}
	err := yaml.Unmarshal(ogcapiCRD, &crd)

	return crd, err
}
