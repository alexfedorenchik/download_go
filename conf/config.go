package conf

import (
	"download/ui"
	"log"
)

type Choice struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func (it Choice) GetName() string {
	return it.Label
}

type Param struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Values      []Choice `json:"choice"`
	resolved    []string
	isResolved  bool
}

func (it Param) GetDescription() string {
	return it.Description
}

func (it Param) GetNamedData() (res []ui.NamedOption) {
	res = make([]ui.NamedOption, len(it.Values))
	for i := range it.Values {
		res[i] = it.Values[i]
	}
	return
}

type Source struct {
	Name       string   `json:"name"`
	Path       []string `json:"path"`
	Parameters []Param  `json:"parameters"`
}

func (it Source) GetName() string {
	return it.Name
}

type Configuration struct {
	Description string   `json:"description"`
	Sources     []Source `json:"sources"`
}

func (it Configuration) GetDescription() string {
	return it.Description
}

func (it Configuration) GetNamedData() (res []ui.NamedOption) {
	res = make([]ui.NamedOption, len(it.Sources))
	for i := range it.Sources {
		res[i] = it.Sources[i]
	}
	return
}

func (it *Param) Resolve(vals []string) {
	it.isResolved = true
	it.resolved = vals
}

func (it Param) GetResolved() []string {
	if !it.isResolved {
		log.Printf("Unresolved param %v", it.Name)
		panic(it)
	}
	return it.resolved
}
