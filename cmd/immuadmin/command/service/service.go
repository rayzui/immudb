/*
Copyright 2019-2020 vChain, Inc.

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

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	srvc "github.com/codenotary/immudb/cmd/immuadmin/command/service/constants"

	"github.com/spf13/cobra"
	daem "github.com/takama/daemon"
)

var installableServices = []string{"immudb", "immugw"}
var availableCommands = []string{"install", "uninstall", "start", "stop", "restart", "status"}

func (cl *commandline) Service(cmd *cobra.Command) {
	ccmd := &cobra.Command{
		Use:   fmt.Sprintf("service %v %v", installableServices, availableCommands),
		Short: "Manage immu services",
		Long: fmt.Sprintf(`Manage immudb related services.
Available services: immudb and immugw.
Root permission are required in order to make administrator operations.
%s`, srvc.UsageDet),
		ValidArgs: availableCommands,
		Example:   srvc.UsageExamples,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("required a service name")
			}
			if stringInSlice("--remove-files", os.Args) {
				return nil
			}
			if len(args) < 2 {
				return errors.New("required a command name")
			}
			if !stringInSlice(args[0], installableServices) {
				return fmt.Errorf("invalid service argument specified: %s. Available list is %v", args[0], installableServices)
			}
			if !stringInSlice(args[1], availableCommands) {
				return fmt.Errorf("invalid command argument specified: %s. Available list is %v", args[1], availableCommands)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			if ok, e := cl.sservice.IsAdmin(); !ok {
				return e
			}
			// delayed operation
			t, _ := cmd.Flags().GetInt("time")

			if t > 0 {
				// if t is present we relaunch same command with --delayed flag set
				var argi []string
				for i, k := range os.Args {
					if k == "--time" || k == "-t" {
						continue
					}
					if _, err = strconv.ParseFloat(k, 64); err == nil {
						continue
					}
					if i != 0 {
						argi = append(argi, k)
					}
				}
				argi = append(argi, "--delayed")
				argi = append(argi, strconv.Itoa(t))
				if err = launch(os.Args[0], argi); err != nil {
					return err
				}
				return nil
			}
			// if delayed flag is set we delay the execution of the action
			d, _ := cmd.Flags().GetInt("delayed")
			if d > 0 {
				time.Sleep(time.Duration(d) * time.Second)
			}

			var msg string
			var localFile string
			var execPath string

			if args[1] == "install" {
				if localFile, err = cmd.Flags().GetString("local-file"); err != nil {
					return err
				}
				if localFile, err = cl.sservice.GetExecutable(localFile, args[0]); err != nil {
					return err
				}
			}

			if execPath, err = cl.sservice.GetDefaultExecPath(localFile); err != nil {
				return err
			}

			if stringInSlice("--remove-files", os.Args) {
				if localFile, err = cmd.Flags().GetString("local-file"); err != nil {
					return err
				}
			}

			if args[1] == "install" {
				if execPath, err = cl.sservice.CopyExecInOsDefault(localFile); err != nil {
					return err
				}
			}

			// todo remove all involved files
			if remove, _ := cmd.Flags().GetBool("remove-files"); remove {
				if err = os.Remove(execPath); err != nil {
					return err
				}
				return nil
			}

			daemon, err := cl.sservice.NewDaemon(args[0], args[0], execPath)
			if err != nil {
				return err
			}

			var u string
			switch args[1] {
			case "install":
				if args[0] == "immugw" {
					fmt.Fprintf(cmd.OutOrStdout(), "To provide the maximum level of security, we recommend running immugw on a different machine than immudb server. Continue ? [Y/n]")
					if u, err = cl.treader.ReadFromTerminalYN("Y"); err != nil {
						return err
					}
					if u != "y" {
						fmt.Fprintf(cmd.OutOrStdout(), "No action\n")
						return
					}
				}

				fmt.Fprintf(cmd.OutOrStdout(), "installing "+localFile+"...\n")
				if err = cl.sservice.InstallSetup(args[0]); err != nil {
					return err
				}
				var cp string
				if cp, err = cl.sservice.GetDefaultConfigPath(args[0]); err != nil {
					return err
				}
				if msg, err = daemon.Install("--config", cp); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")

				if msg, err = daemon.Start(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")

				return nil
			case "uninstall":
				// check if already installed
				var status string
				if status, err = daemon.Status(); err != nil {
					if err == daem.ErrNotInstalled {
						return err
					}
				}
				// stopping service first
				if cl.sservice.IsRunning(status) {
					if msg, err = daemon.Stop(); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to uninstall %s? [y/N]", args[0])
				if u, err = cl.treader.ReadFromTerminalYN("N"); err != nil {
					return err
				}
				if u != "y" {
					fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
					return
				}
				if msg, err = daemon.Remove(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				if args[0] == "immudb" {
					fmt.Fprintf(cmd.OutOrStdout(), "Erase data? [y/N]")
					if u, err = cl.treader.ReadFromTerminalYN("N"); err != nil {
						return err
					}
					if u != "y" {
						fmt.Fprintf(cmd.OutOrStdout(), "No data removed\n")
					} else {
						if err = cl.sservice.EraseData(args[0]); err != nil {
							return err
						}
						fmt.Fprintf(cmd.OutOrStdout(), "Data folder removed\n")
					}
				}
				if err = cl.sservice.RemoveProgramFiles(args[0]); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Program files removed\n")
				if err = cl.sservice.UninstallSetup(args[0]); err != nil {
					return err
				}
				return nil
			case "start":
				if msg, err = daemon.Start(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				return nil
			case "stop":
				if msg, err = daemon.Stop(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				return nil
			case "restart":
				if _, err = daemon.Stop(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				if msg, err = daemon.Start(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				return nil
			case "status":
				if msg, err = daemon.Status(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), msg+"\n")
				return nil
			}
			return nil
		},
	}
	ccmd.PersistentFlags().Bool("remove-files", false, "clean up from all service files")
	ccmd.PersistentFlags().IntP("time", "t", 0, "number of seconds to wait before stopping | restarting the service")
	ccmd.PersistentFlags().Int("delayed", 0, "number of seconds to wait before repeat the parent command. HIDDEN")
	ccmd.PersistentFlags().MarkHidden("delayed")
	ccmd.PersistentFlags().String("local-file", "", "local executable file name")
	cmd.AddCommand(ccmd)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func launch(command string, args []string) (err error) {
	cmd := exec.Command(command, args...)
	if err = cmd.Start(); err != nil {
		return err
	}
	return nil
}
