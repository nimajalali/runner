package stats

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"gopkg.in/inconshreveable/log15.v2"
)

type NewRelicAgentConfig struct {
	Host    string `json:"host"`
	Version string `json:"version"`
	Pid     int    `json:"pid"`
}

// examples: https://docs.newrelic.com/docs/plugins/plugin-developer-resources/developer-reference/metric-data-plugin-api#examples
type newRelicRequest struct {
	Agent      *agent       `json:"agent"`
	Components []*component `json:"components"`
}

type NewRelicReporterConfig struct {
	Agent      *NewRelicAgentConfig
	LicenseKey string `json:"license_key"`
}

type NewRelicReporter struct {
	Agent      *agent
	LicenseKey string
}

func NewNewRelicReporter(version string, licenseKey string) *NewRelicReporter {
	r := &NewRelicReporter{}
	r.Agent = newNewRelicAgent(version)
	r.LicenseKey = licenseKey
	return r
}

func (r *NewRelicReporter) report(stats []*collectedStat) {
	client := &http.Client{}
	req := &newRelicRequest{}
	req.Agent = r.Agent
	comp := newComponent()
	comp.Name = "IronMQ"
	comp.Duration = 60
	comp.GUID = "io.iron.ironmq"
	// TODO - NR has a fixed 3 level heirarchy? and we just use 2?
	req.Components = []*component{comp}

	// now add metrics
	for _, s := range stats {
		for k, v := range s.Counters {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = v
		}
		for k, v := range s.Values {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = int64(v)
		}
		for k, v := range s.Timers {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = int64(v)
		}
	}

	metricsJson, err := json.Marshal(req)
	if err != nil {
		log15.Error("error encoding json for NewRelicReporter", "err", err)
	}

	jsonAsString := string(metricsJson)

	httpRequest, err := http.NewRequest("POST",
		"https://platform-api.newrelic.com/platform/v1/metrics",
		strings.NewReader(jsonAsString))
	if err != nil {
		log15.Error("error creating New Relic request:", "err", err)
		return
	}
	httpRequest.Header.Set("X-License-Key", r.LicenseKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		log15.Error("error sending http request in NewRelicReporter", "err", err)
		return
	}
	defer httpResponse.Body.Close()
	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log15.Error("error reading response body", "err", err)
	} else {
		log15.Debug("response", "code", httpResponse.Status, "body", string(body))
	}
}

type agent struct {
	Host    string `json:"host"`
	Version string `json:"version"`
	Pid     int    `json:"pid"`
}

func newNewRelicAgent(Version string) *agent {
	var err error
	agent := &agent{
		Version: Version,
	}
	agent.Pid = os.Getpid()
	if agent.Host, err = os.Hostname(); err != nil {
		log15.Error("Can not get hostname", "err", err)
		return nil
	}
	return agent
}

type component struct {
	Name     string           `json:"name"`
	GUID     string           `json:"guid"`
	Duration int              `json:"duration"`
	Metrics  map[string]int64 `json:"metrics"`
}

func newComponent() *component {
	c := &component{}
	c.Metrics = make(map[string]int64)
	return c
}