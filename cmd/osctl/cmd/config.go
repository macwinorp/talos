/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/userdata/generate"
	"gopkg.in/yaml.v2"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration",
	Long:  ``,
}

// configTargetCmd represents the config target command.
var configTargetCmd = &cobra.Command{
	Use:   "target <target>",
	Short: "Set the target for the current context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		target = args[0]
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		if c.Context == "" {
			helpers.Fatalf("no context is set")
		}
		c.Contexts[c.Context].Target = target
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configContextCmd represents the configc context command.
var configContextCmd = &cobra.Command{
	Use:   "context <context>",
	Short: "Set the current context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		c.Context = context
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configAddCmd represents the config add command.
var configAddCmd = &cobra.Command{
	Use:   "add <context>",
	Short: "Add a new context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		caBytes, err := ioutil.ReadFile(ca)
		if err != nil {
			helpers.Fatalf("error reading CA: %s", err)
		}
		crtBytes, err := ioutil.ReadFile(crt)
		if err != nil {
			helpers.Fatalf("error reading certificate: %s", err)
		}
		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			helpers.Fatalf("error reading key: %s", err)
		}
		newContext := &config.Context{
			CA:  base64.StdEncoding.EncodeToString(caBytes),
			Crt: base64.StdEncoding.EncodeToString(crtBytes),
			Key: base64.StdEncoding.EncodeToString(keyBytes),
		}
		if c.Contexts == nil {
			c.Contexts = map[string]*config.Context{}
		}
		c.Contexts[context] = newContext
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configGenerateCmd represents the config generate command.
var configGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a set of configuration files",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			log.Fatal("expected a cluster name and comma delimited list of IP addresses")
		}
		input, err := generate.NewInput(args[0], strings.Split(args[1], ","))
		if err != nil {
			helpers.Fatalf("failed to generate PKI and tokens: %v", err)
		}

		workingDir, err := os.Getwd()
		if err != nil {
			helpers.Fatalf("failed to fetch current working dir: %v", err)
		}

		var udType generate.Type
		for idx, master := range strings.Split(args[1], ",") {
			input.Index = idx
			input.IP = net.ParseIP(master)
			if input.Index == 0 {
				udType = generate.TypeInit
			} else {
				udType = generate.TypeControlPlane
			}

			if err = writeUserdata(input, udType, "master-"+strconv.Itoa(idx+1)); err != nil {
				helpers.Fatalf("failed to generate userdata for %s: %v", "master-"+strconv.Itoa(idx+1), err)
			}
			fmt.Println("created file", workingDir+"/master-"+strconv.Itoa(idx+1)+".yaml")
		}
		input.IP = nil

		if err = writeUserdata(input, generate.TypeJoin, "worker"); err != nil {
			helpers.Fatalf("failed to generate userdata for %s: %v", "worker", err)
		}
		fmt.Println("created file", workingDir+"/worker.yaml")

		data, err := generate.Talosconfig(input)
		if err != nil {
			helpers.Fatalf("failed to generate talosconfig: %v", err)
		}
		if err = ioutil.WriteFile("talosconfig", []byte(data), 0644); err != nil {
			helpers.Fatalf("%v", err)
		}
		fmt.Println("created file", workingDir+"/talosconfig")
	},
}

func writeUserdata(input *generate.Input, t generate.Type, name string) (err error) {
	var data string
	data, err = generate.Userdata(t, input)
	if err != nil {
		return err
	}
	ud := &userdata.UserData{}
	if err = yaml.Unmarshal([]byte(data), ud); err != nil {
		return err
	}
	if err = ud.Validate(); err != nil {
		return err

	}
	if err = ioutil.WriteFile(strings.ToLower(name)+".yaml", []byte(data), 0644); err != nil {
		return err
	}
	return nil
}

func init() {
	configCmd.AddCommand(configContextCmd, configTargetCmd, configAddCmd, configGenerateCmd)
	configAddCmd.Flags().StringVar(&ca, "ca", "", "the path to the CA certificate")
	configAddCmd.Flags().StringVar(&crt, "crt", "", "the path to the certificate")
	configAddCmd.Flags().StringVar(&key, "key", "", "the path to the key")
	helpers.Should(configAddCmd.MarkFlagRequired("ca"))
	helpers.Should(configAddCmd.MarkFlagRequired("crt"))
	helpers.Should(configAddCmd.MarkFlagRequired("key"))
	rootCmd.AddCommand(configCmd)
}
