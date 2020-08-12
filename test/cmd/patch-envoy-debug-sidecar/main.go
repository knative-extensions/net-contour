/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	. "github.com/dprotaso/go-yit"
)

var sidecarNode *yaml.Node

func init() {
	sidecarYAML := `
name: config-dumper
image: ko://knative.dev/net-contour/test/cmd/config-dumper
`

	var doc yaml.Node
	err := yaml.Unmarshal([]byte(sidecarYAML), &doc)
	if err != nil {
		log.Fatalf("failed to create sidecar yaml: %s", err)
	}

	sidecarNode = doc.Content[0]
}

func main() {
	// The loop is to support multi-document yaml files.
	// This is handled by using a yaml.Decoder and reading objects until io.EOF, see:
	// https://godoc.org/gopkg.in/yaml.v3#Decoder.Decode
	e := yaml.NewEncoder(os.Stdout)
	e.SetIndent(2)
	defer e.Close()

	decoder := yaml.NewDecoder(os.Stdin)
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("failed to parse yaml: %s", err)
		}

		patchEnvoyDaemonSet(&doc)

		if err := e.Encode(&doc); err != nil {
			log.Fatalf("failed to encode output: %s", err)
		}
	}

}

func patchEnvoyDaemonSet(doc *yaml.Node) {
	// yit
	it := FromNode(doc).
		Values().
		Filter(
			// give me DaemonSets with the name "envoy"
			Intersect(
				WithMapKeyValue(
					WithStringValue("kind"), WithStringValue("DaemonSet"),
				),
				WithMapKeyValue(
					WithStringValue("metadata"),
					WithMapKeyValue(
						WithStringValue("name"), WithStringValue("envoy"),
					),
				),
			),
		).
		// let's iterate over all the nodes at the path
		// .spec.template.spec.containers
		ValuesForMap(WithStringValue("spec"), WithKind(yaml.MappingNode)).
		ValuesForMap(WithStringValue("template"), WithKind(yaml.MappingNode)).
		ValuesForMap(WithStringValue("spec"), WithKind(yaml.MappingNode)).
		ValuesForMap(WithStringValue("containers"), WithKind(yaml.SequenceNode))

	for node, ok := it(); ok; node, ok = it() {
		node.Content = append(node.Content, sidecarNode)
	}
}
