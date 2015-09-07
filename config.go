package gitconfig

// go get github.com/pointlander/peg
//go:generate peg -switch -inline config.peg

func (p *config) addSection(stype string) {
	p.curSection = &Section{
		Type:   stype,
		Values: make(map[string]string),
	}
	p.sections = append(p.sections, p.curSection)
}

func (p *config) setID(id string) {
	p.curSection.ID = id
}

func (p *config) addValue(value string) {
	p.curSection.Values[p.curKey] = value
}

func (p *config) setKey(key string) {
	p.curKey = key
}
