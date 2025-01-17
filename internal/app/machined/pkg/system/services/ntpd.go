/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// NTPd implements the Service interface. It serves as the concrete type with
// the required methods.
type NTPd struct{}

// ID implements the Service interface.
func (n *NTPd) ID(data *userdata.UserData) string {
	return "ntpd"
}

// PreFunc implements the Service interface.
func (n *NTPd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	return containerd.Import(constants.SystemContainerdNamespace, &containerd.ImportRequest{
		Path: "/usr/images/ntpd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/ntpd"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *NTPd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *NTPd) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *NTPd) DependsOn(data *userdata.UserData) []string {
	return []string{"containerd"}
}

func (n *NTPd) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/ntpd"

	args := runner.Args{
		ID:          n.ID(data),
		ProcessArgs: []string{"/ntpd", "--userdata=" + constants.UserDataPath},
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		data,
		&args,
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*32)),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}
