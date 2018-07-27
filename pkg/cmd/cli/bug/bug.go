/*
Copyright 2018 the Heptio Ark contributors.

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

package bug

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"text/template"

	"github.com/heptio/ark/pkg/buildinfo"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/spf13/cobra"
)

const (
	linuxBrowserLaunchCmd = "xdg-open"
	issueURL              = "https://github.com/heptio/ark/issues/new"
	issueTemplate         = `
**What steps did you take and what happened:**
[A clear and concise description of what the bug is, and what commands you ran.)


**What did you expect to happen:**


**The output of the following commands will help us better understand what's going on**:
(Pasting long output into a [GitHub gist](https://gist.github.com) or other pastebin is fine.)

* ` + "`kubectl logs deployment/ark -n heptio-ark`" + `
* ` + "`ark backup describe <backupname>` or `kubectl get backup/<backupname> -n heptio-ark -o yaml`" + `
* ` + "`ark backup logs <backupname>`" + `
* ` + "`ark restore describe <restorename>` or `kubectl get restore/<restorename> -n heptio-ark -o yaml`" + `
* ` + "`ark restore logs <restorename>`" + `


**Anything else you would like to add:**
[Miscellaneous information that will assist in solving the issue.]


**Environment:**

- Ark version (use ` + "`ark version`" + `): {{.ArkVersion}}
    - GitCommit: {{.GitCommit}}
    - GitTreeState: {{.GitTreeState}}
- Kubernetes version (use ` + "`kubectl version`" + `): 

` + "```" + `
{{.KubectlVersion}}
` + "```" + `

- Kubernetes installer & version:
- Cloud provider or hardware configuration:
- OS (e.g. from ` + "`/etc/os-release`" + `): 
    - RuntimeOS: {{.RuntimeOS}} 
    - RuntimeArch: {{.RuntimeArch}}

`
)

func NewCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "bug",
		Short: "Report an Ark bug",
		Long:  "Open a browser window to report an Ark bug",
		Run: func(c *cobra.Command, args []string) {
			body, err := renderToString(newBugInfo())
			cmd.CheckError(err)
			cmd.CheckError(showIssueInBrowser(body))
		},
	}
	return c
}

type arkBugInfo struct {
	ArkVersion     string
	GitCommit      string
	GitTreeState   string
	RuntimeOS      string
	RuntimeArch    string
	KubectlVersion string
}

func newBugInfo() *arkBugInfo {
	var kubectlVersion string
	kubectlCmd := exec.Command("kubectl", "version")
	versionOut, err := kubectlCmd.Output()
	if err == nil {
		kubectlVersion = strings.TrimSpace(string(versionOut))
	}

	bugInfo := arkBugInfo{
		ArkVersion:     buildinfo.Version,
		GitCommit:      buildinfo.GitSHA,
		GitTreeState:   buildinfo.GitTreeState,
		RuntimeOS:      runtime.GOOS,
		RuntimeArch:    runtime.GOARCH,
		KubectlVersion: kubectlVersion}
	return &bugInfo
}

func renderToString(bugInfo *arkBugInfo) (string, error) {
	outputTemplate, err := template.New("ghissue").Parse(issueTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = outputTemplate.Execute(&buf, bugInfo)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// is an executable available on the current PATH?
func cmdExistsOnPath(name string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return true
}

// open a browser window to submit a Github issue using a platform specific binary.
func showIssueInBrowser(body string) error {
	url := issueURL + "?body=" + url.QueryEscape(body)
	var err error
	switch os := runtime.GOOS; os {
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		if cmdExistsOnPath(linuxBrowserLaunchCmd) {
			err = exec.Command(linuxBrowserLaunchCmd, url).Start()
		} else {
			err = fmt.Errorf("Ark can't open a browser window using the command '%s'", linuxBrowserLaunchCmd)
		}
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		err = fmt.Errorf("Ark can't open a browser window on platform %s", os)
	}
	return err
}
