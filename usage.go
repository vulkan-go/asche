package dieselvk

import "fmt"

const (
	MULTIGPU = "DeviceGroup"
)

//Defines the core usage properites and expects usage patterns. Corresponds to JSON object notation
// and should be extendable to JSON parsing implementations. Properties to consider along with the
//usage property layouts. For given compute usage is true. "Compute" -> "True" then the implementation
//Will identify a valid Compute properties layout looking for parameters such as "MultiGPU" -> 2,

type Usage struct {
	Name         string
	String_props map[string]string
	Int_props    map[string]int
	Bool_props   map[string]bool
	Float_props  map[string]float32
	Linked_usage *Usage
}

func NewUsage(name string, default_size uint) *Usage {
	var use Usage
	use.Name = name
	use.String_props = make(map[string]string, default_size)
	use.Int_props = make(map[string]int, default_size)
	use.Bool_props = make(map[string]bool, default_size)
	use.Float_props = make(map[string]float32, default_size)
	return &use
}

func (u *Usage) HasNext() bool {
	if u.Linked_usage != nil {
		return true
	}
	return false
}

func (u *Usage) GetLinkedUsage() (*Usage, error) {
	var use *Usage

	if u.HasNext() {
		use = u.Linked_usage
	} else {
		return nil, fmt.Errorf("Properties %s has no linked usage\n", u.Name)
	}

	return use, nil
}

//Prints usage tree
func (u *Usage) Print() {
	fmt.Print(u.String_props)
	fmt.Print(u.Bool_props)
	fmt.Print(u.Int_props)
	fmt.Print(u.Float_props)
	if u.HasNext() {
		u.Linked_usage.Print()
	}
}
