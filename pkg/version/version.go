// Copyright 2024 Matrix Origin.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"bytes"
	"fmt"
	"os"
)

var (
	// GoVersion go version, setup by makefile
	GoVersion = ""
	// BranchName git branch, setup by makefile
	BranchName = ""
	// CommitID git commit id, setup by makefile
	CommitID = ""
	// BuildTime build time, setup by makefile
	BuildTime = ""
	// Version version, setup by makefile
	Version = ""
)

func GetInfo() string {
	buf := bytes.NewBuffer(make([]byte, 1024))
	buf.WriteString(fmt.Sprintf("%s build info:\n", os.Args[0]))
	buf.WriteString(fmt.Sprintf("  The golang version used to build this binary: %s\n", GoVersion))
	buf.WriteString(fmt.Sprintf("  Git branch name: %s\n", BranchName))
	buf.WriteString(fmt.Sprintf("  Git commit ID: %s\n", CommitID))
	buf.WriteString(fmt.Sprintf("  Buildtime: %s\n", BuildTime))
	buf.WriteString(fmt.Sprintf("  Version: %s\n", Version))
	return buf.String()
}

func GetInfoOneLine() string {
	return fmt.Sprintf("%s v%s compiled at %s (revision id: %s.%s) (%s)",
		os.Args[0], Version, BuildTime, CommitID, BranchName, GoVersion)
}
