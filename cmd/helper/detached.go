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

package helper

import (
	"fmt"
	"github.com/codenotary/immudb/cmd/version"
	"github.com/mitchellh/go-ps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DetachedFlag ...
const DetachedFlag = "detached"

// DetachedShortFlag ...
const DetachedShortFlag = "d"

type Execs interface {
	Command(name string, arg ...string) *exec.Cmd
}

type execs struct{}

func (e execs) Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

type Plauncher interface {
	Detached() error
	Process(processName string) (found bool, p ps.Process, err error)
}

type plauncher struct {
	e Execs
}

func NewPlauncher() *plauncher {
	return &plauncher{execs{}}
}

// Detached launch command in background
func (pl plauncher) Detached() error {
	var err error
	var executable string
	var args []string

	if executable, err = os.Executable(); err != nil {
		return err
	}

	if exists, p, _ := pl.Process(version.App); exists {
		return fmt.Errorf("%s is already running. Pid %d", version.App, p.Pid())
	}

	for i, k := range os.Args {
		if k != "--"+DetachedFlag && k != "-"+DetachedShortFlag && i != 0 {
			args = append(args, k)
		}
	}

	cmd := pl.e.Command(executable, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	fmt.Fprintf(
		os.Stdout, "%s%s has been started with %sPID %d%s\n",
		Green, filepath.Base(executable), Blue, cmd.Process.Pid, Reset)
	return nil
}

//Process find a process with the given name
func (pl plauncher) Process(processName string) (found bool, p ps.Process, err error) {
	ps, err := ps.Processes()
	if err != nil {
		return false, p, err
	}
	for _, p1 := range ps {
		if strings.Contains(p1.Executable(), processName) {
			return true, p1, nil
		}
	}
	return false, nil, nil
}
