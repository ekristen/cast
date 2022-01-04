package saltstack

type Change struct {
	New string `yaml:"new"`
	Old string `yaml:"old"`
}

type Result struct {
	ID        string            `yaml:"__id__"`
	RunNumber int               `yaml:"__run_num__"`
	SLS       string            `yaml:"__sls__"`
	Changes   map[string]Change `yaml:"changes"`
	Comment   string            `yaml:"comment"`
	Duration  float64           `yaml:"duration"`
	Name      string            `yaml:"name"`
	Result    string            `yaml:"result"`
	StartTime string            `yaml:"start_time"`
	Warnings  []string          `yaml:"warnings"`
}

type Results map[string]Result

type LocalResults struct {
	Local Results `yaml:"local"`
}

type LocalResultsErrors struct {
	Local []string `yaml:"local"`
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
