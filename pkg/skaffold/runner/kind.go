/*
Copyright 2019 The Skaffold Authors

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

package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/color"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubectl"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

// loadImagesInKindNodes loads a list of artifact images into every node of kind cluster.
func (r *SkaffoldRunner) loadImagesInKindNodes(ctx context.Context, out io.Writer, kindCluster string, artifacts []build.Artifact) error {
	start := time.Now()
	color.Default.Fprintln(out, "Loading images into kind cluster nodes...")

	var knownImages []string

	for _, artifact := range artifacts {
		// Only `kind load` the images that this runner built
		if !r.wasBuilt(artifact.Tag) {
			continue
		}

		color.Default.Fprintf(out, " - %s -> ", artifact.Tag)

		// Only `kind load` the images that are unknown to the node
		if knownImages == nil {
			var err error
			kubectlCLI := kubectl.NewFromRunContext(r.runCtx)
			if knownImages, err = findKnownImages(ctx, kubectlCLI); err != nil {
				return fmt.Errorf("unable to retrieve node's images: %w", err)
			}
		}
		if util.StrSliceContains(knownImages, artifact.Tag) {
			color.Green.Fprintln(out, "Found")
			continue
		}

		cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", "--name", kindCluster, artifact.Tag)
		if output, err := util.RunCmdOut(cmd); err != nil {
			color.Red.Fprintln(out, "Failed")
			return fmt.Errorf("unable to load image with kind %q: %w, %s", artifact.Tag, err, output)
		}

		color.Green.Fprintln(out, "Loaded")
	}

	color.Default.Fprintln(out, "Images loaded in", time.Since(start))
	return nil
}

func findKnownImages(ctx context.Context, cli *kubectl.CLI) ([]string, error) {
	nodeGetOut, err := cli.RunOut(ctx, "get", "nodes", `-ojsonpath='{@.items[*].status.images[*].names[*]}'`)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect the nodes: %w", err)
	}

	knownImages := strings.Split(string(nodeGetOut), " ")
	return knownImages, nil
}

func (r *SkaffoldRunner) wasBuilt(tag string) bool {
	for _, built := range r.builds {
		if built.Tag == tag {
			return true
		}
	}

	return false
}
