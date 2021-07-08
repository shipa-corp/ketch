// +build integration

package cli_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	ketch   string // ketch executable path
	ingress string // ingress IP

	framework       = "myframework"
	appImage        = "gcr.io/shipa-ci/sample-go-app:latest"
	appName         = "sample-app"
	cName           = "my-cname.com"
	testEnvvarKey   = "FOO"
	testEnvVarValue = "BAR"
)

func init() {
	// set ingress IP
	b, err := exec.Command("kubectl", "get", "svc", "traefik", "-o", "jsonpath='{.status.loadBalancer.ingress[0].ip}'").Output()
	if err != nil {
		panic(err)
	}
	ingress = string(b)

	// set ketch executable path
	ketchExecPath := os.Getenv("KETCH_EXECUTABLE_PATH")
	if ketchExecPath != "" {
		ketch = ketchExecPath
		return
	}
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	ketch = filepath.Join(pwd, "bin", "ketch")
}

// retry tries a command <times> in intervals of <wait> seconds.
// If <match> is never found in command output, an error is returned containing
// all aggregated output.
func retry(cmd *exec.Cmd, match string, times, wait int) error {
	sb := strings.Builder{}
	for i := 0; i < times; i++ {
		b, err := exec.Command(ketch, "app", "info", appName).Output()
		if err != nil {
			return err
		}
		sb.Write(b)
		sb.WriteString("\n")

		if strings.Contains(string(b), match) {
			return nil
		}
		if i < times-1 {
			fmt.Println("retrying command: ", cmd.String())
			time.Sleep(time.Second * time.Duration(wait))
		}
	}
	return fmt.Errorf("retry failed on command %s. Output: %s", cmd.String(), sb.String())
}

func TestHelp(t *testing.T) {
	b, err := exec.Command(ketch, "help").Output()
	require.Nil(t, err)
	require.Contains(t, string(b), "For details see https://theketch.io")
	require.Contains(t, string(b), "Available Commands")
	require.Contains(t, string(b), "Flags")
}

func TestFrameworkAdd(t *testing.T) {
	b, err := exec.Command(ketch, "framework", "add", framework, "--ingress-service-endpoint", ingress, "--ingress-type", "traefik").Output()
	require.Nil(t, err)
	require.Contains(t, string(b), "Successfully added!")
}

func TestAppDeploy(t *testing.T) {
	b, err := exec.Command(ketch, "app", "deploy", appName, "--framework", framework, "-i", appImage).Output()
	require.Nil(t, err)
	require.Equal(t, "", string(b))
}

func TestAppInfo(t *testing.T) {
	cmd := exec.Command(ketch, "app", "info", appName)
	err := retry(cmd, "running", 10, 5)
	require.Nil(t, err)

	headerRegex := regexp.MustCompile("DEPLOYMENT VERSION[ \t]+IMAGE[ \t]+PROCESS NAME[ \t]+WEIGHT[ \t]+STATE[ \t]+CMD")
	dataRegex := regexp.MustCompile(fmt.Sprintf("1[ \t]+%s[ \t]+web[ \t]+100%%[ \t]+1 running[ \t]", appImage))
	b, err := cmd.Output()
	require.Nil(t, err)
	require.True(t, headerRegex.Match(b))
	require.True(t, dataRegex.Match(b))
}

func TestAppRemove(t *testing.T) {
	b, err := exec.Command(ketch, "app", "remove", appName).Output()
	require.Nil(t, err)
	require.Contains(t, string(b), "Successfully removed!")
}

func TestFrameworkRemove(t *testing.T) {
	b, err := exec.Command(ketch, "framework", "remove", framework).Output()
	require.Nil(t, err)
	require.Contains(t, string(b), "Framework successfully removed!")
}
