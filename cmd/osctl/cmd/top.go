/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/proc"
	"golang.org/x/crypto/ssh/terminal"
)

// versionCmd represents the version command
var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Streams top output",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			var err error

			if oneTime {
				var output string
				output, err = topOutput(globalCtx, c)
				if err != nil {
					log.Fatal(err)
				}
				// Note this is unlimited output of process lines
				// we arent artificially limited by the box we would otherwise draw
				fmt.Println(output)
				return
			}

			if err = ui.Init(); err != nil {
				log.Fatalf("failed to initialize termui: %v", err)
			}
			defer ui.Close()

			topUI(globalCtx, c)
		})
	},
}

var sortMethod string
var oneTime bool

func init() {
	topCmd.Flags().StringVarP(&sortMethod, "sort", "s", "rss", "Column to sort output by. [rss|cpu]")
	topCmd.Flags().BoolVarP(&oneTime, "once", "1", false, "Print the current top output ( no gui/auto refresh )")
	rootCmd.AddCommand(topCmd)
}

// nolint: gocyclo
func topUI(ctx context.Context, c *client.Client) {

	l := widgets.NewParagraph()
	l.Title = "Top"
	l.WrapText = false

	var processOutput string

	draw := func() {

		// Attempt to get terminal dimensions
		// Since we're getting this data on each call
		// we'll be able to handle terminal window resizing
		w, h, err := terminal.GetSize(0)
		if err != nil {
			log.Fatal("Unable to determine terminal size")
		}
		// x, y, w, h
		l.SetRect(0, 0, w, h)

		processOutput, err = topOutput(ctx, c)
		if err != nil {
			log.Println(err)
			return
		}

		// Dont refresh if we dont have any output
		if processOutput == "" {
			return
		}

		// Truncate our output based on terminal size
		l.Text = processOutput

		ui.Render(l)
	}

	draw()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second).C
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "r", "m":
				sortMethod = "rss"
			case "c":
				sortMethod = "cpu"
			}
		case <-ticker:
			draw()
		}
	}
}

type by func(p1, p2 *proc.ProcessList) bool

func (b by) sort(procs []proc.ProcessList) {
	ps := &procSorter{
		procs: procs,
		by:    b, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type procSorter struct {
	procs []proc.ProcessList
	by    func(p1, p2 *proc.ProcessList) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *procSorter) Len() int {
	return len(s.procs)
}

// Swap is part of sort.Interface.
func (s *procSorter) Swap(i, j int) {
	s.procs[i], s.procs[j] = s.procs[j], s.procs[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *procSorter) Less(i, j int) bool {
	return s.by(&s.procs[i], &s.procs[j])
}

// Sort Methods
var rss = func(p1, p2 *proc.ProcessList) bool {
	// Reverse sort ( Descending )
	return p1.ResidentMemory > p2.ResidentMemory
}

var cpu = func(p1, p2 *proc.ProcessList) bool {
	// Reverse sort ( Descending )
	return p1.CPUTime > p2.CPUTime
}

func topOutput(ctx context.Context, c *client.Client) (output string, err error) {
	procs, err := c.Top(ctx)
	if err != nil {
		// TODO: Figure out how to expose errors to client without messing
		// up display
		// TODO: Update server side code to not throw an error when process
		// no longer exists ( /proc/1234/comm no such file or directory )
		return output, nil
	}

	switch sortMethod {
	case "cpu":
		by(cpu).sort(procs)
	default:
		by(rss).sort(procs)
	}

	s := make([]string, 0, len(procs))
	s = append(s, "PID | State | Threads | CPU Time | VirtMem | ResMem | Command")
	var cmdline string
	for _, p := range procs {
		switch {
		case p.Executable == "":
			cmdline = p.Command
		case p.Args != "" && strings.Fields(p.Args)[0] == filepath.Base(strings.Fields(p.Executable)[0]):
			cmdline = strings.Replace(p.Args, strings.Fields(p.Args)[0], p.Executable, 1)
		default:
			cmdline = p.Args
		}

		s = append(s,
			fmt.Sprintf("%6d | %1s | %4d | %8.2f | %7s | %7s | %s",
				p.Pid, p.State, p.NumThreads, p.CPUTime, bytefmt.ByteSize(p.VirtualMemory), bytefmt.ByteSize(p.ResidentMemory), cmdline))
	}

	return columnize.SimpleFormat(s), err
}
