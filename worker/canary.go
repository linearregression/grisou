package worker

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/odacremolbap/grisou/client"
	"github.com/pkg/errors"
)

const grisouSuffix string = "grisou"

// DeploymentCanaryController structure to control kubernetes deployments
type DeploymentCanaryController struct {
	kubernetes *client.Kubernetes
	dockerHub  *client.DockerHub
}

// NewDeploymentCanaryController returns a new canary controller for deployments
func NewDeploymentCanaryController(k *client.Kubernetes, d *client.DockerHub) (*DeploymentCanaryController, error) {

	if k == nil {
		return nil, errors.New("Kubernetes client must be set")
	}

	if d == nil {
		return nil, errors.New("Docker Hub client must be set")
	}

	return &DeploymentCanaryController{k, d}, nil
}

// Check creates a canary if a deployment image is outdated
func (dcc *DeploymentCanaryController) Check() error {

	ds, err := dcc.kubernetes.Deployments()
	if err != nil {
		return errors.Wrap(err, "Couldn't retrieve kubernetes deployments")
	}

	for _, d := range ds {

		log.Debugf("Checking Deployment '%s'", d.Name)

		// if deployment is already a grisou canary, skip
		if strings.HasSuffix(d.Name, grisouSuffix) {
			continue
		}

		deployCanary := false

		for i := range d.Spec.Template.Spec.Containers {

			// get image tag
			it := strings.Split(d.Spec.Template.Spec.Containers[i].Image, ":")

			if strings.HasPrefix(it[0], "gcr.io") {
				log.Warnf("%s uses gcr.io repository, which is not supported", it[0])
				continue
			}

			if strings.HasPrefix(it[0], "quay.io") {
				log.Warnf("%s uses quay.io repository, which is not supported", it[0])
				continue
			}

			image, err := dcc.dockerHub.GetImageData(it[0])
			if err != nil {
				log.Errorf("couldn't get image data for '%s'", it[0])
			}

			// get image latest tag
			latest := image.GetLatestTag()

			if it[1] == latest {
				log.Debugf("image '%s' is already using the latet version", it[0])
				continue
			}

			// create canary deployment
			deployCanary = true
			d.Spec.Template.Spec.Containers[i].Image = fmt.Sprintf("%s:%s", it[0], latest)
		}

		if deployCanary {
			d.Name = fmt.Sprintf("%s-%s", d.Name, grisouSuffix)
			d.Spec.Template.Labels["track"] = "canary"
			d.ResourceVersion = ""
			dcc.kubernetes.CreateDeployment(&d)
		}

	}

	return nil
}