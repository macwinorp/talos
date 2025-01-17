/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package baremetal

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"

	"golang.org/x/sys/unix"

	yaml "gopkg.in/yaml.v2"
)

const (
	mnt = "/mnt"
)

// BareMetal is a discoverer for non-cloud environments.
type BareMetal struct{}

// Name implements the platform.Platform interface.
func (b *BareMetal) Name() string {
	return "Bare Metal"
}

// UserData implements the platform.Platform interface.
func (b *BareMetal) UserData() (data *userdata.UserData, err error) {
	var option *string
	if option = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); option == nil {
		return data, errors.Errorf("no user data option was found")
	}

	if *option == constants.UserDataCIData {
		var dev *probe.ProbedBlockDevice
		dev, err = probe.GetDevWithFileSystemLabel(constants.UserDataCIData)
		if err != nil {
			return data, errors.Errorf("failed to find %s iso: %v", constants.UserDataCIData, err)
		}
		if err = os.Mkdir(mnt, 0700); err != nil {
			return data, errors.Errorf("failed to mkdir: %v", err)
		}
		if err = unix.Mount(dev.Path, mnt, dev.SuperBlock.Type(), unix.MS_RDONLY, ""); err != nil {
			return data, errors.Errorf("failed to mount iso: %v", err)
		}
		var dataBytes []byte
		dataBytes, err = ioutil.ReadFile(path.Join(mnt, "user-data"))
		if err != nil {
			return data, errors.Errorf("read user data: %s", err.Error())
		}
		if err = unix.Unmount(mnt, 0); err != nil {
			return data, errors.Errorf("failed to unmount: %v", err)
		}
		if err = yaml.Unmarshal(dataBytes, &data); err != nil {
			return data, errors.Errorf("unmarshal user data: %s", err.Error())
		}

		return data, nil
	}

	return userdata.Download(*option)
}

// Initialize provides the functionality to install talos by downloading the
// required artifacts and writing them to a target device.
// nolint: dupl
func (b *BareMetal) Initialize(data *userdata.UserData) (err error) {
	var endpoint *string
	if endpoint = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); endpoint == nil {
		return errors.Errorf("failed to find %s in kernel parameters", constants.KernelParamUserData)
	}
	cmdline := kernel.NewDefaultCmdline()
	cmdline.Append("initrd", filepath.Join("/", "default", "initramfs.xz"))
	cmdline.Append(constants.KernelParamPlatform, "bare-metal")
	cmdline.Append(constants.KernelParamUserData, *endpoint)

	if err = cmdline.AppendAll(data.Install.ExtraKernelArgs); err != nil {
		return err
	}

	// Attempt to discover a previous installation
	// An err case should only happen if no partitions
	// with matching labels were found
	var mountpoints *mount.Points
	mountpoints, err = owned.MountPointsFromLabels()
	if err != nil {
		// No previous installation was found, attempt an install
		var i *installer.Installer
		i, err = installer.NewInstaller(cmdline, data)
		if err != nil {
			return err
		}
		if err = i.Install(); err != nil {
			return errors.Wrap(err, "failed to install")
		}

		mountpoints, err = owned.MountPointsFromLabels()
		if err != nil {
			return err
		}
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
