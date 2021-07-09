package chart

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

var (
	ErrEmptyProcfile = errors.New("procfile should contain at least one process name with a command")

	processNameRegex = regexp.MustCompile(`^([A-Za-z0-9_-]+)$`)
)

// Procfile represents a parsed Procfile.
type Procfile struct {
	Processes           map[string][]string
	RoutableProcessName string
}

func (p *Procfile) IsRoutable(processName string) bool {
	return p.RoutableProcessName == processName
}

func (p *Procfile) SortedNames() []string {
	names := make([]string, 0, len(p.Processes))
	for name := range p.Processes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type packMetadata struct {
	Processes []packProcess `json:"processes"`
}

// contains other fields like command, but only the name (type) is needed
type packProcess struct {
	Type string `json:"type"`
}

// CreateProcfile creates a Procfile instance from pack build metadata found under the
// `io.buildpacks.build.metadata` label. This function should only be called for images
// build using pack
func CreateProcfile(buildMetadata string) (*Procfile, error) {
	var meta packMetadata
	if err := json.Unmarshal([]byte(buildMetadata), &meta); err != nil {
		return nil, err
	}
	processes := make(map[string][]string, len(meta.Processes))
	var names []string
	for _, process := range meta.Processes {
		if p := processNameRegex.FindStringSubmatch(process.Type); p != nil {
			name := p[1]

			// inside the docker image created by pack, executables specified as the names
			// in the procfile will be added to /cnb/process. These executables run the commands
			// specified in the procfile. Trying to run the commands as they are in the Procfile
			// will result in an executable file not found in $PATH: unknown error
			processes[name] = []string{strings.TrimSpace(name)}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, ErrEmptyProcfile
	}
	return &Procfile{
		Processes:           processes,
		RoutableProcessName: routableProcess(names),
	}, nil
}

// ProcfileFromProcesses construct a Procfile instance from a list of ProcessSpec and returns it.
func ProcfileFromProcesses(processes []ketchv1.ProcessSpec) (*Procfile, error) {
	if len(processes) == 0 {
		return nil, ErrEmptyProcfile
	}
	procfile := Procfile{
		Processes: make(map[string][]string, len(processes)),
	}
	var names []string
	for _, spec := range processes {
		procfile.Processes[spec.Name] = spec.Cmd
		names = append(names, spec.Name)
	}
	procfile.RoutableProcessName = routableProcess(names)
	return &procfile, nil
}

func routableProcess(names []string) string {
	for _, name := range names {
		if name == DefaultRoutableProcessName {
			return DefaultRoutableProcessName
		}
	}
	sort.Strings(names)
	return names[0]
}

func WriteProcfile(processes []ketchv1.ProcessSpec, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, process := range processes {
		_, err = fmt.Fprintf(f, "%s: %s\n", process.Name, strings.Join(process.Cmd, " "))
		if err != nil {
			return err
		}
	}
	return nil
}
