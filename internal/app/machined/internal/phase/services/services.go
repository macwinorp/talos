/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Services represents the Services task.
type Services struct{}

// NewServicesTask initializes and returns an Services task.
func NewServicesTask() phase.Task {
	return &Services{}
}

// RuntimeFunc returns the runtime function.
func (task *Services) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(platform platform.Platform, data *userdata.UserData) error {
		return task.runtime(data, mode)
	}
}

func (task *Services) runtime(data *userdata.UserData, mode runtime.Mode) (err error) {
	task.startSystemServices(data, mode)
	task.startKubernetesServices(data)

	return nil
}

func (task *Services) startSystemServices(data *userdata.UserData, mode runtime.Mode) {
	svcs := system.Services(data)
	// Start the services common to all nodes.
	svcs.Load(
		&services.MachinedAPI{},
		&services.Networkd{},
		&services.Containerd{},
		&services.Udevd{},
		&services.OSD{},
		&services.NTPd{},
	)

	if mode != runtime.Container {
		// udevd-trigger is causing stalls/unresponsive stuff when running in local mode
		// TODO: investigate root cause, but workaround for now is to skip it in container mode
		svcs.Load(
			&services.UdevdTrigger{},
		)
	}

	// Start the services common to all master nodes.
	if data.Services.Kubeadm.IsControlPlane() {
		svcs.Load(
			&services.Trustd{},
			&services.Proxyd{},
		)
	}

}

func (task *Services) startKubernetesServices(data *userdata.UserData) {
	svcs := system.Services(data)
	svcs.Load(
		&services.Kubelet{},
		&services.Kubeadm{},
	)
}
