package chart

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strings"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

var (
	ErrEmptyProcfile = errors.New("procfile should contain at least one process name with a command")

	procfileRegex = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*(.+)$`)
)

// Procfile represents a parsed Procfile.
type Procfile struct {
	Processes           map[string][]string
	RoutableProcessName string
}

// NewProcfile creates a Procfile from a file.
func NewProcfile(fileName string) (*Procfile, error) {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return ParseProcfile(string(content))
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

// ParseProcfile parses the content of Procfile and returns a Procfile instance.
// this function should only be called when building from source, since deploying
// from image does not allow a user to specify a Procfile
func ParseProcfile(content string) (*Procfile, error) {
	procfile := strings.Split(content, "\n")
	processes := make(map[string][]string, len(procfile))
	var names []string
	for _, process := range procfile {
		if p := procfileRegex.FindStringSubmatch(process); p != nil {
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
