package athanor

import (
	"fmt"
	"strings"
	"time"

	"primamateria.systems/materia/pkg/plan"
)

type ComponentReport struct {
	Name  string `json:"name" toml:"name"`
	Error error  `json:"error" toml:"error"`
}

func (c *ComponentReport) String() string {
	if c.Error != nil {
		return fmt.Sprintf("Component: %v\nStatus: FAILED\nError: %v", c.Name, c.Error)
	}
	return fmt.Sprintf("Component: %v\nStatus: Success", c.Name)
}

type BackupReport struct {
	Hostname  string            `json:"hostname" toml:"hostname"`
	Successes []ComponentReport `json:"successes" toml:"successes"`
	Failures  []ComponentReport `json:"failures" toml:"failures"`
	Skipped   []ComponentReport `json:"skipped" toml:"skipped"`
	StartTime time.Time         `json:"start_time" toml:"start_time"`
	EndTime   time.Time         `json:"end_time" toml:"end_time"`
}

func (r *BackupReport) Report() (string, error) {
	var result strings.Builder
	_, err := fmt.Fprintf(&result, "Backup Report for %v\nStart: %v\nEnd: %v", r.Hostname, r.StartTime.Format(time.RFC822), r.EndTime.Format(time.RFC822))
	if err != nil {
		return "", err
	}
	_, err = result.WriteString("\nSuccesses: ")
	if err != nil {
		return "", err
	}
	for _, c := range r.Successes {
		_, err := result.WriteString(c.Name + " ")
		if err != nil {
			return "", err
		}
	}
	_, err = result.WriteString("\nSkipped: ")
	if err != nil {
		return "", err
	}
	for _, c := range r.Skipped {
		_, err := result.WriteString(c.Name + " ")
		if err != nil {
			return "", err
		}
	}
	_, err = result.WriteString("\nFailures: ")
	if err != nil {
		return "", err
	}
	for _, c := range r.Failures {
		_, err := result.WriteString(c.String() + "\n")
		if err != nil {
			return "", err
		}
	}

	return result.String(), nil
}

func (r *BackupReport) AddReport(c string, p *plan.Plan, err error) {
	if err != nil {
		r.Failures = append(r.Failures, ComponentReport{Name: c, Error: err})
		return
	}
	if p == nil || p.Size() == 0 {
		r.Skipped = append(r.Skipped, ComponentReport{Name: c})
	}
	r.Successes = append(r.Successes, ComponentReport{Name: c})
}
