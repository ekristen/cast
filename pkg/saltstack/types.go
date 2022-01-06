package saltstack

import "gopkg.in/yaml.v3"

type Change struct {
	New    string `yaml:"new,omitempty"`
	Old    string `yaml:"old,omitempty"`
	PID    int    `yaml:"pid,omitempty"`
	Code   int    `yaml:"retcode,omitempty"`
	Stderr string `yaml:"stderr,omitempty"`
	Stdout string `yaml:"stdout,omitempty"`
}

type Result struct {
	ID        string                      `yaml:"__id__"`
	RunNumber int                         `yaml:"__run_num__"`
	SLS       string                      `yaml:"__sls__"`
	Changes   map[interface{}]interface{} `yaml:"changes"`
	Comment   string                      `yaml:"comment"`
	Duration  float64                     `yaml:"duration"`
	Name      string                      `yaml:"name"`
	Result    bool                        `yaml:"result"`
	StartTime string                      `yaml:"start_time"`
	Warnings  []string                    `yaml:"warnings"`
}

type Results map[string]Result

type LocalResults struct {
	Local Results `yaml:"local"`
}

type LocalResultsErrors struct {
	Local []string `yaml:"local"`
}

func ParseLocalResults(contents []byte) (res *LocalResults, err error) {
	if err := yaml.Unmarshal(contents, &res); err != nil {
		return nil, err
	}

	return res, nil
}

/*
  test_|-my-custom-combo_|-foo_|-configurable_test_state:
    __id__: my-custom-combo
    __run_num__: 5
    __sls__: top
    changes:
      testing:
        new: Something pretended to change
        old: Unchanged
    comment: bar.baz
    duration: 1.277
    name: foo
    result: null
    start_time: '16:35:55.220220'
    warnings:
    - A warning
*/
