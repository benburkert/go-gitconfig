package gitconfig

func Parse(data []byte) ([]*Section, error) {
	conf := &config{
		Buffer: string(data),
	}

	conf.Init()
	if err := conf.Parse(); err != nil {
		return nil, err
	}
	conf.Execute()

	return conf.sections, nil
}

type Section struct {
	Type, ID string
	Values   map[string]string
}
